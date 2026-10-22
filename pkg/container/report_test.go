package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestResolveReportMeta verifies the report metadata for a container: its Git
// source, from labels, mappings, or OCI annotations, the current and latest
// versions, and the changelog URL, with credentials redacted throughout.
func TestResolveReportMeta(t *testing.T) {
	t.Parallel()

	t.Run("returns empty metadata for a nil container", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, ReportMeta{}, ResolveReportMeta(nil, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}))
	})

	t.Run("prefers a git-repo label over mapping and OCI source", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("myapp:latest"),
			WithLabels(map[string]string{
				git.RepoLabel:         "https://github.com/org/from-label.git",
				git.RefLabel:          "release",
				git.ChangelogURLLabel: "https://example.com/changelog/{tag}",
			}),
			WithImageLabels(map[string]string{
				oci.SourceLabel: "https://github.com/org/from-oci.git",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{
			GitImages: map[string]types.GitImage{
				"myapp:latest": {
					Repo: "https://github.com/org/from-map.git",
					Ref:  "mapped",
				},
			},
		}, ChangelogVars{Tag: "v1.2.3"}, oci.Annotations{})

		assert.Equal(t, "https://github.com/org/from-label.git", meta.GitRepo)
		assert.Equal(t, "release", meta.GitRef)
		assert.Equal(t, "https://example.com/changelog/v1.2.3", meta.Changelog)
		assert.Equal(t, "https://github.com/org/from-oci.git", meta.Source)
	})

	t.Run("preserves bare local Git repository paths", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithLabels(map[string]string{
				git.RepoLabel: "../repos/app?release=1",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{})
		assert.Equal(t, "../repos/app?release=1", meta.GitRepo)
	})

	t.Run("redacts credentials and query tokens from Git metadata", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithLabels(map[string]string{
				git.RepoLabel:         "https://user:secret@github.com/org/app.git?access_token=query#main",
				git.ChangelogURLLabel: "https://example.com/notes?version=2#install",
			}),
			WithImageLabels(map[string]string{
				oci.SourceLabel:        "https://source:source-secret@example.com/org/source.git?token=source-query",
				oci.URLLabel:           "https://example.com/image?format=json&author=team#download",
				oci.DocumentationLabel: "https://example.com/docs?lang=en#install",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{})
		assert.Equal(t, "https://github.com/org/app.git#main", meta.GitRepo)
		assert.NotContains(t, meta.GitRepo, "secret")
		assert.NotContains(t, meta.GitRepo, "query")
		assert.Equal(t, "https://example.com/notes?version=2#install", meta.Changelog)
		assert.Equal(t, "https://example.com/org/source.git", meta.Source)
		assert.Equal(t, "https://example.com/image?format=json&author=team#download", meta.ImageURL)
		assert.Equal(t, "https://example.com/docs?lang=en#install", meta.Documentation)
	})

	t.Run("uses an image mapping then OCI source when labels are absent", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("myapp:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel:        "https://github.com/org/from-oci.git",
				oci.URLLabel:           "https://example.com/image",
				oci.DocumentationLabel: "https://example.com/docs",
				oci.RevisionLabel:      "abc123",
				oci.VersionLabel:       "v2.0.0",
			}),
		)

		mapped := ResolveReportMeta(c, types.UpdateParams{
			GitImages: map[string]types.GitImage{
				"myapp:latest": {Repo: "https://github.com/org/from-map.git", Ref: "mapped"},
			},
		}, ChangelogVars{}, oci.Annotations{})
		assert.Equal(t, "https://github.com/org/from-map.git", mapped.GitRepo)
		assert.Equal(t, "mapped", mapped.GitRef)
		assert.Equal(t, "https://github.com/org/from-map/releases", mapped.Changelog)

		ociOnly := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{})
		assert.Equal(t, "https://github.com/org/from-oci.git", ociOnly.GitRepo)
		assert.Empty(t, ociOnly.GitRef)
		assert.Equal(t, "https://github.com/org/from-oci/releases", ociOnly.Changelog)
		assert.Equal(t, "https://github.com/org/from-oci.git", ociOnly.Source)
		assert.Equal(t, "https://example.com/image", ociOnly.ImageURL)
		assert.Equal(t, "https://example.com/docs", ociOnly.Documentation)
		assert.Equal(t, "abc123", ociOnly.CurrentRevision)
		assert.Empty(t, ociOnly.LatestRevision)
		assert.Equal(t, "v2.0.0", ociOnly.CurrentVersion)
		assert.Empty(t, ociOnly.LatestVersion)
	})

	t.Run("reports current and latest versions from separate images", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.RevisionLabel: "old123",
				oci.VersionLabel:  "10.11.5",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{
			Revision: "new456",
			Version:  "10.11.6",
		})

		assert.Equal(t, "10.11.5", meta.CurrentVersion)
		assert.Equal(t, "10.11.6", meta.LatestVersion)
		assert.Equal(t, "old123", meta.CurrentRevision)
		assert.Equal(t, "new456", meta.LatestRevision)
	})

	t.Run("resolves project-scoped fields from the latest image", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel:        "https://github.com/org/old.git",
				oci.URLLabel:           "https://example.com/old",
				oci.DocumentationLabel: "https://example.com/old-docs",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{
			Source:        "https://github.com/org/new.git",
			URL:           "https://example.com/new",
			Documentation: "https://example.com/new-docs",
			Version:       "1.0.0",
		})

		assert.Equal(t, "https://github.com/org/new.git", meta.GitRepo)
		assert.Equal(t, "https://github.com/org/new.git", meta.Source)
		assert.Equal(t, "https://example.com/new", meta.ImageURL)
		assert.Equal(t, "https://example.com/new-docs", meta.Documentation)
		assert.Equal(t, "https://github.com/org/new/releases", meta.Changelog)
	})

	t.Run("falls back per field when the latest image omits an annotation", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel: "https://github.com/org/app.git",
				oci.URLLabel:    "https://example.com/image",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{Version: "1.0.0"})

		assert.Equal(t, "https://github.com/org/app.git", meta.Source)
		assert.Equal(t, "https://example.com/image", meta.ImageURL)
		assert.Equal(t, "1.0.0", meta.LatestVersion)
	})

	t.Run("derives a versioned changelog when a release tag is known", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel:  "https://github.com/org/app.git",
				oci.VersionLabel: "10.11.6",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{Tag: "v10.11.6"}, oci.Annotations{
			Version: "10.11.6",
		})
		assert.Equal(t, "https://github.com/org/app/releases/tag/v10.11.6", meta.Changelog)
	})

	t.Run("derives changelog URLs for known and mapped hosts", func(t *testing.T) {
		t.Parallel()

		gitlab := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "https://gitlab.com/org/app.git",
		}))
		assert.Equal(t, "https://gitlab.com/org/app/-/releases", ResolveReportMeta(gitlab, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)

		codeberg := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "https://codeberg.org/org/app.git",
		}))
		assert.Equal(t, "https://codeberg.org/org/app/releases", ResolveReportMeta(codeberg, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)

		mappedHost := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "git@git.example.com:org/app.git",
			git.HostLabel: "https://git.example.com:3000/gitea",
		}))
		assert.Equal(t, "https://git.example.com:3000/gitea/org/app/releases", ResolveReportMeta(mappedHost, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)

		ghe := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "https://github.company.com/org/app.git",
			git.HostLabel: "https://github.company.com/github",
		}))
		assert.Equal(t, "https://github.company.com/github/org/app/releases", ResolveReportMeta(ghe, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)

		nested := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "https://gitlab.com/group/sub/repo.git",
		}))
		assert.Equal(t, "https://gitlab.com/group/sub/repo/-/releases", ResolveReportMeta(nested, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)
	})

	t.Run("redacts credentials from the new image's annotations", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel: "https://github.com/org/old.git",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{
			Source:        "https://newuser:newscret@github.com/org/new.git?token=query#main",
			URL:           "https://example.com/image?signature=abc&format=json",
			Documentation: "https://example.com/docs?lang=en#install",
		})

		// The new image's values are the ones reported, so redaction must reach
		// them. Userinfo goes and sensitive query keys go, benign ones stay.
		assert.Equal(t, "https://github.com/org/new.git#main", meta.GitRepo)
		assert.NotContains(t, meta.GitRepo, "newscret")
		assert.NotContains(t, meta.GitRepo, "query")
		assert.Equal(t, "https://example.com/image?format=json", meta.ImageURL)
		assert.NotContains(t, meta.ImageURL, "signature")
		assert.Equal(t, "https://example.com/docs?lang=en#install", meta.Documentation)
	})

	t.Run("does not persist an SCP URL whose user is a token", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, redactGitURL("token@github.com:org/app.git"))
		assert.Empty(t, redactGitURL("ghp_abcdefghijklmnopqrstuvwxyz@github.com:org/app.git"))
		assert.Equal(t, "git@github.com:org/app.git", redactGitURL("git@github.com:org/app.git"))
		assert.Equal(t, "deploy@git.example.com:org/app.git", redactGitURL("deploy@git.example.com:org/app.git"))
	})

	t.Run("leaves changelog empty for an unknown host without OCI URL fields", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithImageName("app:latest"), WithLabels(map[string]string{
			git.RepoLabel: "https://unknown.example/org/app.git",
		}))
		assert.Empty(t, ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)
	})

	t.Run("falls back to OCI URL then documentation for changelog", func(t *testing.T) {
		t.Parallel()

		urlOnly := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.URLLabel: "https://example.com/image",
			}),
		)
		assert.Equal(t, "https://example.com/image", ResolveReportMeta(urlOnly, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)

		docsOnly := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.DocumentationLabel: "https://example.com/docs",
			}),
		)
		assert.Equal(t, "https://example.com/docs", ResolveReportMeta(docsOnly, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{}).Changelog)
	})

	t.Run("uses a mapping ref when the container is not watcher-associated", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithImageName("myapp:latest"))

		meta := ResolveReportMeta(c, types.UpdateParams{
			GitImages: map[string]types.GitImage{
				"myapp:latest": {Ref: "from-map"},
			},
		}, ChangelogVars{}, oci.Annotations{})
		assert.Empty(t, meta.GitRepo)
		assert.Equal(t, "from-map", meta.GitRef)
	})

	t.Run("never uses an OCI version as a display ref", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.VersionLabel: "1.4.0",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{})
		assert.Empty(t, meta.GitRef)
		assert.Equal(t, "1.4.0", meta.CurrentVersion)
	})

	t.Run("does not treat OCI source as a watcher association", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(
			WithImageName("app:latest"),
			WithImageLabels(map[string]string{
				oci.SourceLabel: "https://github.com/org/from-oci.git",
			}),
		)

		meta := ResolveReportMeta(c, types.UpdateParams{}, ChangelogVars{}, oci.Annotations{})
		assert.Equal(t, "https://github.com/org/from-oci.git", meta.GitRepo)

		assoc, ok := git.ResolveAssociation(c, types.UpdateParams{})
		assert.False(t, ok)
		assert.Equal(t, git.Association{}, assoc)
	})
}
