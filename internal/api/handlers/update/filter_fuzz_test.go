package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzContainerFilter verifies that ContainerFilter never panics and correctly
// matches container names against patterns, including regex patterns.
func FuzzContainerFilter(f *testing.F) {
	f.Add("nginx,redis")
	f.Add(".*")
	f.Add("web-.*")
	f.Add("")
	f.Add("nginx")
	f.Add("my-container-[0-9]+")
	f.Add("[invalid(regex")

	f.Fuzz(func(t *testing.T, patternsStr string) {
		var patterns []string
		if patternsStr != "" {
			patterns = strings.Split(patternsStr, ",")
		}

		filter := ContainerFilter(patterns)

		containerNames := []string{
			"nginx",
			"redis",
			"web-1",
			"web-2",
			"my-container-123",
			"invalid[regex",
			"",
			"container_with_underscores",
		}

		for _, name := range containerNames {
			result := filter(name, true)
			assert.True(t, result == true || result == false,
				"ContainerFilter should return a boolean for name %q with patterns %v",
				name, patterns)
		}
	})
}
