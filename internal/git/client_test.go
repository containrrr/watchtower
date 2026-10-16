package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerImage "github.com/moby/moby/api/types/image"

	"github.com/nicholas-fedor/watchtower/internal/compose"
	gitPkg "github.com/nicholas-fedor/watchtower/pkg/container/git"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

type stubLister struct {
	refs []RemoteRef
	err  error
}

func (s stubLister) List(context.Context, string) ([]RemoteRef, error) {
	return s.refs, s.err
}

func testClient(t *testing.T, refs []RemoteRef) *Client {
	t.Helper()

	nop := zerolog.Nop()
	client := New(&nop, Options{})
	client.lister = stubLister{refs: refs}
	client.opts.Hosts = nil

	return client
}

func TestCheckContainer_RejectedComposeStampRetries(t *testing.T) {
	t.Parallel()

	const (
		dir           = "/srv/app"
		files         = "compose.yaml"
		previous      = "aaa1111111"
		otherPrevious = "ccc3333333"
		current       = "bbb2222222"
	)

	client := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: current}})
	api := labeledComposeMember(t, "web", "api", "1", dir, files, current)
	worker := labeledComposeMember(t, "web", "worker", "1", dir, files, current)
	other := labeledComposeMember(t, "other", "api", "1", dir, files, current)

	client.RejectApply(ApplyStampKeyFrom(api), current, previous, "")
	client.RejectApply(ApplyStampKeyFrom(worker), current, otherPrevious, "")
	client.RejectApply(ApplyStampKeyFrom(other), current, previous, "")

	got, err := CheckContainer(t.Context(), client, api, types.UpdateParams{})
	require.NoError(t, err)
	assert.True(t, got.Stale)
	assert.Equal(t, current, got.Commit)

	client.AcceptApply(ApplyStampKeyFrom(api), current)

	got, err = CheckContainer(t.Context(), client, api, types.UpdateParams{})
	require.NoError(t, err)
	assert.False(t, got.Stale)

	got, err = CheckContainer(t.Context(), client, worker, types.UpdateParams{})
	require.NoError(t, err)
	assert.True(t, got.Stale)

	got, err = CheckContainer(t.Context(), client, other, types.UpdateParams{})
	require.NoError(t, err)
	assert.True(t, got.Stale)
}

func labeledComposeMember(t *testing.T, project, service, number, dir, files, commit string) types.Container {
	t.Helper()

	return labeledContainer(t, map[string]string{
		gitPkg.RepoLabel:                "https://example.com/org/app.git",
		gitPkg.RefLabel:                 "main",
		gitPkg.LastCommitLabel:          commit,
		gitPkg.ComposeDirLabel:          dir,
		compose.ComposeProjectLabel:     project,
		compose.ComposeServiceLabel:     service,
		compose.ComposeContainerNumber:  number,
		compose.ComposeConfigFilesLabel: files,
	})
}

func TestCheck_NoStampIsNotStale(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/heads/main", Hash: "aaa111"},
	})

	result, err := client.Check(t.Context(), CheckRequest{
		Repo: "https://example.com/org/app.git",
		Ref:  "main",
	})
	require.NoError(t, err)
	assert.False(t, result.Stale)
	assert.Equal(t, "aaa111", result.Commit)
	assert.Equal(t, kindBranch, result.Kind)
	assert.Equal(t, "main", result.Ref)
}

func TestSameRevision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		observed  string
		remote    string
		wantEqual bool
	}{
		{name: "exact", observed: "abc123def456", remote: "abc123def456", wantEqual: true},
		{name: "case", observed: "ABC123DEF456", remote: "abc123def456", wantEqual: true},
		{name: "short prefix", observed: "abc123d", remote: "abc123def4567890", wantEqual: true},
		{name: "full contains short", observed: "abc123def4567890", remote: "abc123d", wantEqual: true},
		{name: "different", observed: "abc123def456", remote: "fff123def456", wantEqual: false},
		{name: "too short", observed: "abc", remote: "abcdef0", wantEqual: false},
		{name: "empty", observed: "", remote: "abc123def456", wantEqual: false},
		{name: "non hex", observed: "not-a-sha", remote: "not-a-sha", wantEqual: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantEqual, sameRevision(tt.observed, tt.remote))
		})
	}
}

func TestCheck_RejectsUnsafeRef(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	_, err := client.Check(t.Context(), CheckRequest{
		Repo: "https://example.com/org/app.git",
		Ref:  "foo..bar",
	})
	require.ErrorIs(t, err, ErrInvalidRef)
}

func TestCheck_BranchSHACompare(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/heads/main", Hash: "bbb222"},
	})

	fresh, err := client.Check(t.Context(), CheckRequest{
		Repo:       "https://example.com/org/app.git",
		Ref:        "main",
		LastCommit: "bbb222",
	})
	require.NoError(t, err)
	assert.False(t, fresh.Stale)

	stale, err := client.Check(t.Context(), CheckRequest{
		Repo:       "https://example.com/org/app.git",
		Ref:        "main",
		LastCommit: "old",
	})
	require.NoError(t, err)
	assert.True(t, stale.Stale)
	assert.Equal(t, "bbb222", stale.Commit)
}

func TestCheck_PrefersPeeledAnnotatedTag(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.0.0", Hash: "tagobject"},
		{Name: "refs/tags/v1.0.0^{}", Hash: "commitsha"},
	})

	result, err := client.Check(t.Context(), CheckRequest{
		Repo:       "https://example.com/org/app.git",
		Ref:        "v1.0.0",
		LastCommit: "old",
	})
	require.NoError(t, err)
	assert.Equal(t, "commitsha", result.Commit)
	assert.Equal(t, "v1.0.0", result.Tag)
	assert.Equal(t, kindTag, result.Kind)
}

func TestListTags_PrefersPeeledHash(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.2.3", Hash: "tagobject"},
		{Name: "refs/tags/v1.2.3^{}", Hash: "commitsha"},
		{Name: "refs/tags/v1.2.4", Hash: "plain"},
	})

	tags, err := client.listTags(t.Context(), "https://example.com/org/app.git", "")
	require.NoError(t, err)

	byName := map[string]string{}
	for _, tag := range tags {
		byName[tag.Name] = tag.Hash
	}

	assert.Equal(t, "commitsha", byName["v1.2.3"])
	assert.Equal(t, "plain", byName["v1.2.4"])
}

func TestCheck_ResolvesHeadsThenTags(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.0.0", Hash: "tagsha"},
	})

	result, err := client.Check(t.Context(), CheckRequest{
		Repo:       "https://example.com/org/app.git",
		Ref:        "v1.0.0",
		LastCommit: "old",
	})
	require.NoError(t, err)
	assert.True(t, result.Stale)
	assert.Equal(t, "tagsha", result.Commit)
	assert.Equal(t, "v1.0.0", result.Tag)
	assert.Equal(t, kindTag, result.Kind)
}

func TestCheck_MissingRef(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/heads/main", Hash: "aaa"},
	})

	_, err := client.Check(t.Context(), CheckRequest{
		Repo: "https://example.com/org/app.git",
		Ref:  "missing",
	})
	require.ErrorIs(t, err, ErrRefNotFound)
}

func TestCheck_TagPolicyIgnoresNonSemver(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.2.3", Hash: "old"},
		{Name: "refs/tags/v1.2.4", Hash: "new"},
		{Name: "refs/tags/nightly", Hash: "nope"},
		{Name: "refs/tags/v1.3.0", Hash: "minor"},
	})

	patch, err := client.Check(t.Context(), CheckRequest{
		Repo:    "https://example.com/org/app.git",
		Policy:  types.GitPolicyPatch,
		LastTag: "v1.2.3",
	})
	require.NoError(t, err)
	assert.True(t, patch.Stale)
	assert.Equal(t, "v1.2.4", patch.Tag)
	assert.Equal(t, "new", patch.Commit)

	none, err := client.Check(t.Context(), CheckRequest{
		Repo:    "https://example.com/org/app.git",
		Policy:  types.GitPolicyPatch,
		LastTag: "v1.2.4",
	})
	require.NoError(t, err)
	assert.False(t, none.Stale)
}

func TestNew_NilLogAndZeroTimeout(t *testing.T) {
	t.Parallel()

	client := New(nil, Options{})
	require.NotNil(t, client)
	require.NotNil(t, client.log)
	assert.NotNil(t, client.lister)
	assert.Equal(t, defaultGitTimeout, client.http.Timeout)
}

func TestCheck_NilClient(t *testing.T) {
	t.Parallel()

	var client *Client

	got, err := client.Check(t.Context(), CheckRequest{Repo: "https://example.com/org/app.git"})
	require.NoError(t, err)
	assert.Equal(t, CheckResult{}, got)
}

func TestCheck_EmptyRefDefaultsToMain(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: "aaa"}})
	got, err := client.Check(t.Context(), CheckRequest{Repo: "https://example.com/org/app.git"})
	require.NoError(t, err)
	assert.Equal(t, "main", got.Ref)
	assert.False(t, got.Stale)
}

func TestCheck_TagPolicyKnownCommitIsBaseline(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.2.3", Hash: "abc1111"},
		{Name: "refs/tags/v1.2.4", Hash: "def2222"},
		{Name: "refs/tags/nightly", Hash: "zzz9999"},
	})

	t.Run("empty tag matches selected commit", func(t *testing.T) {
		t.Parallel()

		got, err := client.Check(t.Context(), CheckRequest{
			Repo:       "https://example.com/org/app.git",
			Policy:     types.GitPolicyPatch,
			LastCommit: "def2222",
		})
		require.NoError(t, err)
		assert.False(t, got.Stale)
		assert.Equal(t, "v1.2.4", got.Tag)
		assert.Equal(t, "def2222", got.Commit)
	})

	t.Run("non-semver tag matches selected commit", func(t *testing.T) {
		t.Parallel()

		got, err := client.Check(t.Context(), CheckRequest{
			Repo:       "https://example.com/org/app.git",
			Policy:     types.GitPolicyPatch,
			LastTag:    "nightly",
			LastCommit: "def2222",
		})
		require.NoError(t, err)
		assert.False(t, got.Stale)
		assert.Equal(t, "v1.2.4", got.Tag)
		assert.Equal(t, "def2222", got.Commit)
	})

	t.Run("patch does not jump past the commit tag", func(t *testing.T) {
		t.Parallel()

		wider := testClient(t, []RemoteRef{
			{Name: "refs/tags/v1.2.3", Hash: "abc1111"},
			{Name: "refs/tags/v1.2.4", Hash: "def2222"},
			{Name: "refs/tags/v1.3.0", Hash: "ghi3333"},
		})
		got, err := wider.Check(t.Context(), CheckRequest{
			Repo:       "https://example.com/org/app.git",
			Policy:     types.GitPolicyPatch,
			LastCommit: "abc1111",
		})
		require.NoError(t, err)
		assert.True(t, got.Stale)
		assert.Equal(t, "v1.2.4", got.Tag)
		assert.Equal(t, "def2222", got.Commit)
	})

	t.Run("known commit differs from selected tag", func(t *testing.T) {
		t.Parallel()

		got, err := client.Check(t.Context(), CheckRequest{
			Repo:       "https://example.com/org/app.git",
			Policy:     types.GitPolicyPatch,
			LastCommit: "abc1111",
		})
		require.NoError(t, err)
		assert.True(t, got.Stale)
		assert.Equal(t, "v1.2.4", got.Tag)
		assert.Equal(t, "def2222", got.Commit)
	})

	t.Run("non-semver tag and empty commit records highest", func(t *testing.T) {
		t.Parallel()

		got, err := client.Check(t.Context(), CheckRequest{
			Repo:    "https://example.com/org/app.git",
			Policy:  types.GitPolicyMajor,
			LastTag: "nightly",
		})
		require.NoError(t, err)
		assert.False(t, got.Stale)
		assert.Equal(t, "v1.2.4", got.Tag)
		assert.Equal(t, "def2222", got.Commit)
	})
}

func TestCheck_MismatchedGitHostUsesLister(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/heads/main", Hash: "abc1111"},
	})
	client.opts.Token = "super-secret"
	called := false
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true

		if strings.Contains(r.Header.Get("Authorization"), "super-secret") {
			t.Errorf("token sent to %s", r.URL.Host)
		}

		return nil, errors.New("git-host probe")
	})}

	got, err := client.Check(t.Context(), CheckRequest{
		Repo:       "https://git.example.com/org/app.git",
		Ref:        "main",
		Host:       "https://evil.example",
		LastCommit: "old",
	})
	require.NoError(t, err)
	assert.False(t, called)
	assert.True(t, got.Stale)
	assert.Equal(t, "abc1111", got.Commit)
}

func TestLookupAPIHostMatch(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	client.opts.Hosts = map[string]string{"git.example.com": types.GitHostGitea}

	kind, origin, ok := client.lookupAPI(t.Context(), "git.example.com", "https://git.example.com:3000/gitea")
	require.True(t, ok)
	assert.Equal(t, types.GitHostGitea, kind)
	assert.Equal(t, "git.example.com:3000", origin.Host)

	_, _, ok = client.lookupAPI(t.Context(), "git.example.com", "https://evil.example")
	assert.False(t, ok)
}

func TestCheck_TagPolicyNoStampSelectsHighest(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/tags/v1.0.0", Hash: "old"},
		{Name: "refs/tags/v1.2.0", Hash: "new"},
	})

	got, err := client.Check(t.Context(), CheckRequest{
		Repo:   "https://example.com/org/app.git",
		Policy: types.GitPolicyMajor,
	})
	require.NoError(t, err)
	assert.False(t, got.Stale)
	assert.Equal(t, "v1.2.0", got.Tag)
	assert.Equal(t, "new", got.Commit)
	assert.Equal(t, kindTag, got.Kind)
}

func TestCheck_TagPolicyNoStampFallsBackToRef(t *testing.T) {
	t.Parallel()

	client := testClient(t, []RemoteRef{
		{Name: "refs/heads/main", Hash: "branchsha"},
		{Name: "refs/tags/nightly", Hash: "nope"},
	})

	got, err := client.Check(t.Context(), CheckRequest{
		Repo:   "https://example.com/org/app.git",
		Ref:    "main",
		Policy: types.GitPolicyPatch,
	})
	require.NoError(t, err)
	assert.False(t, got.Stale)
	assert.Equal(t, "branchsha", got.Commit)
	assert.Equal(t, kindBranch, got.Kind)
}

func TestCheck_TagPolicyListError(t *testing.T) {
	t.Parallel()

	client := testClient(t, nil)
	client.lister = stubLister{err: errors.New("ls failed")}

	_, err := client.Check(t.Context(), CheckRequest{
		Repo:   "https://example.com/org/app.git",
		Policy: types.GitPolicyPatch,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "ls-remote tags")
}

// TestRepositoryDiagnosticsAreSanitized verifies that diagnostic logs omit repository credentials while preserving a safe repository URL.
func TestRepositoryDiagnosticsAreSanitized(t *testing.T) {
	t.Parallel()

	repo := "https://user:debug-secret@github.com/org/app.git?token=debug-query#debug-fragment"

	tests := []struct {
		name string
		refs []RemoteRef
		run  func(*Client) error
	}{
		{
			name: "resolve ref",
			refs: []RemoteRef{{Name: "refs/heads/main", Hash: "abc111"}},
			run: func(client *Client) error {
				_, err := client.resolveRef(t.Context(), repo, "main", "")

				return err
			},
		},
		{
			name: "list tags",
			refs: []RemoteRef{{Name: "refs/tags/v1.0.0", Hash: "abc111"}},
			run: func(client *Client) error {
				_, err := client.listTags(t.Context(), repo, "")

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			logger := zerolog.New(&logs)
			client := New(&logger, Options{})
			client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("API unavailable")
			})}
			client.lister = stubLister{refs: tt.refs}

			require.NoError(t, tt.run(client))
			assert.NotContains(t, logs.String(), "debug-secret")
			assert.NotContains(t, logs.String(), "debug-query")
			assert.NotContains(t, logs.String(), "debug-fragment")
			assert.Contains(t, logs.String(), `"repo":"https://github.com/org/app.git"`)
		})
	}
}

// TestRepositoryErrorsAreSanitized verifies that remote errors omit repository credentials while retaining a safe repository URL.
func TestRepositoryErrorsAreSanitized(t *testing.T) {
	t.Parallel()

	repo := "https://user:error-secret@github.com/org/app.git?token=error-query#error-fragment"
	tests := []struct {
		name string
		run  func(*Client) error
	}{
		{
			name: "resolve ref",
			run: func(client *Client) error {
				_, err := client.resolveRef(t.Context(), repo, "main", "")

				return err
			},
		},
		{
			name: "list tags",
			run: func(client *Client) error {
				_, err := client.listTags(t.Context(), repo, "")

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := testClient(t, nil)
			client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("API unavailable")
			})}
			client.lister = stubLister{err: errors.New("remote rejected " + repo)}

			err := tt.run(client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "https://github.com/org/app.git")
			assert.NotContains(t, err.Error(), "error-secret")
			assert.NotContains(t, err.Error(), "error-query")
			assert.NotContains(t, err.Error(), "error-fragment")
		})
	}
}

func TestClone(t *testing.T) {
	t.Parallel()

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()

		var client *Client

		_, err := client.Clone(t.Context(), "https://example.com/org/app.git", CheckResult{})
		require.ErrorIs(t, err, ErrCloneFailed)
		assert.ErrorContains(t, err, "client is nil")
	})

	t.Run("checks out local repo", func(t *testing.T) {
		t.Parallel()

		src := initLocalRepo(t)
		dir, err := (&Client{}).Clone(t.Context(), src.dir, CheckResult{
			Commit: src.commit,
			Kind:   kindBranch,
			Ref:    src.branch,
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		got, err := os.ReadFile(filepath.Join(dir, "README"))
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(got))
	})

	t.Run("failed clone removes temp dir", func(t *testing.T) {
		t.Parallel()

		_, err := (&Client{}).Clone(t.Context(), filepath.Join(t.TempDir(), "missing.git"), CheckResult{
			Commit: "aaa",
			Kind:   kindBranch,
			Ref:    "main",
		})
		require.ErrorIs(t, err, ErrCloneFailed)
	})
}

func TestResolveRef(t *testing.T) {
	t.Parallel()

	t.Run("api success", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/git/ref/heads/main", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, map[string]any{"object": map[string]any{"sha": "apisha", "type": "commit"}})
		})

		client := newDetectClient(t, mux)
		client.lister = stubLister{err: errors.New("list should not run")}

		got, err := client.resolveRef(t.Context(), "https://github.com/org/app.git", "main", "")
		require.NoError(t, err)
		assert.Equal(t, "apisha", got.Hash)
		assert.Equal(t, kindBranch, got.Kind)
	})

	t.Run("api error falls back to ls-remote", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		client := newDetectClient(t, mux)
		client.lister = stubLister{refs: []RemoteRef{{Name: "refs/heads/main", Hash: "listed"}}}

		got, err := client.resolveRef(t.Context(), "https://github.com/org/app.git", "main", "")
		require.NoError(t, err)
		assert.Equal(t, "listed", got.Hash)
	})

	t.Run("list error", func(t *testing.T) {
		t.Parallel()

		client := testClient(t, nil)
		client.lister = stubLister{err: errors.New("offline")}

		_, err := client.resolveRef(t.Context(), "https://example.com/org/app.git", "main", "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "ls-remote")
	})
}

func TestListTagsFallbackAndAPI(t *testing.T) {
	t.Parallel()

	t.Run("api success", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, []map[string]any{
				{"name": "v1.0.0", "commit": map[string]string{"sha": "aaa"}},
			})
		})

		client := newDetectClient(t, mux)
		client.lister = stubLister{err: errors.New("list should not run")}

		tags, err := client.listTags(t.Context(), "https://github.com/org/app.git", "")
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "v1.0.0", tags[0].Name)
	})

	t.Run("api error falls back to ls-remote", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		})

		client := newDetectClient(t, mux)
		client.lister = stubLister{refs: []RemoteRef{{Name: "refs/tags/v1.0.0", Hash: "listed"}}}

		tags, err := client.listTags(t.Context(), "https://github.com/org/app.git", "")
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "listed", tags[0].Hash)
	})

	t.Run("list error", func(t *testing.T) {
		t.Parallel()

		client := testClient(t, nil)
		client.lister = stubLister{err: errors.New("offline")}

		_, err := client.listTags(t.Context(), "https://example.com/org/app.git", "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "ls-remote tags")
	})
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	t.Run("uses client timeout when parent has none", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{Timeout: 50 * time.Millisecond}}

		ctx, cancel := client.WithTimeout(t.Context())
		defer cancel()

		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), 50*time.Millisecond)
	})

	t.Run("keeps tighter parent deadline", func(t *testing.T) {
		t.Parallel()

		parent, stop := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer stop()

		client := &Client{opts: Options{Timeout: time.Hour}}

		ctx, cancel := client.WithTimeout(parent)
		defer cancel()

		parentDeadline, ok := parent.Deadline()
		require.True(t, ok)
		gotDeadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.Equal(t, parentDeadline, gotDeadline)
	})

	t.Run("zero timeout uses default", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := (&Client{}).WithTimeout(t.Context())
		defer cancel()

		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), defaultGitTimeout)
	})
}

func TestCheckContainer(t *testing.T) {
	t.Parallel()

	t.Run("not associated", func(t *testing.T) {
		t.Parallel()

		c := mockTypes.NewMockContainer(t)
		c.EXPECT().GetLabel(mock.Anything).Return("", false)
		c.EXPECT().ImageName().Return("nginx:latest")

		got, err := CheckContainer(t.Context(), testClient(t, nil), c, types.UpdateParams{})
		require.NoError(t, err)
		assert.Equal(t, CheckResult{}, got)
	})

	t.Run("uses repo label and stamp", func(t *testing.T) {
		t.Parallel()

		c := labeledContainer(t, map[string]string{
			gitPkg.RepoLabel:       "https://example.com/org/app.git",
			gitPkg.RefLabel:        "main",
			gitPkg.LastCommitLabel: "old",
		})

		client := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: "newsha"}})
		got, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.True(t, got.Stale)
		assert.Equal(t, "newsha", got.Commit)
	})

	t.Run("invalid policy is fail-closed", func(t *testing.T) {
		t.Parallel()

		c := labeledContainer(t, map[string]string{
			gitPkg.RepoLabel:         "https://example.com/org/app.git",
			gitPkg.SemverPolicyLabel: "nightly",
		})

		got, err := CheckContainer(t.Context(), testClient(t, nil), c, types.UpdateParams{})
		require.ErrorIs(t, err, ErrInvalidPolicy)
		assert.Equal(t, CheckResult{}, got)
	})

	t.Run("invalid git-host is fail-closed", func(t *testing.T) {
		t.Parallel()

		rawHost := "https://user:secret@git.example.com?token=secret=bad"
		c := labeledContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://example.com/org/app.git",
			gitPkg.HostLabel: rawHost,
		})

		got, err := CheckContainer(t.Context(), testClient(t, nil), c, types.UpdateParams{})
		require.ErrorIs(t, err, ErrInvalidHost)
		assert.NotContains(t, err.Error(), "secret")
		assert.NotContains(t, err.Error(), rawHost)
		assert.Equal(t, CheckResult{}, got)
	})

	t.Run("no stamp is not stale and is remembered", func(t *testing.T) {
		t.Parallel()

		c := labeledContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://example.com/org/app.git",
			gitPkg.RefLabel:  "main",
		})

		lister := &mutableLister{refs: []RemoteRef{{Name: "refs/heads/main", Hash: "aaa111"}}}
		client := testClient(t, nil)
		client.lister = lister

		first, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.False(t, first.Stale)
		assert.Equal(t, "aaa111", first.Commit)

		second, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.False(t, second.Stale)

		lister.refs = []RemoteRef{{Name: "refs/heads/main", Hash: "bbb222"}}
		third, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.True(t, third.Stale)
		assert.Equal(t, "bbb222", third.Commit)
	})

	t.Run("oci revision is a baseline", func(t *testing.T) {
		t.Parallel()

		c := labeledImageContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://example.com/org/app.git",
			gitPkg.RefLabel:  "main",
		}, "app:latest", ociImage("aaa111", ""))

		client := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: "aaa111"}})
		got, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.False(t, got.Stale)

		staleClient := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: "bbb222"}})
		stale, err := CheckContainer(t.Context(), staleClient, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.True(t, stale.Stale)
		assert.Equal(t, "bbb222", stale.Commit)
	})

	t.Run("git shortsha image tag is a baseline", func(t *testing.T) {
		t.Parallel()

		c := labeledImageContainer(t, map[string]string{
			gitPkg.RepoLabel: "https://example.com/org/app.git",
			gitPkg.RefLabel:  "main",
		}, "app:git-aaa111bbbbcc", nil)

		client := testClient(t, []RemoteRef{{Name: "refs/heads/main", Hash: "aaa111bbbbccdddd"}})
		got, err := CheckContainer(t.Context(), client, c, types.UpdateParams{})
		require.NoError(t, err)
		assert.False(t, got.Stale)
	})
}

type mutableLister struct {
	refs []RemoteRef
}

func (m *mutableLister) List(context.Context, string) ([]RemoteRef, error) {
	return m.refs, nil
}

// labeledContainer returns a container whose GetLabel values come from labels.
//
// Parameters:
//   - t: Test handle.
//   - labels: Label key to value map.
//
// Returns:
//   - types.Container: Mock container.
func labeledContainer(t *testing.T, labels map[string]string) types.Container {
	t.Helper()

	return labeledImageContainer(t, labels, "app:latest", nil)
}

// labeledImageContainer returns a container with labels, image name, and optional inspect data.
//
// Parameters:
//   - t: Test handle.
//   - labels: Label key to value map.
//   - imageName: ImageName() result.
//   - info: Image inspect response, or nil when image metadata is absent.
//
// Returns:
//   - types.Container: Mock container.
func labeledImageContainer(
	t *testing.T,
	labels map[string]string,
	imageName string,
	info *dockerImage.InspectResponse,
) types.Container {
	t.Helper()

	c := mockTypes.NewMockContainer(t)
	c.EXPECT().GetLabel(mock.Anything).RunAndReturn(func(key string) (string, bool) {
		value, ok := labels[key]

		return value, ok
	}).Maybe()
	c.EXPECT().ImageName().Return(imageName).Maybe()
	c.EXPECT().HasImageInfo().Return(info != nil).Maybe()

	if info != nil {
		c.EXPECT().ImageInfo().Return(info).Maybe()
	}

	return c
}

// ociImage returns image inspect data with optional OCI revision and version labels.
//
// Parameters:
//   - revision: org.opencontainers.image.revision value, or empty.
//   - version: org.opencontainers.image.version value, or empty.
//
// Returns:
//   - *dockerImage.InspectResponse: Inspect payload with those labels.
func ociImage(revision, version string) *dockerImage.InspectResponse {
	cfg := &dockerspec.DockerOCIImageConfig{}

	cfg.Labels = map[string]string{}
	if revision != "" {
		cfg.Labels[oci.RevisionLabel] = revision
	}

	if version != "" {
		cfg.Labels[oci.VersionLabel] = version
	}

	return &dockerImage.InspectResponse{Config: cfg}
}

func TestGitHubAPIWithRewrittenClient(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/app/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))

		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": map[string]any{"sha": "apihash", "type": "commit"},
		})
	})
	mux.HandleFunc("/repos/org/app/tags", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"name": "v1.0.1", "commit": map[string]string{"sha": "tag1"}},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{Token: "tok"})
	client.http = srv.Client()

	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)

	// Hit the test server using getJSON.
	var body struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}

	_, err = client.getJSONPage(t.Context(), srv.URL+"/repos/org/app/git/ref/heads/main", &body, "")
	require.NoError(t, err)
	assert.Equal(t, "apihash", body.Object.SHA)

	tags, ok, err := githubTagsAt(t.Context(), client, srv.URL, "org", "app")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "v1.0.1", tags[0].Name)
	assert.NotEmpty(t, parsed.Host)
}

func githubTagsAt(ctx context.Context, client *Client, base, owner, repo string) ([]RemoteRef, bool, error) {
	endpoint := base + "/repos/" + owner + "/" + repo + "/tags?per_page=100"

	var body []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	_, err := client.getJSONPage(ctx, endpoint, &body, "")
	if err != nil {
		return nil, false, err
	}

	tags := make([]RemoteRef, 0, len(body))
	for _, tag := range body {
		tags = append(tags, RemoteRef{Name: tag.Name, Hash: tag.Commit.SHA})
	}

	return tags, true, nil
}

func TestProviderKindUnknownUsesGoGit(t *testing.T) {
	t.Parallel()

	assert.Empty(t, types.ResolveGitHostKind("git.unknown.example", nil))
	assert.Equal(t, types.GitHostGitea, types.ResolveGitHostKind("git.example.com", map[string]string{
		"git.example.com": types.GitHostForgejo,
	}))
	assert.Equal(t, types.GitHostGitHub, types.ResolveGitHostKind("github.com", nil))
	assert.Equal(t, types.GitHostGitLab, types.ResolveGitHostKind("gitlab.com", nil))
	assert.Equal(t, types.GitHostGitea, types.ResolveGitHostKind("codeberg.org", nil))
	assert.Equal(t, types.GitHostGitHub, types.ResolveGitHostKind("github.company.com", map[string]string{
		"github.company.com": types.GitHostGitHub,
	}))
}

func TestClassifyHostsDetectsProviders(t *testing.T) {
	t.Parallel()

	t.Run("gitea", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.22.0"})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("git.example.com")})
		assert.Equal(t, types.GitHostGitea, got["git.example.com"])
	})

	t.Run("gitlab", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/metadata", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version":    "16.8.1",
				"revision":   "cbe7d4e7",
				"enterprise": false,
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("gitlab.internal")})
		assert.Equal(t, types.GitHostGitLab, got["gitlab.internal"])
	})

	t.Run("github enterprise", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Github-Request-Id", "abc123")
			w.WriteHeader(http.StatusOK)
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("github.company.com")})
		assert.Equal(t, types.GitHostGitHub, got["github.company.com"])
	})

	t.Run("path prefix", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/gitlab/api/v4/metadata", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version":  "16.8.1",
				"revision": "abc",
			})
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		origin := url.URL{Scheme: "https", Host: "git.example.com", Path: "/gitlab"}
		got := client.ClassifyHosts(t.Context(), []url.URL{origin})
		assert.Equal(t, types.GitHostGitLab, got["git.example.com"])
		assert.Equal(t, "https://git.example.com/gitlab/api/v4/projects/group%2Fapp/repository/commits/main",
			joinEscaped(client.gitlabAPIAt(url.URL{}, "git.example.com"), "projects", "group/app", "repository", "commits", "main").String())
	})

	t.Run("unauthorized without fingerprint", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("git.unknown.example")})
		assert.Empty(t, got["git.unknown.example"])
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.NewServeMux())
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = rewriteHostClient(srv)

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("git.unknown.example")})
		assert.Empty(t, got["git.unknown.example"])
	})

	t.Run("skips built-in", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := New(&nop, Options{})

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("github.com")})
		assert.NotContains(t, got, "github.com")
	})
}

func TestGitHubAPIURL(t *testing.T) {
	t.Parallel()

	nop := zerolog.Nop()
	client := New(&nop, Options{})

	assert.Equal(t, "https://api.github.com/repos/org/app", client.githubAPIAt(url.URL{}, "github.com", "repos", "org", "app").String())
	assert.Equal(t, "https://github.company.com/api/v3/repos/org/app", client.githubAPIAt(url.URL{}, "github.company.com", "repos", "org", "app").String())
}

func httpsOrigin(host string) url.URL {
	return url.URL{Scheme: "https", Host: host}
}

func TestSplitRepo(t *testing.T) {
	t.Parallel()

	host, owner, name, ok := splitRepo("https://github.com/org/app.git")
	require.True(t, ok)
	assert.Equal(t, "github.com", host)
	assert.Equal(t, "org", owner)
	assert.Equal(t, "app", name)

	host, owner, name, ok = splitRepo("git@gitlab.com:org/app.git")
	require.True(t, ok)
	assert.Equal(t, "gitlab.com", host)
	assert.Equal(t, "org", owner)
	assert.Equal(t, "app", name)

	host, owner, name, ok = splitRepo("https://gitlab.com/group/sub/repo.git")
	require.True(t, ok)
	assert.Equal(t, "gitlab.com", host)
	assert.Equal(t, "group/sub", owner)
	assert.Equal(t, "repo", name)

	host, owner, name, ok = splitRepo("ssh://git@github.com/org/app.git")
	require.True(t, ok)
	assert.Equal(t, "github.com", host)
	assert.Equal(t, "org", owner)
	assert.Equal(t, "app", name)
}

func TestGitHubRefTriesTagsAfterMissingBranch(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/app/git/ref/heads/v1.2.3", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/repos/org/app/git/ref/tags/v1.2.3", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": map[string]any{"sha": "tagcommit", "type": "commit"},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{})
	client.http = rewriteHostClient(srv)

	resolved, found, err := client.githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "heads/v1.2.3")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, resolved.Hash)

	resolved, found, err = client.githubRef(t.Context(), "github.com", url.URL{}, "org", "app", "tags/v1.2.3")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "tagcommit", resolved.Hash)
}

func TestGitHubTagsFollowsLinkHeader(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/app/tags", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "v1.0.1", "commit": map[string]string{"sha": "newer"}},
			})

			return
		}

		w.Header().Set("Link", `<https://api.github.com/repos/org/app/tags?page=2>; rel="next"`)
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"name": "v1.0.0", "commit": map[string]string{"sha": "older"}},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{})
	client.http = rewriteHostClient(srv)

	tags, ok, err := client.githubTags(t.Context(), "github.com", url.URL{}, "org", "app")
	require.NoError(t, err)
	assert.True(t, ok)
	require.Len(t, tags, 2)
	assert.Equal(t, "v1.0.0", tags[0].Name)
	assert.Equal(t, "v1.0.1", tags[1].Name)
}

func rewriteHostClient(srv *httptest.Server) *http.Client {
	base := srv.Client()

	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = "http"
			cloned.URL.Host = strings.TrimPrefix(srv.URL, "http://")
			cloned.Host = cloned.URL.Host

			return transport.RoundTrip(cloned)
		}),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
