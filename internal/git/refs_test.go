package git

import (
	"errors"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goGit "github.com/go-git/go-git/v5"
)

func TestValidateRef(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateRef(""))
	require.NoError(t, validateRef("main"))
	require.NoError(t, validateRef("v1.2.3"))
	require.NoError(t, validateRef("release/1.2"))
	require.NoError(t, validateRef("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	require.NoError(t, validateRef("refs/heads/main"))
	require.NoError(t, validateRef("refs/tags/v1.2.3"))
	require.ErrorIs(t, validateRef("foo..bar"), errInvalidRef)
	require.ErrorIs(t, validateRef("has space"), errInvalidRef)
}

func TestResolveListedRefPrefersPeeledTag(t *testing.T) {
	t.Parallel()

	resolved, err := resolveListedRef([]RemoteRef{
		{Name: "refs/tags/v1.0.0", Hash: "tagobject"},
		{Name: "refs/tags/v1.0.0^{}", Hash: "commitsha"},
	}, "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "commitsha", resolved.Hash)
	assert.Equal(t, kindTag, resolved.Kind)
}

func TestCollectTagHashesPrefersPeeled(t *testing.T) {
	t.Parallel()

	got := collectTagHashes([]RemoteRef{
		{Name: "refs/heads/main", Hash: "branch"},
		{Name: "refs/tags/v1.2.3", Hash: "tagobject"},
		{Name: "refs/tags/v1.2.3^{}", Hash: "commitsha"},
		{Name: "refs/tags/v1.2.4", Hash: "plain"},
	})
	assert.Equal(t, "commitsha", got["v1.2.3"])
	assert.Equal(t, "plain", got["v1.2.4"])
	assert.NotContains(t, got, "main")
}

func TestCloneReference(t *testing.T) {
	t.Parallel()

	assert.Equal(t, plumbing.NewBranchReferenceName("main"), cloneReference(CheckResult{
		Kind: kindBranch,
		Ref:  "main",
	}))
	assert.Equal(t, plumbing.NewTagReferenceName("v1.2.3"), cloneReference(CheckResult{
		Kind: kindTag,
		Tag:  "v1.2.3",
	}))
	assert.Equal(t, plumbing.NewTagReferenceName("v1.2.3"), cloneReference(CheckResult{
		Kind: kindTag,
		Ref:  "v1.2.3",
	}))
	assert.Equal(t, plumbing.ReferenceName("refs/tags/v1.2.3"), cloneReference(CheckResult{
		Kind: kindTag,
		Tag:  "refs/tags/v1.2.3",
	}))
	assert.Empty(t, cloneReference(CheckResult{Kind: kindTag}).String())
	assert.Empty(t, cloneReference(CheckResult{Kind: kindBranch}).String())
	assert.Empty(t, cloneReference(CheckResult{Commit: "abc"}).String())
}

func TestRevisionTarget(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc", revisionTarget(CheckResult{Commit: "abc", Tag: "v1", Ref: "main"}))
	assert.Equal(t, "v1", revisionTarget(CheckResult{Tag: "v1", Ref: "main"}))
	assert.Equal(t, "main", revisionTarget(CheckResult{Ref: "main"}))
	assert.Empty(t, revisionTarget(CheckResult{}))
}

func TestRefName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, plumbing.ReferenceName("refs/heads/main"), refName("refs/heads/main", kindTag))
	assert.Equal(t, plumbing.NewTagReferenceName("v1.0.0"), refName("v1.0.0", kindTag))
	assert.Equal(t, plumbing.NewBranchReferenceName("main"), refName("main", kindBranch))
}

func TestClassifyTransport(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, classifyTransport(transport.ErrAuthenticationRequired), errAuthRequired)
	require.ErrorIs(t, classifyTransport(transport.ErrAuthorizationFailed), errAuthFailed)
	require.ErrorIs(t, classifyTransport(transport.ErrRepositoryNotFound), errRepoNotFound)
	require.ErrorIs(t, wrapRemoteErr("ls-remote", transport.ErrAuthenticationRequired), errAuthRequired)

	other := errors.New("network down")
	require.ErrorIs(t, classifyTransport(other), other)
}

func TestListOptionsAppendsPeeled(t *testing.T) {
	t.Parallel()

	nop := zerolog.Nop()
	client := New(&nop, Options{InsecureSkipTLS: true, CABundle: []byte("pem")})
	opts := client.listOptions(nil)
	assert.Equal(t, goGit.AppendPeeled, opts.PeelingOption)
	assert.True(t, opts.InsecureSkipTLS)
	assert.Equal(t, []byte("pem"), opts.CABundle)
}
