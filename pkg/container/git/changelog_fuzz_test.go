package git

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FuzzParseRepo verifies clone URLs from labels never panic and that a
// successful split has host, owner, and name.
func FuzzParseRepo(f *testing.F) {
	f.Add("https://github.com/org/app.git")
	f.Add("git@github.com:org/app.git")
	f.Add("deploy@git.example.com:org/app.git")
	f.Add("ssh://git@gitlab.com/group/app.git")
	f.Add("github.com/org/app")
	f.Add("https://gitlab.com/a/b/c.git")
	f.Add("git@:0/0")
	f.Add("git@github.com")
	f.Add("https://github.com")
	f.Add("https://")
	f.Add("")
	f.Add("  ")

	f.Fuzz(func(t *testing.T, raw string) {
		host, owner, name, ok := parseRepo(raw)
		if !ok {
			assert.Empty(t, host)
			assert.Empty(t, owner)
			assert.Empty(t, name)

			return
		}

		assert.NotEmpty(t, host)
		assert.NotEmpty(t, owner)
		assert.NotEmpty(t, name)
		assert.False(t, strings.HasSuffix(name, "/"))
	})
}

// FuzzApplyVars verifies changelog templates never panic.
func FuzzApplyVars(f *testing.F) {
	f.Add("https://example.com/{tag}", "v1.2.3", "abc")
	f.Add("https://example.com/{major}.{minor}.{patch}", "1.2.3", "")
	f.Add("https://example.com/{commit}", "nightly", "deadbeef")
	f.Add("{tag}{commit}{major}", "", "")
	f.Add("", "v1.0.0", "abc")

	f.Fuzz(func(_ *testing.T, template, tag, commit string) {
		_ = applyVars(template, ChangelogVars{Tag: tag, Commit: commit})
	})
}

// FuzzDerivedReleasesURL verifies release URL derivation never panics and
// yields an http or https URL that parses and has no dot path segments.
func FuzzDerivedReleasesURL(f *testing.F) {
	f.Add("https://github.com/org/app.git", "", "")
	f.Add("https://gitlab.com/org/app.git", "", "v1.2.3")
	f.Add("https://codeberg.org/org/app.git", "", "release/1.2.3")
	f.Add("https://git.example.com/org/app.git", "https://git.example.com/api/v1", "")
	f.Add("https://unknown.example/org/app.git", "", "v1.2.3")
	f.Add("", "", "")
	f.Add("gitlAB.Com/0/0", "", "\x01")

	f.Fuzz(func(t *testing.T, repo, extra, tag string) {
		got := derivedReleasesURL(repo, extra, tag)
		if got == "" {
			return
		}

		assert.True(t, strings.HasPrefix(got, "http://") || strings.HasPrefix(got, "https://"), got)

		// No dot segment may restructure the release path.
		parsed, err := url.Parse(got)
		require.NoError(t, err)

		for segment := range strings.SplitSeq(parsed.Path, "/") {
			assert.NotEqual(t, ".", segment, got)
			assert.NotEqual(t, "..", segment, got)
		}
	})
}

// FuzzSafeReleaseTag verifies tag admission never panics and never admits a
// tag that could restructure the derived release path or contains an ASCII
// control character.
func FuzzSafeReleaseTag(f *testing.F) {
	f.Add("v1.2.3")
	f.Add("1.2.3")
	f.Add("release/1.2.3")
	f.Add("..")
	f.Add("/")
	f.Add("")
	f.Add("1.2.3?a=b")
	f.Add("1.2.3\x01")

	f.Fuzz(func(t *testing.T, tag string) {
		if !safeReleaseTag(tag) {
			return
		}

		assert.NotEmpty(t, tag)
		assert.NotContains(t, tag, "?")
		assert.False(t, strings.ContainsFunc(tag, isASCIIControl), "%q", tag)

		for part := range strings.SplitSeq(tag, "/") {
			assert.NotEqual(t, ".", part)
			assert.NotEqual(t, "..", part)
			assert.NotEmpty(t, part)
		}
	})
}
