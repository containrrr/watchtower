package apply

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzRemoteContext verifies Docker Git context URLs never panic and that a
// successful result cannot smuggle a path-escape fragment.
func FuzzRemoteContext(f *testing.F) {
	f.Add("https://github.com/org/app.git", "abc123", "")
	f.Add("https://github.com/org/app.git", "abc123", ".")
	f.Add("https://github.com/org/app.git", "abc123", "apps/api")
	f.Add("git@github.com:org/app.git", "abc123", "")
	f.Add("ssh://git@github.com/org/app.git", "deadbeef", "docker")
	f.Add("https://github.com/org/app.git", "", "")
	f.Add("https://github.com/org/app.git", "abc#123", "")
	f.Add("https://github.com/org/app.git", "abc123", "../etc")
	f.Add("https://github.com/org/app.git", "abc123", "/abs")
	f.Add("", "abc123", "")
	f.Add("http://git.example.com/org/app", "abc123", `foo\bar`)
	f.Add("0:0", "0", `\\`)

	f.Fuzz(func(t *testing.T, repo, commit, contextRel string) {
		got, err := RemoteContext(repo, commit, contextRel)
		if err != nil {
			return
		}

		require.Contains(t, got, "#")
		_, fragment, _ := strings.Cut(got, "#")

		_, subdir, hasSubdir := strings.Cut(fragment, ":")
		if hasSubdir {
			cleaned := path.Clean(subdir)
			assert.NotEqual(t, "..", cleaned)
			assert.False(t, strings.HasPrefix(cleaned, "../"))
			assert.False(t, strings.HasPrefix(cleaned, "/"))
		}
	})
}

// FuzzContextFragment verifies subdirectory fragments never escape the repo.
func FuzzContextFragment(f *testing.F) {
	f.Add("")
	f.Add(".")
	f.Add("apps/api")
	f.Add("../etc")
	f.Add("/abs")
	f.Add("foo:bar")
	f.Add(`foo\bar`)
	f.Add(`\\`)
	f.Add("a/./b")

	f.Fuzz(func(t *testing.T, rel string) {
		got, err := SafeRelPath(rel)
		if err != nil {
			assert.ErrorIs(t, err, errPathEscape)

			return
		}

		if got == "" {
			return
		}

		assert.False(t, path.IsAbs(got))
		assert.True(t, filepath.IsLocal(filepath.FromSlash(got)))
		assert.NotContains(t, got, ":")
	})
}

// FuzzSafeRelPath verifies Dockerfile and context paths never escape the repo.
func FuzzSafeRelPath(f *testing.F) {
	f.Add("Dockerfile")
	f.Add("build/docker/Dockerfile")
	f.Add("../etc/passwd")
	f.Add("/abs")
	f.Add(`\\`)
	f.Add("")
	f.Add(".")

	f.Fuzz(func(t *testing.T, rel string) {
		got, err := SafeRelPath(rel)
		if err != nil {
			assert.ErrorIs(t, err, errPathEscape)

			return
		}

		if got == "" {
			return
		}

		cleaned := path.Clean(got)
		assert.False(t, path.IsAbs(cleaned))
		assert.True(t, filepath.IsLocal(filepath.FromSlash(cleaned)))
	})
}

// FuzzWithAuth verifies token embedding never panics, empty tokens leave the
// remote unchanged, and a successful result carries the token.
func FuzzWithAuth(f *testing.F) {
	f.Add("https://github.com/org/app.git#abc", "x-access-token", "secret")
	f.Add("https://github.com/org/app.git#abc", "git", "")
	f.Add("https://github.com/org/app.git#abc", "", "secret")
	f.Add("ssh://git@github.com/org/app.git", "git", "secret")
	f.Add("://", "git", "secret")
	f.Add("", "user", "pw")
	f.Add("https:0", "0", "0")

	f.Fuzz(func(t *testing.T, remote, username, token string) {
		got, err := WithAuth(remote, username, token)
		if strings.TrimSpace(token) == "" {
			require.NoError(t, err)
			assert.Equal(t, remote, got)

			return
		}

		if err != nil {
			return
		}

		parsed, err := url.Parse(got)
		require.NoError(t, err)

		assert.Equal(t, "https", parsed.Scheme)
		require.NotNil(t, parsed.User)
		pw, ok := parsed.User.Password()
		assert.True(t, ok)
		assert.Equal(t, strings.TrimSpace(token), pw)
	})
}
