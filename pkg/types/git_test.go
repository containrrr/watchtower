package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGitHostKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, GitHostGitHub, ResolveGitHostKind("github.com", nil))
	assert.Equal(t, GitHostGitLab, ResolveGitHostKind("gitlab.com", nil))
	assert.Equal(t, GitHostGitea, ResolveGitHostKind("codeberg.org", nil))
	assert.Empty(t, ResolveGitHostKind("git.example.com", nil))
	assert.Empty(t, ResolveGitHostKind("bitbucket.org", nil))

	extra := map[string]string{
		"git.example.com":    GitHostGitea,
		"code.local":         GitHostForgejo,
		"gitlab.internal":    GitHostGitLab,
		"github.company.com": GitHostGitHub,
	}
	assert.Equal(t, GitHostGitea, ResolveGitHostKind("git.example.com", extra))
	assert.Equal(t, GitHostGitea, ResolveGitHostKind("code.local", extra))
	assert.Equal(t, GitHostGitLab, ResolveGitHostKind("gitlab.internal", extra))
	assert.Equal(t, GitHostGitHub, ResolveGitHostKind("github.company.com", extra))
	assert.Equal(t, GitHostGitHub, ResolveGitHostKind("github.com", extra))
	assert.Equal(t, GitHostGitea, ResolveGitHostKind("git.example.com:8443", extra))
	assert.Equal(t, GitHostGitHub, ResolveGitHostKind("github.com.", nil))
	assert.Equal(t, GitHostGitea, ResolveGitHostKind("git.example.com.", extra))
}

func TestCanonicalGitHostKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, GitHostGitHub, canonicalGitHostKind("GitHub"))
	assert.Equal(t, GitHostGitLab, canonicalGitHostKind("gitlab"))
	assert.Equal(t, GitHostGitea, canonicalGitHostKind("gitea"))
	assert.Equal(t, GitHostGitea, canonicalGitHostKind("forgejo"))
	assert.Empty(t, canonicalGitHostKind("bitbucket"))
}

func TestValidGitPolicy(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidGitPolicy(GitPolicyNone))
	assert.True(t, ValidGitPolicy(GitPolicyPatch))
	assert.True(t, ValidGitPolicy(GitPolicyMinor))
	assert.True(t, ValidGitPolicy(GitPolicyMajor))
	assert.False(t, ValidGitPolicy(""))
	assert.False(t, ValidGitPolicy("nightly"))
}
