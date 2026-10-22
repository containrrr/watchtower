package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestSelectTag(t *testing.T) {
	t.Parallel()

	tags := []string{"v1.2.3", "v1.2.4", "v1.3.0", "v2.0.0", "latest"}

	tests := []struct {
		name    string
		lastTag string
		tags    []string
		policy  string
		want    string
		ok      bool
	}{
		{name: "empty policy", lastTag: "v1.2.3", tags: tags, policy: ""},
		{name: "none policy", lastTag: "v1.2.3", tags: tags, policy: types.GitPolicyNone},
		{name: "patch", lastTag: "v1.2.3", tags: tags, policy: types.GitPolicyPatch, want: "v1.2.4", ok: true},
		{name: "minor", lastTag: "v1.2.3", tags: tags, policy: types.GitPolicyMinor, want: "v1.3.0", ok: true},
		{name: "major", lastTag: "v1.2.3", tags: tags, policy: types.GitPolicyMajor, want: "v2.0.0", ok: true},
		{name: "already latest major", lastTag: "v2.0.0", tags: tags, policy: types.GitPolicyMajor},
		{name: "only non-semver", lastTag: "v1.0.0", tags: []string{"nightly", "foo"}, policy: types.GitPolicyPatch},
		{name: "no last tag picks highest", lastTag: "", tags: tags, policy: types.GitPolicyMajor, want: "v2.0.0", ok: true},
		{name: "invalid last tag treated as empty", lastTag: "nightly", tags: tags, policy: types.GitPolicyMajor, want: "v2.0.0", ok: true},
		{name: "unprefixed last tag", lastTag: "1.2.3", tags: tags, policy: types.GitPolicyPatch, want: "v1.2.4", ok: true},
		{name: "unprefixed remote tags", lastTag: "v1.2.3", tags: []string{"1.2.4", "1.3.0"}, policy: types.GitPolicyPatch, want: "1.2.4", ok: true},
		{name: "whitespace last tag", lastTag: "  ", tags: []string{"v1.0.1"}, policy: types.GitPolicyMajor, want: "v1.0.1", ok: true},
		{name: "older candidates ignored", lastTag: "v1.2.4", tags: []string{"v1.2.3", "v1.2.4"}, policy: types.GitPolicyPatch},
		{name: "patch skips pre-release", lastTag: "v1.2.3", tags: []string{"v1.2.4-rc.1", "v1.2.4"}, policy: types.GitPolicyPatch, want: "v1.2.4", ok: true},
		{name: "patch ignores only pre-release", lastTag: "v1.2.3", tags: []string{"v1.2.4-rc.1"}, policy: types.GitPolicyPatch},
		{name: "minor skips pre-release", lastTag: "v1.2.3", tags: []string{"v1.3.0-rc.1", "v1.3.0"}, policy: types.GitPolicyMinor, want: "v1.3.0", ok: true},
		{name: "pre-release baseline moves to release", lastTag: "v1.2.3-rc.1", tags: []string{"v1.2.3", "v1.2.4-rc.1"}, policy: types.GitPolicyPatch, want: "v1.2.3", ok: true},
		{name: "build metadata does not hide a release", lastTag: "v1.2.3", tags: []string{"v1.2.4+sha.1"}, policy: types.GitPolicyPatch, want: "v1.2.4+sha.1", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := selectTag(tt.lastTag, tt.tags, tt.policy)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAllowedBump(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   string
		candidate string
		policy    string
		want      bool
	}{
		{name: "not newer", current: "v1.2.3", candidate: "v1.2.3", policy: types.GitPolicyMajor},
		{name: "older", current: "v1.2.3", candidate: "v1.2.2", policy: types.GitPolicyMajor},
		{name: "patch same minor", current: "v1.2.3", candidate: "v1.2.4", policy: types.GitPolicyPatch, want: true},
		{name: "patch different minor", current: "v1.2.3", candidate: "v1.3.0", policy: types.GitPolicyPatch},
		{name: "minor same major", current: "v1.2.3", candidate: "v1.4.0", policy: types.GitPolicyMinor, want: true},
		{name: "minor different major", current: "v1.2.3", candidate: "v2.0.0", policy: types.GitPolicyMinor},
		{name: "major any newer", current: "v1.2.3", candidate: "v2.0.0", policy: types.GitPolicyMajor, want: true},
		{name: "unknown policy", current: "v1.2.3", candidate: "v1.2.4", policy: "nightly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, allowedBump(tt.current, tt.candidate, tt.policy))
		})
	}
}

func TestCanonicalize(t *testing.T) {
	t.Parallel()

	assert.Empty(t, canonicalize(""))
	assert.Empty(t, canonicalize("  "))
	assert.Equal(t, "v1.2.3", canonicalize("v1.2.3"))
	assert.Equal(t, "v1.2.3", canonicalize("1.2.3"))
	assert.Equal(t, "v1.2.3", canonicalize("  1.2.3  "))
	assert.Equal(t, "nightly", canonicalize("nightly"))
	assert.Equal(t, "v1.2.3-rc.1", canonicalize("1.2.3-rc.1"))
}

func TestSelectTagKeepsRemoteName(t *testing.T) {
	t.Parallel()

	got, ok := selectTag("v1.0.0", []string{"1.0.1"}, types.GitPolicyPatch)
	require.True(t, ok)
	assert.Equal(t, "1.0.1", got)
}
