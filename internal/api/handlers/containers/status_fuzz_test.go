package containers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/watchtower/pkg/container"
)

// repoDigestPart returns the digest part of a RepoDigest, after its first @.
//
// Parameters:
//   - repoDigest: RepoDigest in the format "name@digest".
//
// Returns:
//   - string: The part after the first @, empty when there is none.
func repoDigestPart(repoDigest string) string {
	_, digest, _ := strings.Cut(repoDigest, "@")

	return digest
}

// FuzzExtractDigest verifies that ExtractImageDigest never panics and returns
// either nothing or the non-empty digest part of the RepoDigest. Docker reports
// RepoDigests as "name@sha256:...", so the digest is passed on as it is.
func FuzzExtractDigest(f *testing.F) {
	f.Add("nginx@sha256:abc123")
	f.Add("my-registry.com/nginx@sha256:abc123def456")
	f.Add("")
	f.Add("no-digest-here")
	f.Add("@sha256:abc")
	f.Add("nginx@")
	f.Add("nginx@sha256:")
	f.Add("sha256:abc123")
	f.Add("registry.example.com:5000/org/image@sha256:abcdef1234567890")

	f.Fuzz(func(t *testing.T, repoDigest string) {
		digest := container.ExtractImageDigest([]string{repoDigest}, "nginx:latest")

		if repoDigest == "" {
			assert.Empty(t, digest, "empty input should return empty digest")

			return
		}

		assert.Equal(t, repoDigestPart(repoDigest), digest)
	})
}

// FuzzExtractDigestMultiple verifies that ExtractImageDigest selects the digest
// of one of the RepoDigests, or nothing when none has a digest part.
func FuzzExtractDigestMultiple(f *testing.F) {
	f.Add("nginx@sha256:abc123")
	f.Add("sha256:invalid,nginx@sha256:valid")
	f.Add("no-digest,also-no-digest")

	f.Fuzz(func(t *testing.T, digestsStr string) {
		var digests []string
		if digestsStr != "" {
			digests = strings.Split(digestsStr, ",")
		}

		result := container.ExtractImageDigest(digests, "nginx:latest")

		candidates := make([]string, 0, len(digests))
		for _, d := range digests {
			if part := repoDigestPart(d); part != "" {
				candidates = append(candidates, part)
			}
		}

		if len(candidates) == 0 {
			assert.Empty(t, result)

			return
		}

		assert.Contains(t, candidates, result)
	})
}

// FuzzFilterStatuses verifies that filterStatuses never panics and correctly
// filters container statuses by name and image.
func FuzzFilterStatuses(f *testing.F) {
	f.Add("nginx", "nginx:latest")
	f.Add("", "")
	f.Add("nginx", "")
	f.Add("", "nginx:latest")
	f.Add("nonexistent", "nonexistent:latest")

	f.Fuzz(func(t *testing.T, nameFilter, imageFilter string) {
		statuses := []Status{
			{
				Name:    "nginx-proxy",
				Image:   "nginx:latest",
				ImageID: "sha256:abc",
				Digest:  "sha256:def",
			},
			{
				Name:    "redis-cache",
				Image:   "redis:7",
				ImageID: "sha256:123",
				Digest:  "sha256:456",
			},
			{
				Name:    "mysql-db",
				Image:   "mysql:8.0",
				ImageID: "sha256:789",
				Digest:  "sha256:012",
			},
		}

		filtered := filterStatuses(statuses, nameFilter, imageFilter)

		for _, s := range filtered {
			if nameFilter != "" {
				assert.Equal(t, nameFilter, s.Name,
					"filtered status should match name filter")
			}

			if imageFilter != "" {
				assert.Equal(t, imageFilter, s.Image,
					"filtered status should match image filter")
			}
		}

		if nameFilter == "" && imageFilter == "" {
			assert.Len(t, filtered, len(statuses),
				"no filter should return all statuses")
		}
	})
}
