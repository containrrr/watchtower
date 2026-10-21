package release

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/digest"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// rateLimitCacheTTL is how long a rate-limited lookup is remembered.
//
// A rate limit is a statement about the whole host, not a per-request hiccup, so
// every sibling container on that host would be refused too. Caching it briefly
// stops one throttled registry from absorbing one doomed request per container.
// It is short so a later session still gets a real attempt.
const rateLimitCacheTTL = time.Minute

// Candidate sweep outcomes.
const (
	// outcomeFailed means a candidate failed for a reason unrelated to the
	// registry's willingness to answer, so the result is inconclusive.
	outcomeFailed probeOutcome = iota
	// outcomeRateLimited means the registry refused the request outright.
	outcomeRateLimited
	// outcomeNoMatch means every candidate answered and none matched.
	outcomeNoMatch
	// outcomeMatched means a candidate answered and its digest matched.
	outcomeMatched
)

// errFetchTagDigest indicates a candidate release tag lookup failed.
var errFetchTagDigest = errors.New("failed to fetch candidate tag digest")

// Fetcher retrieves the manifest digest of a specific tag.
//
// It is an interface so callers can substitute a spy and assert that a default
// session never reaches the registry.
type Fetcher interface {
	// FetchDigestForTag returns the normalized digest of tag, or empty when the
	// tag does not exist.
	FetchDigestForTag(
		log *zerolog.Logger,
		ctx context.Context,
		container types.Container,
		authToken string,
		tag string,
		endpoints ...string,
	) (string, error)
}

// Resolver confirms the release tag spelling for a pulled image.
//
// Results are cached per process, keyed by registry, repository, and the new
// image digest, because several containers commonly share one upstream image.
// A definitive miss is cached too, so a mirror that publishes no versioned tags
// is not asked twice within a session.
//
// The zero value is not usable.
// Construct one with NewResolver.
type Resolver struct {
	fetcher Fetcher

	// cache maps a cache key to a cached answer. A zero expiry never lapses.
	cache sync.Map

	// now reports the current time, so cache expiry is testable.
	now func() time.Time
}

// cachedAnswer is one cached release tag result.
type cachedAnswer struct {
	// tag is the confirmed spelling, or empty for a definitive miss.
	tag string
	// expires is when a short-lived answer lapses.
	// A zero value means the answer is final for the process lifetime.
	expires time.Time
}

// DigestFetcher fetches tag digests through the digest package and satisfies
// Fetcher.
//
// Construct one with NewDigestFetcher.
type DigestFetcher struct {
	// settings holds the settings for registry requests.
	settings registryConfig.Registry
	// rateLimiter paces the requests with the rest of the process's registry traffic.
	rateLimiter *ratelimit.Limiter
}

// probeOutcome is how a candidate sweep ended.
type probeOutcome int

// probeResult carries a sweep's conclusion and any confirmed tag.
type probeResult struct {
	// tag is the confirmed spelling, or empty.
	tag string
	// outcome is how the sweep ended.
	outcome probeOutcome
}

// NewResolver returns a Resolver that reaches the registry through fetcher.
//
// A Resolver with a nil fetcher makes no lookups, and ResolveTag returns empty.
//
// Parameters:
//   - fetcher: Fetcher to use, such as a DigestFetcher.
//
// Returns:
//   - *Resolver: Resolver ready for use.
func NewResolver(fetcher Fetcher) *Resolver {
	return &Resolver{fetcher: fetcher, now: time.Now}
}

// NewDigestFetcher returns a DigestFetcher that makes registry requests with
// settings, paced by rateLimiter.
//
// Parameters:
//   - settings: Registry request settings.
//   - rateLimiter: Rate limiter shared with the process's other registry
//     requests. Must not be nil.
//
// Returns:
//   - DigestFetcher: Fetcher ready for use.
func NewDigestFetcher(settings registryConfig.Registry, rateLimiter *ratelimit.Limiter) DigestFetcher {
	return DigestFetcher{settings: settings, rateLimiter: rateLimiter}
}

// ResolveTag returns the release tag whose manifest matches newDigest, or empty
// when no candidate spelling matches or the lookup could not be completed.
//
// The most likely spelling is tried first: a version that already carries a "v"
// is used as-is, otherwise the "v"-prefixed form leads. The alternative is only
// tried when the first does not match, so a common case costs one request and
// a worst case costs two.
//
// A rate limit on the first candidate abandons the second. Probing is an
// optional convenience, so it must never make a session slower or noisier than
// the unversioned link it replaces.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for request lifecycle control.
//   - container: Container whose image supplies the repository and registry host.
//   - registryAuth: Base64-encoded auth string for registry access.
//   - version: OCI version annotation of the new image.
//   - newDigest: Manifest digest of the new image.
//   - endpoints: Optional registry mirror host overrides to try first.
//
// Returns:
//   - string: The confirmed tag spelling, or empty.
func (r *Resolver) ResolveTag(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	version string,
	newDigest string,
	endpoints ...string,
) string {
	if r == nil || r.fetcher == nil || container == nil || version == "" || newDigest == "" {
		return ""
	}

	fields := map[string]any{
		"container": container.Name(),
		"image":     container.ImageName(),
		"version":   version,
	}

	pinned, err := isDigestPinned(container.ImageName())
	if err != nil || pinned {
		log.Debug().
			Fields(fields).
			Msg("Skipping release tag lookup for a digest-pinned image")

		return ""
	}

	key := cacheKey(container, newDigest)
	if cached, ok := r.loadAnswer(key); ok {
		log.Debug().
			Fields(fields).
			Str("cache_key", key).
			Str("tag", cached.tag).
			Msg("Using cached release tag lookup")

		return cached.tag
	}

	outcome := r.probe(log, ctx, container, registryAuth, version, newDigest, endpoints...)

	switch outcome.outcome {
	case outcomeMatched, outcomeNoMatch:
		// A match and a clean sweep that found nothing are both final answers, so
		// cache them indefinitely. That keeps a latest-only mirror from being
		// asked again for every sibling container.
		r.storeAnswer(key, cachedAnswer{tag: outcome.tag})
	case outcomeRateLimited:
		// Every sibling on this host would be refused too, so remember the
		// refusal briefly rather than spend one doomed request each.
		r.storeAnswer(key, cachedAnswer{expires: r.clock()().Add(rateLimitCacheTTL)})
	case outcomeFailed:
		// A transport failure says nothing about the other candidate, so leave
		// the gap uncached and let the next sibling try.
	}

	if outcome.tag == "" {
		log.Debug().
			Fields(fields).
			Str("outcome", outcome.outcome.name()).
			Msg("No release tag matched the pulled image")
	}

	return outcome.tag
}

// tagCandidates returns the release tag spellings to try, most likely first.
//
// A version that already carries a "v" is assumed to match its own release tag.
// Otherwise the "v"-prefixed form leads, because the common case is an OCI
// version written without a prefix and a release tag written with one. The
// alternative spelling is always last so it is only tried when the first misses.
//
// Parameters:
//   - version: OCI version annotation of the new image.
//
// Returns:
//   - []string: One or two candidate tag spellings, in priority order.
func tagCandidates(version string) []string {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}

	trimmed := strings.TrimPrefix(version, "v")
	if trimmed == version {
		return []string{"v" + version, version}
	}

	if trimmed == "" {
		return []string{version}
	}

	return []string{version, trimmed}
}

// FetchDigestForTag returns the manifest digest of a specific tag.
//
// Parameters:
//   - log: Logger.
//   - ctx: The context controlling the request's lifecycle.
//   - container: Container whose image supplies the repository and registry host.
//   - authToken: A base64-encoded authentication string for registry access.
//   - tag: Exact tag to fetch the digest for.
//   - endpoints: Optional registry mirror host overrides to try before the canonical host.
//
// Returns:
//   - string: The normalized digest, or empty when the tag does not exist.
//   - error: Non-nil if the request fails, nil on success.
func (f DigestFetcher) FetchDigestForTag(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	authToken string,
	tag string,
	endpoints ...string,
) (string, error) {
	remote, err := digest.FetchDigestForTag(log, ctx, container, authToken, tag, f.settings, f.rateLimiter, endpoints...)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errFetchTagDigest, err)
	}

	return remote, nil
}

// name returns a loggable label for the outcome.
func (o probeOutcome) name() string {
	switch o {
	case outcomeMatched:
		return "matched"
	case outcomeNoMatch:
		return "no_match"
	case outcomeRateLimited:
		return "rate_limited"
	case outcomeFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// probe tries each candidate spelling and reports how the sweep ended.
//
// A candidate that fails leaves the sweep inconclusive, because a transport
// error does not prove the other spelling is absent. A rate limit ends the sweep
// outright, because it is a statement about the host rather than this request.
//
// Parameters:
//   - log: Process logger.
//   - ctx: Context for request lifecycle control.
//   - container: Container whose image supplies the repository and registry host.
//   - registryAuth: Base64-encoded auth string for registry access.
//   - version: OCI version annotation of the new image.
//   - newDigest: Manifest digest of the new image.
//   - endpoints: Optional registry mirror host overrides to try first.
//
// Returns:
//   - probeResult: The confirmed tag, if any, and the sweep outcome.
func (r *Resolver) probe(
	log *zerolog.Logger,
	ctx context.Context,
	container types.Container,
	registryAuth string,
	version string,
	newDigest string,
	endpoints ...string,
) probeResult {
	failed := false

	for i, tag := range tagCandidates(version) {
		found, err := r.fetcher.FetchDigestForTag(log, ctx, container, registryAuth, tag, endpoints...)
		if err != nil {
			if ratelimit.Is(err) {
				// The registry is already throttling this host. Abandon the probe
				// rather than spend a second request the registry will refuse.
				log.Debug().
					Err(err).
					Str("container", container.Name()).
					Str("image", container.ImageName()).
					Str("tag", tag).
					Msg("Abandoning release tag lookup after a rate limit")

				return probeResult{outcome: outcomeRateLimited}
			}

			log.Debug().
				Err(err).
				Str("container", container.Name()).
				Str("image", container.ImageName()).
				Str("tag", tag).
				Msg("Release tag candidate lookup failed")

			failed = true

			continue
		}

		if found == "" {
			continue
		}

		if !sameManifestDigest(log, newDigest, found) {
			continue
		}

		log.Debug().
			Str("container", container.Name()).
			Str("image", container.ImageName()).
			Str("tag", tag).
			Bool("first_candidate", i == 0).
			Msg("Confirmed release tag spelling for the pulled image")

		return probeResult{tag: tag, outcome: outcomeMatched}
	}

	if failed {
		return probeResult{outcome: outcomeFailed}
	}

	return probeResult{outcome: outcomeNoMatch}
}

// clock returns the resolver's time source, defaulting to the wall clock.
//
// Returns:
//   - func() time.Time: Time source.
func (r *Resolver) clock() func() time.Time {
	if r.now == nil {
		return time.Now
	}

	return r.now
}

// storeAnswer caches a result for as long as it should be trusted.
//
// Parameters:
//   - key: Cache key.
//   - answer: Result to remember.
//
// Returns:
//   - none.
func (r *Resolver) storeAnswer(key string, answer cachedAnswer) {
	r.cache.Store(key, answer)
}

// loadAnswer returns a cached result, discarding one that has lapsed.
//
// Parameters:
//   - key: Cache key.
//
// Returns:
//   - cachedAnswer: The remembered result.
//   - bool: True when a live entry was found.
func (r *Resolver) loadAnswer(key string) (cachedAnswer, bool) {
	stored, ok := r.cache.Load(key)
	if !ok {
		return cachedAnswer{}, false
	}

	answer, isCachedAnswer := stored.(cachedAnswer)
	if !isCachedAnswer {
		return cachedAnswer{}, false
	}

	if !answer.expires.IsZero() && r.clock()().After(answer.expires) {
		r.cache.Delete(key)

		return cachedAnswer{}, false
	}

	return answer, true
}

// sameManifestDigest reports whether two manifest digests describe the same
// image.
//
// Both sides are bare manifest digests rather than the repo-scoped
// "repo@sha256:..." form that container image info carries, so the
// digest package's repo-scoped digest comparison does not apply. Normalization strips a leading
// "sha256:" because a HEAD response may omit it.
//
// Parameters:
//   - log: Process logger.
//   - a: First manifest digest.
//   - b: Second manifest digest.
//
// Returns:
//   - bool: True when both digests normalize to the same value.
func sameManifestDigest(log *zerolog.Logger, a, b string) bool {
	return digest.NormalizeDigest(log, a) == digest.NormalizeDigest(log, b)
}

// cacheKey identifies a lookup so sibling containers on one image share it.
//
// The new digest is part of the key because a later session may pull a
// different image for the same repository, and the previous answer would then
// be wrong.
//
// Parameters:
//   - container: Container whose image supplies the repository.
//   - newDigest: Manifest digest of the new image.
//
// Returns:
//   - string: Cache key.
func cacheKey(container types.Container, newDigest string) string {
	host, path, _ := splitReference(container.ImageName())

	return strings.Join([]string{host, path, newDigest}, "|")
}
