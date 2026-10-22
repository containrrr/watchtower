package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goGit "github.com/go-git/go-git/v5"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// localRepo is a filesystem Git repository used as a clone source.
type localRepo struct {
	dir    string
	branch string
	commit string
}

func TestCloneCheckout(t *testing.T) {
	t.Parallel()

	t.Run("checks out branch commit", func(t *testing.T) {
		t.Parallel()

		src := initLocalRepo(t)
		dest := filepath.Join(t.TempDir(), "clone")

		err := cloneCheckout(t.Context(), dest, src.dir, CheckResult{
			Commit: src.commit,
			Kind:   kindBranch,
			Ref:    src.branch,
		}, &Client{})
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dest, "README"))
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(got))

		cloned, err := goGit.PlainOpen(dest)
		require.NoError(t, err)
		head, err := cloned.Head()
		require.NoError(t, err)
		assert.Equal(t, src.commit, head.Hash().String())
	})

	t.Run("checks out tag", func(t *testing.T) {
		t.Parallel()

		src := initLocalRepo(t)
		createLightweightTag(t, src.dir, "v1.0.0", src.commit)
		dest := filepath.Join(t.TempDir(), "clone")

		err := cloneCheckout(t.Context(), dest, src.dir, CheckResult{
			Commit: src.commit,
			Kind:   kindTag,
			Tag:    "v1.0.0",
			Ref:    "v1.0.0",
		}, &Client{})
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dest, "README"))
		require.NoError(t, err)
		assert.Equal(t, "hello\n", string(got))
	})

	t.Run("auth failure wraps errCloneFailed", func(t *testing.T) {
		t.Parallel()

		err := cloneCheckout(t.Context(), filepath.Join(t.TempDir(), "clone"), "git@github.com:org/app.git", CheckResult{
			Commit: "aaa111",
			Kind:   kindBranch,
			Ref:    "main",
		}, &Client{opts: Options{SSHKeyPath: filepath.Join(t.TempDir(), "missing")}})
		require.ErrorIs(t, err, errCloneFailed)
		assert.ErrorContains(t, err, "load ssh key:")
	})

	t.Run("missing repo wraps errCloneFailed", func(t *testing.T) {
		t.Parallel()

		err := cloneCheckout(t.Context(), filepath.Join(t.TempDir(), "clone"), filepath.Join(t.TempDir(), "missing.git"), CheckResult{
			Commit: "aaa111",
			Kind:   kindBranch,
			Ref:    "main",
		}, &Client{})
		require.ErrorIs(t, err, errCloneFailed)
	})

	t.Run("unknown revision wraps errCloneFailed", func(t *testing.T) {
		t.Parallel()

		src := initLocalRepo(t)
		err := cloneCheckout(t.Context(), filepath.Join(t.TempDir(), "clone"), src.dir, CheckResult{
			Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			Kind:   kindBranch,
			Ref:    src.branch,
		}, &Client{})
		require.ErrorIs(t, err, errCloneFailed)
		assert.ErrorContains(t, err, "resolve")
	})

	t.Run("repository error is sanitized", func(t *testing.T) {
		t.Parallel()

		repo := "https://user:clone-secret@%zz/repo.git?token=clone-query#clone-fragment"
		err := cloneCheckout(t.Context(), filepath.Join(t.TempDir(), "clone"), repo, CheckResult{
			Commit: "aaa111",
			Kind:   kindBranch,
			Ref:    "main",
		}, &Client{})
		require.ErrorIs(t, err, errCloneFailed)
		assert.NotContains(t, err.Error(), "clone-secret")
		assert.NotContains(t, err.Error(), "clone-query")
		assert.NotContains(t, err.Error(), "clone-fragment")
	})
}

func TestCloneOptions(t *testing.T) {
	t.Parallel()

	auth := &gitHTTP.BasicAuth{Username: tokenUserGit, Password: "tok"}
	bundle := []byte("pem")
	repo := "https://github.com/org/app.git"

	tests := []struct {
		name    string
		opts    Options
		rev     CheckResult
		auth    *gitHTTP.BasicAuth
		wantRef plumbing.ReferenceName
	}{
		{
			name:    "branch sets reference",
			rev:     CheckResult{Commit: "aaa111", Kind: kindBranch, Ref: "main"},
			auth:    auth,
			wantRef: plumbing.NewBranchReferenceName("main"),
		},
		{
			name:    "tag uses tag name",
			rev:     CheckResult{Kind: kindTag, Tag: "v1.2.3"},
			wantRef: plumbing.NewTagReferenceName("v1.2.3"),
		},
		{
			name:    "tag falls back to ref",
			rev:     CheckResult{Kind: kindTag, Ref: "v2.0.0"},
			wantRef: plumbing.NewTagReferenceName("v2.0.0"),
		},
		{
			name: "sha only omits reference",
			rev:  CheckResult{Commit: "aaa111"},
		},
		{
			name: "branch without ref omits reference",
			rev:  CheckResult{Kind: kindBranch, Commit: "aaa111"},
		},
		{
			name: "tls options",
			opts: Options{InsecureSkipTLS: true, CABundle: bundle},
			rev:  CheckResult{Commit: "aaa111"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := (&Client{opts: tt.opts}).cloneOptions(repo, tt.rev, tt.auth)
			assert.Equal(t, repo, got.URL)
			assert.Equal(t, 1, got.Depth)
			assert.True(t, got.SingleBranch)
			assert.Equal(t, goGit.NoTags, got.Tags)
			assert.True(t, got.NoCheckout)
			assert.Equal(t, tt.wantRef, got.ReferenceName)
			assert.Equal(t, tt.opts.InsecureSkipTLS, got.InsecureSkipTLS)
			assert.Equal(t, tt.opts.CABundle, got.CABundle)
			assert.Equal(t, tt.auth, got.Auth)
		})
	}
}

// initLocalRepo creates a non-bare repo with one committed file.
//
// Parameters:
//   - t: Test handle.
//
// Returns:
//   - localRepo: Directory, default branch, and commit hash.
func initLocalRepo(t *testing.T) localRepo {
	t.Helper()

	dir := t.TempDir()
	repo, err := goGit.PlainInit(dir, false)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("hello\n"), 0o600))

	_, err = worktree.Add("README")
	require.NoError(t, err)

	hash, err := worktree.Commit("init", &goGit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	return localRepo{
		dir:    dir,
		branch: head.Name().Short(),
		commit: hash.String(),
	}
}

// createLightweightTag points name at commit in repoDir.
//
// Parameters:
//   - t: Test handle.
//   - repoDir: Existing Git repository.
//   - name: Tag name.
//   - commit: Commit SHA.
func createLightweightTag(t *testing.T, repoDir, name, commit string) {
	t.Helper()

	repo, err := goGit.PlainOpen(repoDir)
	require.NoError(t, err)

	_, err = repo.CreateTag(name, plumbing.NewHash(commit), nil)
	require.NoError(t, err)
}
