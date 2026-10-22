package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// FuzzSelectTag verifies tag-policy selection never panics and only returns
// a tag from the advertised list.
func FuzzSelectTag(f *testing.F) {
	f.Add("v1.2.3", "v1.2.3\nv1.2.4\nv1.3.0", types.GitPolicyPatch)
	f.Add("v1.2.3", "v1.2.4\nv2.0.0", types.GitPolicyMinor)
	f.Add("", "v1.0.0\nv2.0.0", types.GitPolicyMajor)
	f.Add("nightly", "v1.0.0", types.GitPolicyMajor)
	f.Add("v1.0.0", "latest\nnightly", types.GitPolicyPatch)
	f.Add("v1.2.3", "v1.2.4", "")
	f.Add("v1.2.3", "v1.2.4", types.GitPolicyNone)
	f.Add("1.2.3", "1.2.4", types.GitPolicyPatch)

	f.Fuzz(func(t *testing.T, lastTag, rawTags, policy string) {
		tags := strings.Split(rawTags, "\n")

		got, ok := selectTag(lastTag, tags, policy)
		if !ok {
			assert.Empty(t, got)

			return
		}

		assert.Contains(t, tags, got)
	})
}
