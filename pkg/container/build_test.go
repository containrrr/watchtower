package container

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerClient "github.com/moby/moby/client"

	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
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

// TestClientBuildRemoteImage verifies that a remote build sends the Git context,
// tags, and Dockerfile to Docker, takes the image ID from the build stream or
// else from an inspect of the first tag, and keeps credentials out of its
// errors.
func TestClientBuildRemoteImage(t *testing.T) {
	t.Parallel()

	const remote = "https://user:secret@github.com/org/app.git#abc"

	// expectBuild expects one build of remote tagged app:latest and answers it
	// with the given stream body.
	expectBuild := func(api *mockContainer.MockAPIClient, body string, err error) {
		api.EXPECT().ImageBuild(mock.Anything, http.NoBody, dockerClient.ImageBuildOptions{
			RemoteContext: remote,
			Tags:          []string{"app:latest"},
			Remove:        true,
			Dockerfile:    DefaultGitDockerfile,
		}).Return(dockerClient.ImageBuildResult{Body: io.NopCloser(strings.NewReader(body))}, err).Once()
	}

	t.Run("returns the image ID from the build stream", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectBuild(api, `{"aux":{"ID":"sha256:built"}}`, nil)

		id, err := (&client{log: newTestLogger(t), api: api}).BuildRemoteImage(t.Context(), remote, "", []string{"app:latest"})
		require.NoError(t, err)
		assert.Equal(t, types.ImageID("sha256:built"), id)
	})

	t.Run("inspects the first tag when the stream has no image ID", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectBuild(api, `{"stream":"Step 1/1"}`, nil)
		api.EXPECT().ImageInspect(mock.Anything, "app:latest").Return(inspected("sha256:tagged"), nil).Once()

		id, err := (&client{log: newTestLogger(t), api: api}).BuildRemoteImage(t.Context(), remote, "", []string{"app:latest"})
		require.NoError(t, err)
		assert.Equal(t, types.ImageID("sha256:tagged"), id)
	})

	t.Run("fails when no image ID can be found", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectBuild(api, `{"stream":"Step 1/1"}`, nil)
		api.EXPECT().ImageInspect(mock.Anything, "app:latest").Return(inspected(""), nil).Once()

		_, err := (&client{log: newTestLogger(t), api: api}).BuildRemoteImage(t.Context(), remote, "", []string{"app:latest"})
		require.ErrorIs(t, err, errBuildImageIDMissing)
	})

	t.Run("redacts credentials from a failed build", func(t *testing.T) {
		t.Parallel()

		api := newDockerAPI(t)
		expectBuild(api, "", fmt.Errorf("unable to prepare context: %s", remote))

		_, err := (&client{log: newTestLogger(t), api: api}).BuildRemoteImage(t.Context(), remote, "", []string{"app:latest"})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret")
	})

	t.Run("rejects an empty remote before calling Docker", func(t *testing.T) {
		t.Parallel()

		// The mock has no expectations, so any Docker call fails the test.
		_, err := (&client{log: newTestLogger(t), api: mockContainer.NewMockAPIClient(t)}).
			BuildRemoteImage(t.Context(), "", "", []string{"app:latest"})
		require.ErrorIs(t, err, errGitRemoteEmpty)
	})
}
