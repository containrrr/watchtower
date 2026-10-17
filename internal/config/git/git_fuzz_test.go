package git

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// FuzzParseImageMapping verifies --git-image values never panic and that a
// successful parse always has an image, repo, and recognized policy.
func FuzzParseImageMapping(f *testing.F) {
	f.Add("myapp:latest=https://github.com/org/app.git")
	f.Add("myapp:latest=https://github.com/org/app.git#main")
	f.Add("myapp:latest=https://github.com/org/app.git#develop@patch")
	f.Add("myapp=git@github.com:org/app.git")
	f.Add("myapp=git@github.com:org/app.git#v1.2.3@minor")
	f.Add("myapp=ssh://git@github.com/org/app.git")
	f.Add("=https://github.com/org/app.git")
	f.Add("myapp=")
	f.Add("not-a-mapping")
	f.Add("myapp=https://github.com/org/app.git@nightly")
	f.Add("")
	f.Add("image=repo#")
	f.Add("image=@patch")

	f.Fuzz(func(t *testing.T, raw string) {
		image, mapping, err := parseImageMapping(raw)
		if err != nil {
			if !errors.Is(err, ErrInvalidGitImage) && !errors.Is(err, errInvalidGitPolicy) {
				t.Fatalf("unexpected error: %v", err)
			}

			return
		}

		assert.NotEmpty(t, image)
		assert.NotEmpty(t, mapping.Repo)
		assert.True(t, types.ValidGitPolicy(mapping.Policy), "policy %q", mapping.Policy)
		assert.NotEmpty(t, mapping.Ref)
	})
}

// FuzzParseComposeProjects verifies name=/path rows never panic and that
// successful maps contain only non-empty keys and values.
func FuzzParseComposeProjects(f *testing.F) {
	f.Add("web=/srv/web")
	f.Add("web=/srv/web\napi=/srv/api")
	f.Add("nopair")
	f.Add("=/srv/web")
	f.Add("web=")
	f.Add("")
	f.Add("web=/srv/web\n\nweb=/other")

	f.Fuzz(func(t *testing.T, raw string) {
		values := strings.Split(raw, "\n")

		got, err := ParseComposeProjects(values)
		if err != nil {
			assert.ErrorIs(t, err, ErrInvalidComposeProject)

			return
		}

		for name, dir := range got {
			assert.NotEmpty(t, name)
			assert.NotEmpty(t, dir)
		}
	})
}
