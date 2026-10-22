package git

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goGit "github.com/go-git/go-git/v5"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestGitListerList(t *testing.T) {
	t.Parallel()

	t.Run("lists refs from local repo", func(t *testing.T) {
		t.Parallel()

		src := initLocalRepo(t)
		createLightweightTag(t, src.dir, "v1.0.0", src.commit)

		refs, err := gitLister{client: &Client{}}.List(t.Context(), src.dir)
		require.NoError(t, err)
		require.NotEmpty(t, refs)

		var sawCommit, sawTag bool

		for _, ref := range refs {
			if ref.Hash == src.commit {
				sawCommit = true
			}

			if ref.Name == "refs/tags/v1.0.0" {
				sawTag = true

				assert.Equal(t, src.commit, ref.Hash)
			}
		}

		assert.True(t, sawCommit)
		assert.True(t, sawTag)
	})

	t.Run("auth failure", func(t *testing.T) {
		t.Parallel()

		_, err := gitLister{client: &Client{opts: Options{
			SSHKeyPath: filepath.Join(t.TempDir(), "missing"),
		}}}.List(t.Context(), "git@github.com:org/app.git")
		require.Error(t, err)
		assert.ErrorContains(t, err, "load ssh key:")
	})

	t.Run("missing repo wraps ls-remote", func(t *testing.T) {
		t.Parallel()

		_, err := gitLister{client: &Client{}}.List(t.Context(), filepath.Join(t.TempDir(), "missing.git"))
		require.Error(t, err)
		assert.ErrorContains(t, err, "ls-remote:")
	})
}

func TestListOptions(t *testing.T) {
	t.Parallel()

	auth := &gitHTTP.BasicAuth{Username: tokenUserGit, Password: "tok"}
	client := &Client{opts: Options{InsecureSkipTLS: true, CABundle: []byte("pem")}}

	got := client.listOptions(auth)
	assert.Equal(t, auth, got.Auth)
	assert.Equal(t, goGit.AppendPeeled, got.PeelingOption)
	assert.True(t, got.InsecureSkipTLS)
	assert.Equal(t, []byte("pem"), got.CABundle)
}

func TestWrapRemoteErr(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, wrapRemoteErr("ls-remote", nil))
	})

	t.Run("classifies transport sentinels", func(t *testing.T) {
		t.Parallel()

		err := wrapRemoteErr("ls-remote", transport.ErrAuthenticationRequired)
		require.ErrorIs(t, err, errAuthRequired)
		assert.ErrorContains(t, err, "ls-remote:")
	})

	t.Run("preserves unknown errors", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("network down")
		err := wrapRemoteErr("clone", cause)
		require.ErrorIs(t, err, cause)
		assert.ErrorContains(t, err, "clone:")
	})
}

func TestClassifyTransportMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "auth required", err: transport.ErrAuthenticationRequired, want: errAuthRequired},
		{name: "auth failed", err: transport.ErrAuthorizationFailed, want: errAuthFailed},
		{name: "repo not found", err: transport.ErrRepositoryNotFound, want: errRepoNotFound},
		{name: "other", err: errors.New("network down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyTransport(tt.err)
			if tt.want != nil {
				require.ErrorIs(t, got, tt.want)

				return
			}

			require.ErrorIs(t, got, tt.err)
		})
	}
}
