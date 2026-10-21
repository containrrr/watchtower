package container

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func Test_confinedDockerfile(t *testing.T) {
	t.Parallel()

	t.Run("defaults empty to Dockerfile", func(t *testing.T) {
		t.Parallel()

		got, err := confinedDockerfile("")
		require.NoError(t, err)
		assert.Equal(t, DefaultGitDockerfile, got)
	})

	t.Run("accepts a nested relative path", func(t *testing.T) {
		t.Parallel()

		got, err := confinedDockerfile("build/docker/Dockerfile")
		require.NoError(t, err)
		assert.Equal(t, "build/docker/Dockerfile", got)
	})

	t.Run("rejects path escape and absolute paths", func(t *testing.T) {
		t.Parallel()

		_, err := confinedDockerfile("../Dockerfile")
		require.ErrorIs(t, err, errGitDockerfileEscape)
		_, err = confinedDockerfile("/etc/passwd")
		require.ErrorIs(t, err, errGitDockerfileEscape)
	})
}

func Test_redactedRemote(t *testing.T) {
	t.Parallel()

	t.Run("replaces a password and leaves other URLs unchanged", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "https://xxxxx:xxxxx@github.com/org/app.git#abc", redactedRemote("https://user:secret@github.com/org/app.git#abc"))
		assert.Equal(t, "https://github.com/org/app.git", redactedRemote("https://github.com/org/app.git"))
		assert.Equal(t, "://bad", redactedRemote("://bad"))
	})
}

func Test_consumeBuildStream(t *testing.T) {
	t.Parallel()

	t.Run("returns the last aux image ID", func(t *testing.T) {
		t.Parallel()

		body := strings.NewReader(`{"stream":"Step 1/1"}
{"aux":{"ID":"sha256:first"}}
{"aux":{"ID":"sha256:last"}}
`)
		id, err := consumeBuildStream(body)
		require.NoError(t, err)
		assert.Equal(t, types.ImageID("sha256:last"), id)
	})

	t.Run("returns a build error from the stream", func(t *testing.T) {
		t.Parallel()

		_, err := consumeBuildStream(strings.NewReader(`{"error":"failed to solve"}`))
		require.ErrorIs(t, err, errImageBuildFailed)
	})

	t.Run("redacts a token echoed by the daemon", func(t *testing.T) {
		t.Parallel()

		remote := "https://git:secret@github.com/org/app.git#abc"
		streamErr := fmt.Errorf("%w: failed %s", errImageBuildFailed, remote)
		got := redactBuildError(streamErr, remote)
		assert.NotContains(t, got.Error(), "secret")
		assert.Contains(t, got.Error(), "xxxxx:xxxxx")

		clientErr := fmt.Errorf("image build: Get %q: denied", remote)
		got = redactBuildError(clientErr, remote)
		assert.NotContains(t, got.Error(), "secret")
		assert.Contains(t, got.Error(), "xxxxx:xxxxx")

		tokenOnly := redactBuildError(errors.New("clone https://token@github.com/org/app.git failed"), "")
		assert.NotContains(t, tokenOnly.Error(), "token@")
		assert.Contains(t, tokenOnly.Error(), "xxxxx:xxxxx@")
	})

	t.Run("returns a decode error for invalid JSON", func(t *testing.T) {
		t.Parallel()

		_, err := consumeBuildStream(strings.NewReader("{"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read build stream:")
	})

	t.Run("ignores a string aux payload from BuildKit", func(t *testing.T) {
		t.Parallel()

		body := strings.NewReader(`{"aux":"trace"}
{"aux":{"ID":"sha256:last"}}
`)
		id, err := consumeBuildStream(body)
		require.NoError(t, err)
		assert.Equal(t, types.ImageID("sha256:last"), id)
	})
}
