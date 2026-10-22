package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzFilterImages verifies that filterImages never panics and correctly
// filters image statuses by name and image ID.
func FuzzFilterImages(f *testing.F) {
	f.Add("nginx:latest", "sha256:abc")
	f.Add("", "")
	f.Add("nginx:latest", "")
	f.Add("", "sha256:abc")
	f.Add("nonexistent", "sha256:999")

	f.Fuzz(func(t *testing.T, nameFilter, idFilter string) {
		statuses := []ImageStatus{
			{
				Name:       "nginx:latest",
				ImageID:    "sha256:abc",
				Digest:     "sha256:def",
				Containers: 2,
			},
			{
				Name:       "redis:7",
				ImageID:    "sha256:123",
				Digest:     "sha256:456",
				Containers: 1,
			},
			{
				Name:       "mysql:8.0",
				ImageID:    "sha256:789",
				Digest:     "sha256:012",
				Containers: 3,
			},
		}

		filtered := filterImages(statuses, nameFilter, idFilter)

		for _, s := range filtered {
			if nameFilter != "" {
				assert.Equal(t, nameFilter, s.Name,
					"filtered image should match name filter")
			}

			if idFilter != "" {
				assert.Equal(t, idFilter, s.ImageID,
					"filtered image should match ID filter")
			}
		}

		if nameFilter == "" && idFilter == "" {
			assert.Len(t, filtered, len(statuses),
				"no filter should return all statuses")
		}
	})
}
