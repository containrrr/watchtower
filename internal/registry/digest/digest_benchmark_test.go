package digest

import (
	"strings"
	"testing"
)

const (
	// testDigest is a sample SHA256 digest used in benchmarks.
	testDigest = "d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547"
)

// BenchmarkDigestsMatch benchmarks the digestsMatch function which compares local digests
// with a remote digest to check for matches.
// This measures the CPU overhead of digest comparison without network operations.
func BenchmarkDigestsMatch(b *testing.B) {
	// Pre-defined test data representing typical Docker image digests
	localDigests := []string{
		"ghcr.io/example/app@sha256:" + testDigest,
		"docker.io/library/nginx@sha256:a5967740c5c9a1c4c6f5a5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c5c",
	}
	remoteDigest := testDigest
	log := newNopLogger()

	for b.Loop() {
		_ = digestsMatch(log, localDigests, remoteDigest)
	}
}

// BenchmarkDigestsMatchNoMatch benchmarks digestsMatch when there are no matches.
// This represents the worst case where the loop must iterate through all digests.
func BenchmarkDigestsMatchNoMatch(b *testing.B) {
	localDigests := []string{
		"ghcr.io/example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"docker.io/library/nginx@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"docker.io/library/postgres@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"quay.io/redhat/ubi8@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		"gcr.io/distroless/static@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	}
	remoteDigest := testDigest
	log := newNopLogger()

	for b.Loop() {
		_ = digestsMatch(log, localDigests, remoteDigest)
	}
}

// BenchmarkDigestsMatchManyDigests benchmarks digestsMatch with a large number of local digests.
// This simulates containers with multiple image tags or mirrored registries.
func BenchmarkDigestsMatchManyDigests(b *testing.B) {
	// Generate 20 local digests with match at the end to simulate worst-case iteration
	localDigests := make([]string, 20)
	for i := range 19 {
		localDigests[i] = "ghcr.io/mirror/registry-" + string(rune('a'+i)) +
			"/app@sha256:" + strings.Repeat(string(rune('a'+i)), 64)
	}

	localDigests[19] = "ghcr.io/mirror/registry-t/app@sha256:" + testDigest

	remoteDigest := testDigest
	log := newNopLogger()

	for b.Loop() {
		_ = digestsMatch(log, localDigests, remoteDigest)
	}
}

// BenchmarkNormalizeDigest benchmarks the NormalizeDigest function which strips
// common prefixes like "sha256:" from digest strings.
func BenchmarkNormalizeDigest(b *testing.B) {
	testDigests := []string{
		"sha256:" + testDigest,
		"sha256:abc123",
		testDigest,
		"sha256:" + strings.Repeat("a", 64),
	}
	log := newNopLogger()

	for b.Loop() {
		for _, d := range testDigests {
			_ = NormalizeDigest(log, d)
		}
	}
}

// BenchmarkDigestsMatchEarlyMatch benchmarks the digestsMatch function when the first digest matches.
// This is the best case scenario where the loop exits early.
func BenchmarkDigestsMatchEarlyMatch(b *testing.B) {
	localDigests := []string{
		"ghcr.io/example/app@sha256:" + testDigest,
		"docker.io/library/nginx@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"docker.io/library/postgres@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	remoteDigest := testDigest
	log := newNopLogger()

	for b.Loop() {
		_ = digestsMatch(log, localDigests, remoteDigest)
	}
}

// BenchmarkDigestsMatchLateMatch benchmarks digestsMatch when the last digest matches.
// This is the worst case for early exit optimization.
func BenchmarkDigestsMatchLateMatch(b *testing.B) {
	localDigests := []string{
		"ghcr.io/example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"docker.io/library/nginx@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"docker.io/library/postgres@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"ghcr.io/example/app@sha256:" + testDigest, // Matches last
	}
	remoteDigest := testDigest
	log := newNopLogger()

	for b.Loop() {
		_ = digestsMatch(log, localDigests, remoteDigest)
	}
}
