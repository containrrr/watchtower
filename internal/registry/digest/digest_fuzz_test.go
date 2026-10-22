package digest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
)

// FuzzExtractHeadDigest checks that a HEAD response's digest is its normalized
// Docker-Content-Digest header whatever the status code, that a missing header
// is an invalid registry response, and that a header with no hash yields an
// empty digest and no error so the caller falls back to GET.
//
// The seeds pair successful and failing status codes with a full digest, an
// empty header, malformed values, and a bare or short sha256 prefix.
func FuzzExtractHeadDigest(f *testing.F) {
	f.Add(200, "sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add(200, "")
	f.Add(404, "sha256:invalid")
	f.Add(500, "malformed-digest")
	f.Add(200, "invalid-digest")
	f.Add(200, "sha256:")
	f.Add(200, "sha256:short")

	f.Fuzz(func(t *testing.T, statusCode int, headerValue string) {
		resp := &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		}
		resp.Header.Set(ContentDigestHeader, headerValue)

		got, err := extractHeadDigest(newNopLogger(), resp)

		header := resp.Header.Get(ContentDigestHeader)
		if header == "" {
			require.ErrorIs(t, err, errInvalidRegistryResponse)
			assert.Empty(t, got)

			return
		}

		require.NoError(t, err)
		assert.Equal(t, NormalizeDigest(newNopLogger(), header), got)
	})
}

// FuzzExtractGetDigest checks that a GET response's digest header takes
// precedence over its body, that a successful extraction is never empty, and
// that every failure is an invalid registry response or an oversized manifest.
//
// The seeds cover JSON bodies with a digest, an empty digest, and invalid JSON,
// plain text digests, empty bodies, each with JSON and plain text content
// types, and a header that must win over an unparseable body.
func FuzzExtractGetDigest(f *testing.F) {
	f.Add([]byte(`{"digest": "sha256:abc123"}`), "application/json", "")
	f.Add([]byte(`invalid json`), "application/json", "")
	f.Add([]byte(`sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547`), "text/plain", "")
	f.Add([]byte(``), "", "")
	f.Add([]byte(`{"digest": ""}`), "application/vnd.oci.image.index.v1+json", "")
	f.Add([]byte(`{"digest": "sha256:abc123"}`), "text/plain", "")
	f.Add([]byte(`invalid json`), "", "sha256:abc123")
	f.Add([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}`),
		"application/vnd.oci.image.index.v1+json", "")

	f.Fuzz(func(t *testing.T, body []byte, contentType, headerValue string) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}
		resp.Header.Set("Content-Type", contentType)
		resp.Header.Set(ContentDigestHeader, headerValue)

		got, err := extractGetDigest(newNopLogger(), resp)

		if headerDigest := NormalizeDigest(newNopLogger(), resp.Header.Get(ContentDigestHeader)); headerDigest != "" {
			require.NoError(t, err)
			assert.Equal(t, headerDigest, got)

			return
		}

		if err != nil {
			assert.True(t,
				errors.Is(err, errInvalidRegistryResponse) || errors.Is(err, errManifestTooLarge),
				"unexpected error: %v", err)
			assert.Empty(t, got)

			return
		}

		assert.NotEmpty(t, got)
	})
}

// FuzzNormalizeDigest checks that normalizing strips at most a leading sha256:
// prefix, so the result is a suffix of the input, and that formatting the
// result restores a sha256 digest.
//
// The seeds cover a full sha256 digest, a bare hash, another algorithm, an
// empty string, a bare prefix, a long value, and punctuation after the prefix.
func FuzzNormalizeDigest(f *testing.F) {
	f.Add("sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add("")
	f.Add("d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add("md5:1234567890abcdef")
	f.Add("sha256:")
	f.Add(strings.Repeat("a", 1000))
	f.Add("sha256:abc!@#$%^&*()")

	f.Fuzz(func(t *testing.T, input string) {
		got := NormalizeDigest(newNopLogger(), input)

		assert.True(t, strings.HasSuffix(input, got), "%q is not a suffix of %q", got, input)

		hash, prefixed := strings.CutPrefix(input, "sha256:")
		if !prefixed {
			assert.Equal(t, input, got)

			return
		}

		assert.Equal(t, hash, got)

		// A bare hash formats back to the sha256 digest it came from.
		if hash != "" && !strings.Contains(hash, ":") {
			assert.Equal(t, input, formatDigest(got))
		}
	})
}

// FuzzDigestsMatch checks that an empty remote digest matches nothing, that a
// non-empty remote digest without @ always matches a local repo digest built
// from it, and that the result does not depend on the order of the local
// digests or on extra entries without an @ separator.
//
// The seeds are JSON arrays of local digests with a matching, a different, an
// empty, and a malformed remote digest, plus empty, malformed, and null arrays.
func FuzzDigestsMatch(f *testing.F) {
	f.Add(`["repo@sha256:abc123"]`, "sha256:abc123")
	f.Add(`["repo@sha256:abc123"]`, "sha256:def456")
	f.Add(`[]`, "sha256:abc123")
	f.Add(`["malformed-digest"]`, "sha256:abc123")
	f.Add(`["repo@invalid-digest"]`, "sha256:abc123")
	f.Add(`["repo1@sha256:abc123", "repo2@sha256:def456"]`, "sha256:abc123")
	f.Add(`["repo@"]`, "sha256:abc123")
	f.Add(`["repo@sha256:abc123"]`, "")
	f.Add(`["repo@sha256:abc123"]`, "invalid-remote")
	f.Add(`invalid json`, "sha256:abc123")
	f.Add(`null`, "sha256:abc123")

	f.Fuzz(func(t *testing.T, localJSON, remote string) {
		var localDigests []string
		// Malformed JSON leaves no local digests, which must not match.
		_ = json.Unmarshal([]byte(localJSON), &localDigests)

		log := newNopLogger()
		got := digestsMatch(log, localDigests, remote)

		if len(localDigests) == 0 || NormalizeDigest(log, remote) == "" {
			assert.False(t, got)
		}

		reversed := slices.Clone(localDigests)
		slices.Reverse(reversed)
		assert.Equal(t, got, digestsMatch(log, reversed, remote), "order of local digests changed the result")

		withMalformed := append(slices.Clone(localDigests), "no-separator")
		assert.Equal(t, got, digestsMatch(log, withMalformed, remote), "an entry without @ changed the result")

		// Only a non-empty digest without @ is a remote digest a registry can send.
		if NormalizeDigest(log, remote) != "" && !strings.Contains(remote, "@") {
			withRemote := append(slices.Clone(localDigests), "repo@"+remote)
			assert.True(t, digestsMatch(log, withRemote, remote), "repo@%s does not match %q", remote, remote)
		}
	})
}

// FuzzBuildManifestURLForImage checks that a manifest URL built for an image
// reference is a /v2/ manifest URL on the registry's host with the scheme the
// TLS setting selects, that the URL parses back to the host it was built with,
// that a host override replaces its host, and that every failure, including an
// override that is not a host, is a manifest URL build error.
//
// The seeds cover registries with and without ports, Docker Hub, lscr.io,
// invalid references, an empty reference, long values, and host overrides as
// bare hosts, full URLs, and malformed strings.
func FuzzBuildManifestURLForImage(f *testing.F) {
	f.Add("ghcr.io/k6io/operator:latest", "", false)
	f.Add("docker.io/library/alpine:latest", "registry.example.com", false)
	f.Add("localhost:5000/myimage:v1.0", "127.0.0.1:5000", true)
	f.Add("nginx:latest", "http://mirror.example.com:5000", false)
	f.Add("invalid image ref", "", false)
	f.Add("", "example.com", false)
	f.Add("example.com/test/image:", "", true)
	f.Add("http://invalid url with spaces/test/image:latest", "bad host", false)
	f.Add(strings.Repeat("a", 1000), strings.Repeat("b", 1000), false)
	f.Add("lscr.io/test/image:latest", "", false)
	f.Add("registry.example.com/image", "overridden.host", true)

	f.Fuzz(func(t *testing.T, imageRef, hostOverride string, tlsSkip bool) {
		settings := registryConfig.Registry{TLSSkip: tlsSkip}

		got, _, parsed, err := buildManifestURLForImage(newNopLogger(), imageRef, "fuzz-container", settings, hostOverride)
		if err != nil {
			require.ErrorIs(t, err, errFailedBuildManifestURL)
			assert.Empty(t, got)

			return
		}

		require.NotNil(t, parsed)
		assert.True(t, strings.HasPrefix(parsed.Path, "/v2/"), "path %q is not under /v2/", parsed.Path)
		assert.Contains(t, parsed.Path, "/manifests/")

		// The returned URL parses back to the same host, with or without an override.
		u, parseErr := url.Parse(got)
		require.NoError(t, parseErr)
		assert.NotEmpty(t, u.Host)
		assert.Equal(t, parsed.Host, u.Host)

		wantScheme := "https"
		if tlsSkip {
			wantScheme = "http"
		}

		overrideURL, overrideErr := url.Parse(hostOverride)

		switch {
		case hostOverride == "":
			assert.Equal(t, wantScheme, u.Scheme)
		case overrideErr == nil && overrideURL.Host != "":
			assert.Equal(t, overrideURL.Host, u.Host)
		default:
			// A bare host override is the URL's host and keeps the TLS scheme.
			assert.Equal(t, hostOverride, u.Host)
			assert.Equal(t, wantScheme, u.Scheme)
		}
	})
}

// FuzzHandleManifestResponse checks that a manifest response leads to exactly
// one outcome: a retry with a new URL, a digest, or an error that is an
// invalid registry response or a rate limit.
//
// The seeds cover HEAD and GET successes with digest headers and JSON bodies,
// redirects with valid and invalid locations, missing and malformed digests,
// 401, 404, and 500 statuses, and redirected and original hosts.
func FuzzHandleManifestResponse(f *testing.F) {
	f.Add(200, "HEAD", "example.com", "challenge.com", "example.com",
		"sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547", false)
	f.Add(200, "GET", "example.com", "", "example.com", `{"digest": "sha256:abc123"}`, true)
	f.Add(302, "HEAD", "example.com", "challenge.com", "example.com",
		"https://redirect.example.com/v2/repo/manifests/tag", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "invalid-digest", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "sha256:", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "sha256:short", false)
	f.Add(404, "HEAD", "example.com", "", "example.com", "", false)
	f.Add(500, "GET", "example.com", "", "example.com", "", false)
	f.Add(301, "HEAD", "example.com", "", "example.com", "invalid://url", false)
	f.Add(302, "HEAD", "example.com", "challenge.com", "example.com", "", true)
	f.Add(401, "HEAD", "example.com", "challenge.com", "challenge.com", "", false)
	f.Add(404, "HEAD", "example.com", "challenge.com", "challenge.com", "", false)
	f.Add(429, "GET", "example.com", "", "example.com", "", false)

	f.Fuzz(func(
		t *testing.T,
		statusCode int,
		method, originalHost, challengeHost, currentHost, headerValue string,
		redirected bool,
	) {
		parsedURL, err := url.Parse("https://example.com/v2/test/manifests/latest")
		require.NoError(t, err)

		resp := &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader([]byte{})),
			Request:    &http.Request{URL: parsedURL},
		}

		if statusCode >= 300 && statusCode < 400 {
			resp.Header.Set("Location", headerValue)
		} else {
			resp.Header.Set(ContentDigestHeader, headerValue)

			if method == http.MethodGet && strings.HasPrefix(headerValue, "{") {
				resp.Body = io.NopCloser(bytes.NewReader([]byte(headerValue)))
				resp.Header.Set("Content-Type", "application/json")
			}
		}

		digest, manifestURL, retry, err := handleManifestResponse(newNopLogger(), resp,
			method, originalHost, challengeHost, redirected, parsedURL, currentHost)

		switch {
		case retry:
			require.NoError(t, err)
			assert.Empty(t, digest)
			assert.NotEmpty(t, manifestURL)
		case err != nil:
			assert.True(t,
				errors.Is(err, errInvalidRegistryResponse) || errors.Is(err, ratelimit.ErrRateLimited),
				"unexpected error: %v", err)
			assert.Empty(t, digest)
			assert.Empty(t, manifestURL)
		default:
			assert.Empty(t, manifestURL)
		}
	})
}
