package git

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestResolveViaAPI(t *testing.T) {
	t.Parallel()

	t.Run("unparseable repo", func(t *testing.T) {
		t.Parallel()

		resolved, found, err := (&Client{}).resolveViaAPI(t.Context(), "not-a-repo", "main", "")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})

	t.Run("unknown host uses go-git", func(t *testing.T) {
		t.Parallel()

		resolved, found, err := (&Client{}).resolveViaAPI(t.Context(), "https://git.unknown.example/org/app.git", "main", "")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})

	t.Run("github branch", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{"sha": "abc123", "type": "commit"},
			})
		})

		client := newDetectClient(t, mux)
		resolved, found, err := client.resolveViaAPI(t.Context(), "https://github.com/org/app.git", "main", "")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "main", resolved.Name)
		assert.Equal(t, "abc123", resolved.Hash)
		assert.Equal(t, kindBranch, resolved.Kind)
	})

	t.Run("github falls back to tag", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/heads/v1.2.3", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		mux.HandleFunc("/repos/org/app/git/ref/tags/v1.2.3", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{"sha": "tagsha", "type": "commit"},
			})
		})

		client := newDetectClient(t, mux)
		resolved, found, err := client.resolveViaAPI(t.Context(), "https://github.com/org/app.git", "v1.2.3", "")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, kindTag, resolved.Kind)
		assert.Equal(t, "tagsha", resolved.Hash)
	})

	t.Run("gitlab extra host", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/repository/branches/") {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			writeJSON(w, map[string]any{"commit": map[string]string{"id": "glsha"}})
		})

		client := newDetectClient(t, mux)
		client.opts.Hosts = map[string]string{"gitlab.internal": types.GitHostGitLab}

		resolved, found, err := client.resolveViaAPI(t.Context(), "https://gitlab.internal/group/app.git", "main", "https://gitlab.internal")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "glsha", resolved.Hash)
		assert.Equal(t, kindBranch, resolved.Kind)
	})

	t.Run("gitea extra host", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/git/refs/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"ref": "refs/heads/main", "object": map[string]string{"sha": "giteasha", "type": "commit"}},
			})
		})

		client := newDetectClient(t, mux)
		client.opts.Hosts = map[string]string{"git.example.com": types.GitHostGitea}

		resolved, found, err := client.resolveViaAPI(t.Context(), "https://git.example.com/org/app.git", "main", "https://git.example.com")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "giteasha", resolved.Hash)
		assert.Equal(t, kindBranch, resolved.Kind)
	})
}

func TestListTagsViaAPI(t *testing.T) {
	t.Parallel()

	t.Run("unparseable repo", func(t *testing.T) {
		t.Parallel()

		tags, used, err := (&Client{}).listTagsViaAPI(t.Context(), "", "")
		require.NoError(t, err)
		assert.False(t, used)
		assert.Nil(t, tags)
	})

	t.Run("unknown host", func(t *testing.T) {
		t.Parallel()

		tags, used, err := (&Client{}).listTagsViaAPI(t.Context(), "https://git.unknown.example/org/app.git", "")
		require.NoError(t, err)
		assert.False(t, used)
		assert.Nil(t, tags)
	})

	t.Run("github", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "aaa"}},
			})
		})

		client := newDetectClient(t, mux)
		tags, used, err := client.listTagsViaAPI(t.Context(), "https://github.com/org/app.git", "")
		require.NoError(t, err)
		require.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name)
		assert.Equal(t, "aaa", tags[0].Hash)
	})

	t.Run("gitlab", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/repository/tags") {
				w.WriteHeader(http.StatusNotFound)

				return
			}

			writeJSON(w, []map[string]any{
				{"name": "v2.0.0", "commit": map[string]string{"id": "bbb"}},
			})
		})

		client := newDetectClient(t, mux)
		tags, used, err := client.listTagsViaAPI(t.Context(), "https://gitlab.com/org/app.git", "")
		require.NoError(t, err)
		require.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, "bbb", tags[0].Hash)
	})

	t.Run("gitea", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"name": "v3.0.0", "commit": map[string]string{"sha": "ccc"}},
			})
		})

		client := newDetectClient(t, mux)
		client.opts.Hosts = map[string]string{"code.local": types.GitHostGitea}

		tags, used, err := client.listTagsViaAPI(t.Context(), "https://code.local/org/app.git", "https://code.local")
		require.NoError(t, err)
		require.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, "v3.0.0", tags[0].Name)
	})
}

func TestSplitRepoParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		repo  string
		host  string
		owner string
		repoN string
		ok    bool
	}{
		{name: "https", repo: "https://github.com/org/app.git", host: "github.com", owner: "org", repoN: "app", ok: true},
		{name: "ssh scp", repo: "git@gitlab.com:group/app.git", host: "gitlab.com", owner: "group", repoN: "app", ok: true},
		{name: "nested group", repo: "https://gitlab.com/a/b/c.git", host: "gitlab.com", owner: "a/b", repoN: "c", ok: true},
		{name: "empty", repo: ""},
		{name: "host only", repo: "https://github.com"},
		{name: "no owner", repo: "https://github.com/app.git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host, owner, name, ok := splitRepo(tt.repo)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.host, host)
			assert.Equal(t, tt.owner, owner)
			assert.Equal(t, tt.repoN, name)
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
		{name: "strips git suffix", host: "github.com", path: "/org/app.git", owner: "org", repo: "app", ok: true},
		{name: "nested owner", host: "gitlab.com", path: "group/sub/repo", owner: "group/sub", repo: "repo", ok: true},
		{name: "no slash", host: "github.com", path: "app"},
		{name: "trailing slash", host: "github.com", path: "org/"},
		{name: "empty path", host: "github.com", path: ""},
		{name: "leading slash only", host: "github.com", path: "/"},
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

func TestGithubRef(t *testing.T) {
	t.Parallel()

	t.Run("commit object", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{"sha": "commitsha", "type": "commit"},
			})
		})

		resolved, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "heads/main")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "main", resolved.Name)
		assert.Equal(t, "commitsha", resolved.Hash)
	})

	t.Run("peels annotated tag", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{
					"sha":  "tagobject",
					"type": "tag",
					"url":  "https://api.github.com/repos/org/app/git/tags/tagobject",
				},
			})
		})
		mux.HandleFunc("/repos/org/app/git/tags/tagobject", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]string{"sha": "peeled", "type": "commit"},
			})
		})

		resolved, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "tags/v1.0.0")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "v1.0.0", resolved.Name)
		assert.Equal(t, "peeled", resolved.Hash)
	})

	t.Run("rejects an annotated tag targeting a tree", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/tags/tree", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{
					"sha":  "tagobject",
					"type": "tag",
					"url":  "https://api.github.com/repos/org/app/git/tags/tagobject",
				},
			})
		})
		mux.HandleFunc("/repos/org/app/git/tags/tagobject", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]string{"sha": "treeobject", "type": "tree"},
			})
		})

		resolved, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "tags/tree")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})

	t.Run("unpeeled annotated tag is not found", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{
				"object": map[string]any{
					"sha":  "tagobject",
					"type": "tag",
					"url":  "https://api.github.com/repos/org/app/git/tags/missing",
				},
			})
		})

		resolved, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "tags/v1.0.0")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})

	t.Run("empty sha", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"object": map[string]any{}})
		})

		_, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "heads/main")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "heads/missing")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, found, err := newDetectClient(t, mux).githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "heads/main")
		require.Error(t, err)
		assert.False(t, found)
		require.ErrorIs(t, err, errAPIStatus)
	})
}

func TestGithubTags(t *testing.T) {
	t.Parallel()

	t.Run("single page", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "aaa"}},
			})
		})

		tags, used, err := newDetectClient(t, mux).githubTags(t.Context(), "github.com", url.URL{}, "org", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, RemoteRef{Name: "v1.0.0", Hash: "aaa"}, tags[0])
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})

		_, used, err := newDetectClient(t, mux).githubTags(t.Context(), "github.com", url.URL{}, "org", "app")
		require.Error(t, err)
		assert.False(t, used)
	})
}

func TestGitlabRef(t *testing.T) {
	t.Parallel()

	t.Run("branch", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.EscapedPath(), "/repository/branches/main")
			writeJSON(w, map[string]any{"commit": map[string]string{"id": "glsha"}})
		})

		resolved, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "main")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "main", resolved.Name)
		assert.Equal(t, kindBranch, resolved.Kind)
		assert.Equal(t, "glsha", resolved.Hash)
	})

	t.Run("version tag", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case strings.Contains(path, "/repository/branches/v1.2.3"):
				w.WriteHeader(http.StatusNotFound)
			case strings.Contains(path, "/repository/tags/v1.2.3"):
				writeJSON(w, map[string]any{"commit": map[string]string{"id": "tagsha"}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		resolved, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "v1.2.3")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, kindTag, resolved.Kind)
		assert.Equal(t, "tagsha", resolved.Hash)
	})

	t.Run("non-semver tag", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case strings.Contains(path, "/repository/branches/release"):
				w.WriteHeader(http.StatusNotFound)
			case strings.Contains(path, "/repository/tags/release"):
				writeJSON(w, map[string]any{"commit": map[string]string{"id": "releasesha"}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		resolved, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "release")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "release", resolved.Name)
		assert.Equal(t, kindTag, resolved.Kind)
		assert.Equal(t, "releasesha", resolved.Hash)
	})

	t.Run("same-named branch takes precedence", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case strings.Contains(path, "/repository/branches/release"):
				writeJSON(w, map[string]any{"commit": map[string]string{"id": "branchsha"}})
			case strings.Contains(path, "/repository/tags/release"):
				writeJSON(w, map[string]any{"commit": map[string]string{"id": "tagsha"}})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		resolved, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "release")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "release", resolved.Name)
		assert.Equal(t, kindBranch, resolved.Kind)
		assert.Equal(t, "branchsha", resolved.Hash)
	})

	t.Run("empty commit id", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case strings.Contains(path, "/repository/branches/main"):
				writeJSON(w, map[string]any{"commit": map[string]string{"id": ""}})
			case strings.Contains(path, "/repository/tags/main"):
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		_, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "main")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, found, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "missing")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		_, _, err := newDetectClient(t, mux).gitlabRef(t.Context(), "gitlab.com", url.URL{}, "org", "app", "main")
		require.ErrorIs(t, err, errAPIStatus)
	})
}

func TestGitlabTags(t *testing.T) {
	t.Parallel()

	t.Run("single page", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"id": "aaa"}},
			})
		})

		tags, used, err := newDetectClient(t, mux).gitlabTags(t.Context(), "gitlab.com", url.URL{}, "group", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, "aaa", tags[0].Hash)
	})

	t.Run("paginates via link header", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				writeJSON(w, []map[string]any{
					{"name": "v1.0.1", "commit": map[string]string{"id": "bbb"}},
				})

				return
			}

			w.Header().Set("Link", `<https://gitlab.com/api/v4/projects/group%2Fapp/repository/tags?page=2>; rel="next"`)
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"id": "aaa"}},
			})
		})

		tags, used, err := newDetectClient(t, mux).gitlabTags(t.Context(), "gitlab.com", url.URL{}, "group", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, 2)
		assert.Equal(t, "v1.0.1", tags[1].Name)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, used, err := newDetectClient(t, mux).gitlabTags(t.Context(), "gitlab.com", url.URL{}, "org", "app")
		require.Error(t, err)
		assert.False(t, used)
	})
}

func TestGiteaRef(t *testing.T) {
	t.Parallel()

	t.Run("branch", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/git/refs/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"object": map[string]string{"sha": "branchsha", "type": "commit"}},
			})
		})

		resolved, found, err := newDetectClient(t, mux).giteaRef(t.Context(), "git.example.com", url.URL{}, "org", "app", "main")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, kindBranch, resolved.Kind)
		assert.Equal(t, "branchsha", resolved.Hash)
	})

	t.Run("falls back to tag", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/git/refs/heads/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		mux.HandleFunc("/api/v1/repos/org/app/git/refs/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"object": map[string]string{"sha": "tagsha", "type": "commit"}},
			})
		})

		resolved, found, err := newDetectClient(t, mux).giteaRef(t.Context(), "git.example.com", url.URL{}, "org", "app", "v1.0.0")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, kindTag, resolved.Kind)
		assert.Equal(t, "tagsha", resolved.Hash)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, _, err := newDetectClient(t, mux).giteaRef(t.Context(), "git.example.com", url.URL{}, "org", "app", "main")
		require.ErrorIs(t, err, errAPIStatus)
	})
}

func TestGiteaRefs(t *testing.T) {
	t.Parallel()

	t.Run("empty body", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []any{})
		})

		_, found, err := newDetectClient(t, mux).giteaRefs(t.Context(), "https://git.example.com/refs", "main", kindBranch, "")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("empty sha", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{{"object": map[string]string{"sha": ""}}})
		})

		_, found, err := newDetectClient(t, mux).giteaRefs(t.Context(), "https://git.example.com/refs", "main", kindBranch, "")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("prefers peeled commit", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"ref": "refs/tags/v1.0.0", "object": map[string]string{"sha": "tagobject", "type": "tag"}},
				{"ref": "refs/tags/v1.0.0^{}", "object": map[string]string{"sha": "peeled", "type": "commit"}},
			})
		})

		resolved, found, err := newDetectClient(t, mux).giteaRefs(t.Context(), "https://git.example.com/refs", "v1.0.0", kindTag, "")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "peeled", resolved.Hash)
	})

	t.Run("rejects a peeled tree object", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"ref": "refs/tags/v1.0.0", "object": map[string]string{"sha": "tagobject", "type": "tag"}},
				{"ref": "refs/tags/v1.0.0^{}", "object": map[string]string{"sha": "treeobject", "type": "tree"}},
			})
		})

		resolved, found, err := newDetectClient(t, mux).giteaRefs(t.Context(), "https://git.example.com/refs", "v1.0.0", kindTag, "")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})

	t.Run("tag object only is not found", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"ref": "refs/tags/v1.0.0", "object": map[string]string{"sha": "tagobject", "type": "tag"}},
			})
		})

		resolved, found, err := newDetectClient(t, mux).giteaRefs(t.Context(), "https://git.example.com/refs", "v1.0.0", kindTag, "")
		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, resolved.Hash)
	})
}

func TestGiteaTags(t *testing.T) {
	t.Parallel()

	t.Run("single page", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/tags", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "aaa"}},
			})
		})

		tags, used, err := newDetectClient(t, mux).giteaTags(t.Context(), "git.example.com", url.URL{}, "org", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name)
	})

	t.Run("empty first page", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []any{})
		})

		tags, used, err := newDetectClient(t, mux).giteaTags(t.Context(), "git.example.com", url.URL{}, "org", "app")
		require.NoError(t, err)
		assert.True(t, used)
		assert.Empty(t, tags)
	})

	t.Run("paginates when the server caps pages at 50", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/tags", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, strconv.Itoa(giteaTagsPerPage), r.URL.Query().Get("limit"))

			page := r.URL.Query().Get("page")
			if page == "1" {
				items := make([]map[string]any, giteaTagsPerPage)
				for i := range items {
					items[i] = map[string]any{
						"name":   "v1.0.0",
						"commit": map[string]string{"sha": "aaa"},
					}
				}

				writeJSON(w, items)

				return
			}

			if page == "2" {
				items := make([]map[string]any, giteaTagsPerPage)
				for i := range items {
					items[i] = map[string]any{
						"name":   "v1.0.1",
						"commit": map[string]string{"sha": "bbb"},
					}
				}

				writeJSON(w, items)

				return
			}

			writeJSON(w, []map[string]any{
				{"name": "v1.0.2", "commit": map[string]string{"sha": "ccc"}},
			})
		})

		tags, used, err := newDetectClient(t, mux).giteaTags(t.Context(), "git.example.com", url.URL{}, "org", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, giteaTagsPerPage*2+1)
		assert.Equal(t, "v1.0.2", tags[len(tags)-1].Name)
	})

	t.Run("normalizes a Gitea Link page to the safe limit", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/repos/org/app/tags", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, strconv.Itoa(giteaTagsPerPage), r.URL.Query().Get("limit"))

			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", "<https://git.example.com/api/v1/repos/org/app/tags?page=2&limit=100>; rel=\"next\"")
				writeJSON(w, []map[string]any{
					{"name": "v1.0.0", "commit": map[string]string{"sha": "aaa"}},
				})

				return
			}

			writeJSON(w, []map[string]any{
				{"name": "v1.0.1", "commit": map[string]string{"sha": "bbb"}},
			})
		})

		client := newDetectClient(t, mux)
		tags, used, err := client.giteaTags(t.Context(), "git.example.com", url.URL{}, "org", "app")
		require.NoError(t, err)
		assert.True(t, used)
		require.Len(t, tags, 2)
		assert.Equal(t, "v1.0.1", tags[1].Name)
	})

	t.Run("http error", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		_, used, err := newDetectClient(t, mux).giteaTags(t.Context(), "git.example.com", url.URL{}, "org", "app")
		require.Error(t, err)
		assert.False(t, used)
	})
}

func TestGetJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Empty(t, r.Header.Get("Authorization"))
		writeJSON(w, map[string]string{"id": "ok"})
	}))
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{Token: "tok"})
	client.http = srv.Client()

	var body struct {
		ID string `json:"id"`
	}

	_, err := client.getJSONPage(t.Context(), srv.URL, &body, "")
	require.NoError(t, err)
	assert.Equal(t, "ok", body.ID)
}

func TestGetJSONPage(t *testing.T) {
	t.Parallel()

	t.Run("returns next link", func(t *testing.T) {
		t.Parallel()

		var srv *httptest.Server

		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Link", `<`+srv.URL+`/page2>; rel="next"`)
			writeJSON(w, map[string]string{"ok": "1"})
		}))
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = srv.Client()

		var dest map[string]string

		next, err := client.getJSONPage(t.Context(), srv.URL, &dest, "")
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(next, "/page2"))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = srv.Client()

		_, err := client.getJSONPage(t.Context(), srv.URL, &map[string]any{}, "")
		require.ErrorIs(t, err, errAPINotFound)
		require.ErrorIs(t, err, errRefNotFound)
	})

	t.Run("status error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = srv.Client()

		_, err := client.getJSONPage(t.Context(), srv.URL, &map[string]any{}, "")
		require.ErrorIs(t, err, errAPIStatus)
	})

	t.Run("decode error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = srv.Client()

		_, err := client.getJSONPage(t.Context(), srv.URL, &map[string]any{}, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "decode json:")
	})

	t.Run("http do error", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})}

		_, err := client.getJSONPage(t.Context(), "https://git.example.com/api", &map[string]any{}, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "http get:")
	})

	t.Run("invalid request url", func(t *testing.T) {
		t.Parallel()

		_, err := (&Client{}).getJSONPage(t.Context(), ":", &map[string]any{}, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "new request:")
	})
}

func TestSameOriginNext(t *testing.T) {
	t.Parallel()

	current := "https://api.github.com/repos/org/app/tags?page=1"
	assert.Equal(t, "https://api.github.com/repos/org/app/tags?page=2",
		sameOriginNext(current, "https://api.github.com/repos/org/app/tags?page=2"))
	assert.Empty(t, sameOriginNext(current, "https://evil.example/steal"))
	assert.Empty(t, sameOriginNext(current, "http://api.github.com/repos/org/app/tags?page=2"))
	assert.Empty(t, sameOriginNext(current, "file:///etc/passwd"))
	assert.Empty(t, sameOriginNext(current, ""))
	assert.Empty(t, sameOriginNext(":", "https://api.github.com/x"))
}

func TestGetJSONPageDropsCrossHostNext(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", `<https://evil.example/steal>; rel="next"`)
		writeJSON(w, map[string]string{"ok": "1"})
	}))
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{Token: "secret-token"})
	client.http = srv.Client()

	var dest map[string]string

	next, err := client.getJSONPage(t.Context(), srv.URL, &dest, "")
	require.NoError(t, err)
	assert.Empty(t, next)
}

func TestNextLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "quoted rel", header: `<https://ex/p2>; rel="next"`, want: "https://ex/p2"},
		{name: "unquoted rel", header: `<https://ex/p2>; rel=next`, want: "https://ex/p2"},
		{name: "among others", header: `<https://ex/p1>; rel="prev", <https://ex/p3>; rel="next"`, want: "https://ex/p3"},
		{name: "no next", header: `<https://ex/p1>; rel="prev"`},
		{name: "empty"},
		{name: "missing brackets", header: `https://ex/p2; rel="next"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, nextLink(tt.header))
		})
	}
}

// writeJSON writes v as a JSON 200 response.
//
// Parameters:
//   - w: Response writer.
//   - v: Value encoded as JSON.
func writeJSON(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
