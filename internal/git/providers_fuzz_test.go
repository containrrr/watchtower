package git

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzSameOriginNext verifies pagination URLs cannot leave the original host.
func FuzzSameOriginNext(f *testing.F) {
	f.Add("https://api.github.com/repos/o/a/tags", "https://api.github.com/repos/o/a/tags?page=2")
	f.Add("https://api.github.com/repos/o/a/tags", "https://evil.example/x")
	f.Add("https://api.github.com/repos/o/a/tags", "http://api.github.com/repos/o/a/tags")
	f.Add("https://api.github.com/repos/o/a/tags", "file:///etc/passwd")
	f.Add("", "https://api.github.com/x")

	f.Fuzz(func(t *testing.T, current, next string) {
		got := sameOriginNext(current, next)
		if got == "" {
			return
		}

		cur, err := url.Parse(current)
		require.NoError(t, err)
		nxt, err := url.Parse(got)
		require.NoError(t, err)
		assert.Equal(t, strings.ToLower(cur.Host), strings.ToLower(nxt.Host))
		assert.Equal(t, strings.ToLower(cur.Scheme), strings.ToLower(nxt.Scheme))
	})
}

// FuzzNextLink verifies Link-header parsing never panics and that a next URL
// came from an angle-bracketed token.
func FuzzNextLink(f *testing.F) {
	f.Add(`<https://ex/p2>; rel="next"`)
	f.Add(`<https://ex/p2>; rel=next`)
	f.Add(`<https://ex/p1>; rel="prev", <https://ex/p3>; rel="next"`)
	f.Add(`https://ex/p2; rel="next"`)
	f.Add("")
	f.Add("<>")
	f.Add(`<https://ex/p2`)

	f.Fuzz(func(t *testing.T, header string) {
		got := nextLink(header)
		if got == "" {
			return
		}

		assert.Contains(t, header, "<"+got+">")
	})
}

// FuzzSplitRepo verifies clone URL splitting never panics and that a
// successful parse has host, owner, and name.
func FuzzSplitRepo(f *testing.F) {
	f.Add("https://github.com/org/app.git")
	f.Add("git@gitlab.com:group/app.git")
	f.Add("https://gitlab.com/a/b/c.git")
	f.Add("")
	f.Add("https://github.com")
	f.Add("https://github.com/app.git")

	f.Fuzz(func(t *testing.T, repo string) {
		host, owner, name, ok := splitRepo(repo)
		if !ok {
			return
		}

		assert.NotEmpty(t, host)
		assert.NotEmpty(t, owner)
		assert.NotEmpty(t, name)
		assert.False(t, strings.HasSuffix(name, ".git"))
	})
}
