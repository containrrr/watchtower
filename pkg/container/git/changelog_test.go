package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangelog(t *testing.T) {
	t.Parallel()

	t.Run("explicit template", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			ChangelogURLLabel: "https://example.com/notes/{tag}#{commit}",
		}, "app:latest")
		got := Changelog(c, "https://github.com/org/app.git", ChangelogVars{Tag: "v1.2.3", Commit: "abc"})
		assert.Equal(t, "https://example.com/notes/v1.2.3#abc", got)
	})

	t.Run("derived github url", func(t *testing.T) {
		t.Parallel()

		got := Changelog(testContainer(t, nil, "app:latest"), "https://github.com/org/app.git", ChangelogVars{})
		assert.Equal(t, "https://github.com/org/app/releases", got)
	})

	t.Run("derived url for a known tag", func(t *testing.T) {
		t.Parallel()

		got := Changelog(testContainer(t, nil, "app:latest"), "https://github.com/org/app.git", ChangelogVars{Tag: "v1.2.3"})
		assert.Equal(t, "https://github.com/org/app/releases/tag/v1.2.3", got)
	})

	t.Run("explicit template wins over a known tag", func(t *testing.T) {
		t.Parallel()

		c := testContainer(t, map[string]string{
			ChangelogURLLabel: "https://example.com/notes/{tag}",
		}, "app:latest")
		got := Changelog(c, "https://github.com/org/app.git", ChangelogVars{Tag: "v1.2.3"})
		assert.Equal(t, "https://example.com/notes/v1.2.3", got)
	})

	t.Run("empty when unknown host", func(t *testing.T) {
		t.Parallel()

		got := Changelog(testContainer(t, nil, "app:latest"), "https://git.unknown.example/org/app.git", ChangelogVars{})
		assert.Empty(t, got)
	})

	t.Run("unsafe tag falls back to the release index", func(t *testing.T) {
		t.Parallel()

		got := Changelog(testContainer(t, nil, "app:latest"), "https://github.com/org/app.git", ChangelogVars{Tag: "../other"})
		assert.Equal(t, "https://github.com/org/app/releases", got)
	})
}

func TestApplyVars(t *testing.T) {
	t.Parallel()

	got := applyVars("https://ex/{major}.{minor}.{patch}/{tag}/{commit}", ChangelogVars{
		Tag:    "v1.2.3",
		Commit: "deadbeef",
	})
	assert.Equal(t, "https://ex/1.2.3/v1.2.3/deadbeef", got)

	got = applyVars("https://ex/{tag}/{commit}/{major}", ChangelogVars{})
	assert.Equal(t, "https://ex/{tag}/{commit}/{major}", got)
}

func TestOrUnchanged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "{tag}", orUnchanged("", "{tag}"))
	assert.Equal(t, "v1", orUnchanged("v1", "{tag}"))
}

func TestSplitSemverParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tag   string
		major string
		minor string
		patch string
	}{
		{name: "prefixed", tag: "v1.2.3", major: "1", minor: "2", patch: "3"},
		{name: "unprefixed", tag: "1.2.3", major: "1", minor: "2", patch: "3"},
		{name: "prerelease", tag: "v1.2.3-rc.1", major: "1", minor: "2", patch: "3"},
		{name: "major only", tag: "v2", major: "2", minor: "0", patch: "0"},
		{name: "invalid", tag: "nightly"},
		{name: "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			major, minor, patch := splitSemverParts(tt.tag)
			assert.Equal(t, tt.major, major)
			assert.Equal(t, tt.minor, minor)
			assert.Equal(t, tt.patch, patch)
		})
	}
}

func TestCanonicalizeSemver(t *testing.T) {
	t.Parallel()

	assert.Empty(t, canonicalizeSemver(""))
	assert.Empty(t, canonicalizeSemver("  "))
	assert.Equal(t, "v1.2.3", canonicalizeSemver("v1.2.3"))
	assert.Equal(t, "v1.2.3", canonicalizeSemver("1.2.3"))
	assert.Equal(t, "nightly", canonicalizeSemver("nightly"))
}

func TestDerivedReleasesURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      string
		apiOrigin string
		tag       string
		want      string
	}{
		{name: "github", repo: "https://github.com/org/app.git", want: "https://github.com/org/app/releases"},
		{name: "gitlab", repo: "https://gitlab.com/org/app.git", want: "https://gitlab.com/org/app/-/releases"},
		{name: "codeberg", repo: "https://codeberg.org/org/app.git", want: "https://codeberg.org/org/app/releases"},
		{name: "github versioned", repo: "https://github.com/org/app.git", tag: "v1.2.3", want: "https://github.com/org/app/releases/tag/v1.2.3"},
		{name: "github versioned unprefixed", repo: "https://github.com/org/app.git", tag: "1.2.3", want: "https://github.com/org/app/releases/tag/1.2.3"},
		{name: "gitlab versioned", repo: "https://gitlab.com/org/app.git", tag: "v1.2.3", want: "https://gitlab.com/org/app/-/releases/v1.2.3"},
		{name: "codeberg versioned", repo: "https://codeberg.org/org/app.git", tag: "v1.2.3", want: "https://codeberg.org/org/app/releases/tag/v1.2.3"},
		{name: "hierarchical tag", repo: "https://github.com/org/app.git", tag: "release/1.2.3", want: "https://github.com/org/app/releases/tag/release/1.2.3"},
		{name: "dot-dot tag", repo: "https://github.com/org/app.git", tag: "../../secret", want: "https://github.com/org/app/releases"},
		{name: "dot tag", repo: "https://github.com/org/app.git", tag: ".", want: "https://github.com/org/app/releases"},
		{name: "tag with query", repo: "https://github.com/org/app.git", tag: "1.2.3?a=b", want: "https://github.com/org/app/releases"},
		{name: "tag with space", repo: "https://github.com/org/app.git", tag: "1.2.3 x", want: "https://github.com/org/app/releases"},
		{name: "tag with fragment", repo: "https://github.com/org/app.git", tag: "1.2.3#f", want: "https://github.com/org/app/releases"},
		{name: "tag with control character", repo: "https://gitlab.com/org/app.git", tag: "\x01", want: "https://gitlab.com/org/app/-/releases"},
		{name: "gitlab traversal", repo: "https://gitlab.com/org/app.git", tag: "..", want: "https://gitlab.com/org/app/-/releases"},
		{
			name:      "git-host label origin",
			repo:      "git@git.example.com:org/app.git",
			apiOrigin: "https://git.example.com:3000/gitea",
			want:      "https://git.example.com:3000/gitea/org/app/releases",
		},
		{
			name:      "git-host label origin versioned",
			repo:      "git@git.example.com:org/app.git",
			apiOrigin: "https://git.example.com:3000/gitea",
			tag:       "v1.2.3",
			want:      "https://git.example.com:3000/gitea/org/app/releases/tag/v1.2.3",
		},
		{
			name:      "self-hosted GitLab path",
			repo:      "https://gitlab.example.com/group/app.git",
			apiOrigin: "https://gitlab.example.com/gitlab",
			want:      "https://gitlab.example.com/gitlab/group/app/-/releases",
		},
		{
			name:      "self-hosted GitLab path versioned",
			repo:      "https://gitlab.example.com/group/app.git",
			apiOrigin: "https://gitlab.example.com/gitlab",
			tag:       "v1.2.3",
			want:      "https://gitlab.example.com/gitlab/group/app/-/releases/v1.2.3",
		},
		{
			name:      "known GitHub API path is not in releases URL",
			repo:      "https://github.com/org/app.git",
			apiOrigin: "https://github.com/api/v3",
			want:      "https://github.com/org/app/releases",
		},
		{
			name:      "mismatched git-host",
			repo:      "https://github.com/org/app.git",
			apiOrigin: "https://git.example.com/gitlab",
		},
		{
			name:      "pathless unknown git-host",
			repo:      "git@git.example.com:org/app.git",
			apiOrigin: "https://git.example.com:3000",
		},
		{
			name:      "unknown git-host path",
			repo:      "https://git.example.com/org/app.git",
			apiOrigin: "https://git.example.com/internal",
		},
		{name: "unknown", repo: "https://git.unknown.example/org/app.git"},
		{name: "unknown with tag", repo: "https://git.unknown.example/org/app.git", tag: "v1.2.3"},
		{name: "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, derivedReleasesURL(tt.repo, tt.apiOrigin, tt.tag))
		})
	}
}

func TestSafeReleaseTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag  string
		want bool
	}{
		{tag: "v1.2.3", want: true},
		{tag: "1.2.3", want: true},
		{tag: "v1.2.3-rc.1", want: true},
		{tag: "release/1.2.3", want: true},
		{tag: "2026.09.1", want: true},
		{tag: "nightly", want: true},
		{tag: ""},
		{tag: "/"},
		{tag: "//"},
		{tag: "a//b"},
		{tag: "."},
		{tag: ".."},
		{tag: "a/../b"},
		{tag: "a/./b"},
		{tag: "1.2.3 x"},
		{tag: "1.2.3?a=b"},
		{tag: "1.2.3#f"},
		{tag: "1.2.3\\x"},
		{tag: "1.2.3%2f"},
		{tag: "1.2.3\x00"},
		{tag: "1.2.3\x01"},
		{tag: "1.2.3\n"},
		{tag: "1.2.3\x1f"},
		{tag: "1.2.3\x7f"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, safeReleaseTag(tt.tag))
		})
	}
}

func TestParseRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		host  string
		owner string
		repo  string
		ok    bool
	}{
		{name: "https", raw: "https://github.com/org/app.git", host: "github.com", owner: "org", repo: "app", ok: true},
		{name: "https with port", raw: "https://git.example.com:8443/org/app.git", host: "git.example.com:8443", owner: "org", repo: "app", ok: true},
		{name: "scp", raw: "git@github.com:org/app.git", host: "github.com", owner: "org", repo: "app", ok: true},
		{name: "scp with another user", raw: "deploy@git.example.com:org/app.git", host: "git.example.com", owner: "org", repo: "app", ok: true},
		{name: "scp nested group with another user", raw: "gitlab@gitlab.example.com:group/sub/app.git", host: "gitlab.example.com", owner: "group/sub", repo: "app", ok: true},
		{name: "scp uppercase host", raw: "deploy@Git.Example.com:org/app", host: "git.example.com", owner: "org", repo: "app", ok: true},
		{name: "ssh url with user and port", raw: "ssh://deploy@git.example.com:2222/org/app.git", host: "git.example.com:2222", owner: "org", repo: "app", ok: true},
		{name: "ssh url", raw: "ssh://git@gitlab.com/group/app.git", host: "gitlab.com", owner: "group", repo: "app", ok: true},
		{name: "no scheme", raw: "github.com/org/app", host: "github.com", owner: "org", repo: "app", ok: true},
		{name: "nested group", raw: "https://gitlab.com/a/b/c.git", host: "gitlab.com", owner: "a/b", repo: "c", ok: true},
		{name: "scp empty host", raw: "git@:org/app.git"},
		{name: "scp missing colon", raw: "git@github.com"},
		{name: "scp empty host with another user", raw: "deploy@:org/app.git"},
		{name: "scp missing path with another user", raw: "deploy@git.example.com:"},
		{name: "scp empty user", raw: "@git.example.com:org/app.git"},
		{name: "scp at sign in host", raw: "a@b@git.example.com:org/app.git"},
		{name: "host only", raw: "https://github.com"},
		{name: "scheme without host", raw: "https://"},
		{name: "empty"},
		{name: "whitespace", raw: "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, owner, name, ok := parseRepo(tt.raw)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.owner, owner)
			assert.Equal(t, tt.repo, name)
		})
	}
}

func TestSplitOwnerRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		host  string
		path  string
		owner string
		repo  string
		ok    bool
	}{
		{name: "simple", host: "github.com", path: "org/app.git", owner: "org", repo: "app", ok: true},
		{name: "leading slash", host: "github.com", path: "/org/app", owner: "org", repo: "app", ok: true},
		{name: "nested", host: "gitlab.com", path: "group/sub/repo", owner: "group/sub", repo: "repo", ok: true},
		{name: "no slash", host: "github.com", path: "app"},
		{name: "trailing slash", host: "github.com", path: "org/"},
		{name: "empty path", host: "github.com", path: ""},
		{name: "dot segment", host: "github.com", path: "org/./app"},
		{name: "parent segment", host: "github.com", path: "org/../app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, owner, name, ok := splitOwnerRepo(tt.host, tt.path)
			assert.Equal(t, tt.ok, ok)

			if !tt.ok {
				assert.Empty(t, host)
				assert.Empty(t, owner)
				assert.Empty(t, name)

				return
			}

			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.owner, owner)
			assert.Equal(t, tt.repo, name)
		})
	}
}
