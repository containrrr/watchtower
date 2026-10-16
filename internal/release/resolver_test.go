package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const testDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

// spyFetcher records the tags it was asked for and replays canned results.
//
// It is mutex guarded so it can be shared by the concurrent lookup tests, where
// several goroutines may reach it before any cache entry exists.
type spyFetcher struct {
	// mu guards calls.
	mu sync.Mutex
	// digests maps a tag to the digest returned for it. A missing tag yields
	// empty with no error, which is how a registry reports an unknown tag.
	digests map[string]string
	// errs maps a tag to the error returned for it.
	errs map[string]error
	// calls records every tag requested, in order.
	calls []string
}

func (f *spyFetcher) FetchDigestForTag(
	_ *zerolog.Logger,
	_ context.Context,
	_ types.Container,
	_ string,
	tag string,
	_ ...string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, tag)

	if err, ok := f.errs[tag]; ok {
		return "", err
	}

	return f.digests[tag], nil
}

// callCount returns how many candidates the fetcher has served.
func (f *spyFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.calls)
}

func testLog() *zerolog.Logger {
	n := zerolog.Nop()

	return &n
}

// transientThenOKFetcher fails its first sweep, then behaves like a registry
// that published the prefixed tag.
type transientThenOKFetcher struct {
	err     error
	sweptUp bool
	calls   int
}

func (f *transientThenOKFetcher) FetchDigestForTag(
	_ *zerolog.Logger,
	_ context.Context,
	_ types.Container,
	_ string,
	tag string,
	_ ...string,
) (string, error) {
	f.calls++

	if !f.sweptUp {
		f.sweptUp = true

		return "", f.err
	}

	if tag == "v1.2.3" {
		return testDigest, nil
	}

	return "", nil
}

func TestTagCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    []string
	}{
		{name: "unprefixed prefers the v form", version: "1.2.3", want: []string{"v1.2.3", "1.2.3"}},
		{name: "prefixed prefers itself", version: "v1.2.3", want: []string{"v1.2.3", "1.2.3"}},
		{name: "prerelease", version: "1.2.3-rc.1", want: []string{"v1.2.3-rc.1", "1.2.3-rc.1"}},
		{name: "calendar version", version: "2026.09.1", want: []string{"v2026.09.1", "2026.09.1"}},
		{name: "non semver is still probed", version: "nightly", want: []string{"vnightly", "nightly"}},
		{name: "whitespace is trimmed", version: "  1.2.3  ", want: []string{"v1.2.3", "1.2.3"}},
		{name: "empty", version: ""},
		{name: "whitespace only", version: "   "},
		{name: "lone v yields one candidate", version: "v", want: []string{"v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, TagCandidates(tt.version))
		})
	}
}

// The v-prefixed spelling must win when both match, since that is the common case.
func TestResolveTagPrefersFirstCandidate(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{
		"v1.2.3": testDigest,
		"1.2.3":  testDigest,
	}}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Equal(t, "v1.2.3", got)
	assert.Equal(t, []string{"v1.2.3"}, fetcher.calls, "the first match must end the probe")
}

// A maintainer who tags without a "v" costs one extra request, and only that.
func TestResolveTagFallsBackToSecondCandidate(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{"1.2.3": testDigest}}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Equal(t, "1.2.3", got)
	assert.Equal(t, []string{"v1.2.3", "1.2.3"}, fetcher.calls)
}

func TestResolveTagNoMatch(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{
		"v1.2.3": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"1.2.3":  "sha256:3333333333333333333333333333333333333333333333333333333333333333",
	}}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Empty(t, got)
	assert.Equal(t, []string{"v1.2.3", "1.2.3"}, fetcher.calls)
}

// A registry that publishes no versioned tags is the latest-only mirror case.
func TestResolveTagEmptyRegistry(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{}}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Empty(t, got)
	assert.Equal(t, []string{"v1.2.3", "1.2.3"}, fetcher.calls)
}

// A clean sweep that found nothing is a final answer, so it is cached.
func TestResolveTagCachesDefinitiveMiss(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	assert.Empty(t, r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
	assert.Empty(t, r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
	assert.Len(t, fetcher.calls, 2, "only the first attempt should reach the registry")
}

// A transient failure must not deny every sibling container its own attempt.
func TestResolveTagDoesNotCacheTransientFailure(t *testing.T) {
	t.Parallel()

	transient := errors.New("connection reset")
	fetcher := &transientThenOKFetcher{err: transient}

	r := NewResolver(fetcher)
	ctx := context.Background()

	// The first sweep is inconclusive, so it returns empty and caches nothing.
	assert.Empty(t, r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))

	// A sibling container must still get a real attempt, which now succeeds.
	assert.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
}

// A rate limit is a statement about the whole host, so it is remembered briefly
// rather than costing every sibling container its own doomed request.
func TestResolveTagRateLimitIsNegativelyCachedForTheTTL(t *testing.T) {
	t.Parallel()

	fetcher := &transientThenOKFetcher{err: fmt.Errorf("429: %w", ratelimit.ErrRateLimited)}

	r := NewResolver(fetcher)
	now := time.Now()
	r.now = func() time.Time { return now }

	ctx := context.Background()

	assert.Empty(t, r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))

	// Siblings inside the TTL must not re-probe a host that just refused.
	for range 3 {
		assert.Empty(t, r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
	}

	assert.Equal(t, 1, fetcher.calls, "the refusal must be cached, not retried per sibling")

	// Once the TTL lapses the next container gets a real attempt, which succeeds.
	now = now.Add(rateLimitCacheTTL + time.Second)

	assert.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
}

// A rate limit must abandon the probe rather than spend a second refused request.
func TestResolveTagAbandonsSecondCandidateAfterRateLimit(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{
		digests: map[string]string{"1.2.3": testDigest},
		errs:    map[string]error{"v1.2.3": fmt.Errorf("429: %w", ratelimit.ErrRateLimited)},
	}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Empty(t, got)
	assert.Equal(t, []string{"v1.2.3"}, fetcher.calls)
}

// A non rate-limit failure on the first candidate still allows the second.
func TestResolveTagContinuesAfterOrdinaryError(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{
		digests: map[string]string{"1.2.3": testDigest},
		errs:    map[string]error{"v1.2.3": errors.New("connection reset")},
	}

	r := NewResolver(fetcher)
	got := r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest)

	assert.Equal(t, "1.2.3", got)
	assert.Equal(t, []string{"v1.2.3", "1.2.3"}, fetcher.calls)
}

func TestResolveTagSkipsWhenNotWorthProbing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container types.Container
		version   string
		digest    string
	}{
		{name: "no version", container: testContainer(), digest: testDigest},
		{name: "no new digest", container: testContainer(), version: "1.2.3"},
		{name: "nil container", version: "1.2.3", digest: testDigest},
		{name: "digest pinned", container: pinnedContainer(), version: "1.2.3", digest: testDigest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetcher := &spyFetcher{digests: map[string]string{"v1.2.3": testDigest}}

			r := NewResolver(fetcher)
			got := r.ResolveTag(testLog(), context.Background(), tt.container, "", tt.version, tt.digest)

			assert.Empty(t, got)
			assert.Empty(t, fetcher.calls, "no candidate may be requested")
		})
	}
}

// Sibling containers on one image must not each pay for the lookup.
func TestResolveTagCachesAcrossContainers(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{"v1.2.3": testDigest}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	first := r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest)
	second := r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest)

	assert.Equal(t, "v1.2.3", first)
	assert.Equal(t, "v1.2.3", second)
	assert.Equal(t, []string{"v1.2.3"}, fetcher.calls)
}

// A different pulled image invalidates the previous answer.
func TestResolveTagCacheKeysOnDigest(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{
		"v1.2.3": testDigest,
		"v1.3.0": "sha256:4444444444444444444444444444444444444444444444444444444444444444",
	}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	assert.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
	assert.Equal(
		t,
		"v1.3.0",
		r.ResolveTag(
			testLog(),
			ctx,
			testContainer(),
			"",
			"1.3.0",
			"sha256:4444444444444444444444444444444444444444444444444444444444444444",
		),
	)
}

func TestResolveTagClearCache(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{"v1.2.3": testDigest}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	require.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))

	r.ClearCache()

	require.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
	assert.Len(t, fetcher.calls, 2)
}

// TestResolveTagNilFetcher verifies that a Resolver built without a fetcher
// makes no lookups and returns empty instead of panicking.
func TestResolveTagNilFetcher(t *testing.T) {
	t.Parallel()

	r := NewResolver(nil)

	assert.NotPanics(t, func() {
		assert.Empty(t, r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest))
	})
}

func TestResolveTagNilResolver(t *testing.T) {
	t.Parallel()

	var r *Resolver

	assert.Empty(t, r.ResolveTag(testLog(), context.Background(), testContainer(), "", "1.2.3", testDigest))
}

// One resolver is shared by every parallel staleness worker, so the cache must
// be safe under concurrent lookups. Run with -race to make this meaningful.
func TestResolveTagIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	fetcher := &spyFetcher{digests: map[string]string{"v1.2.3": testDigest}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	var wg sync.WaitGroup

	for range 32 {
		wg.Go(func() {
			assert.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
		})
	}

	wg.Wait()

	// The cache must collapse the fan-out. The exact count is not deterministic
	// because several goroutines may reach the fetcher before the first entry is
	// stored, but it can never be one request per goroutine.
	assert.Less(t, fetcher.callCount(), 32, "the cache must serve most callers")
}

// Concurrent lookups spanning different digests must not cross-contaminate the
// per-digest cache entries.
func TestResolveTagConcurrentDistinctDigests(t *testing.T) {
	t.Parallel()

	second := "sha256:2222222222222222222222222222222222222222222222222222222222222222"

	fetcher := &spyFetcher{digests: map[string]string{
		"v1.2.3": testDigest,
		"v2.0.0": second,
	}}

	r := NewResolver(fetcher)
	ctx := context.Background()

	var wg sync.WaitGroup

	for range 16 {
		wg.Go(func() {
			assert.Equal(t, "v1.2.3", r.ResolveTag(testLog(), ctx, testContainer(), "", "1.2.3", testDigest))
			assert.Equal(t, "v2.0.0", r.ResolveTag(testLog(), ctx, testContainer(), "", "2.0.0", second))
		})
	}

	wg.Wait()
}

// TestDigestFetcherUsesItsSettings verifies that a DigestFetcher makes its
// registry requests with the settings it was built with, so a registry served
// over plain HTTP is reachable only when the settings skip TLS.
func TestDigestFetcherUsesItsSettings(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodHead && r.URL.Path == "/v2/org/app/manifests/v1.2.3":
			w.Header().Set("Docker-Content-Digest", testDigest)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	container := stubContainer{imageName: strings.TrimPrefix(server.URL, "http://") + "/org/app:1.2.3"}

	got, err := NewDigestFetcher(registryConfig.Registry{TLSSkip: true}).
		FetchDigestForTag(testLog(), t.Context(), container, "", "v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimPrefix(testDigest, "sha256:"), got)

	_, err = NewDigestFetcher(registryConfig.Registry{}).
		FetchDigestForTag(testLog(), t.Context(), container, "", "v1.2.3")
	require.Error(t, err, "an HTTPS request reached the plain HTTP registry")
}
