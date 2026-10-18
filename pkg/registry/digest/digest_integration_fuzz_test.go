package digest_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/digest"
)

// FuzzExtractHeadDigest fuzzes the header parsing in extractHeadDigest to test for crashes or unexpected behavior with malformed inputs.
func FuzzExtractHeadDigest(f *testing.F) {
	// Seed with known good and bad inputs
	f.Add(200, "sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add(200, "")
	f.Add(404, "sha256:invalid")
	f.Add(500, "malformed-digest")
	f.Add(200, "invalid-digest")
	f.Add(200, "sha256:")
	f.Add(200, "sha256:short")

	f.Fuzz(func(_ *testing.T, statusCode int, headerValue string) {
		// Create a mock response with the fuzzed status and header
		resp := &http.Response{
			StatusCode: statusCode,
			Header:     http.Header{},
		}

		resp.Header.Set(digest.ContentDigestHeader, headerValue)

		// Call extractHeadDigest because we don't care about the result, only that it doesn't panic
		_, _ = digest.ExtractHeadDigest(newNopLogger(), resp)
	})
}

// FuzzExtractGetDigest fuzzes the body parsing in ExtractGetDigest to test for crashes or unexpected behavior with malformed inputs.
func FuzzExtractGetDigest(f *testing.F) {
	// Seed with known good and bad inputs
	f.Add([]byte(`{"digest": "sha256:abc123"}`))
	f.Add([]byte(`invalid json`))
	f.Add([]byte(`sha256:valid`))
	f.Add([]byte(``))
	f.Add([]byte(`{"digest": ""}`))

	f.Fuzz(func(_ *testing.T, body []byte) {
		// Create a mock response with the fuzzed body
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(body)),
		}

		resp.Header.Set("Content-Type", "application/json")
		defer resp.Body.Close()

		// Call ExtractGetDigest because we don't care about the result, only that it doesn't panic
		_, _ = digest.ExtractGetDigest(newNopLogger(), resp)
	})
}

// FuzzNormalizeDigest fuzzes the NormalizeDigest function to test for crashes or unexpected behavior with malformed inputs.
func FuzzNormalizeDigest(f *testing.F) {
	// Seed with known good and bad inputs
	f.Add("sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add("")
	f.Add("d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547")
	f.Add("md5:1234567890abcdef")
	f.Add("sha256:")
	f.Add(strings.Repeat("a", 1000))
	f.Add("sha256:abc!@#$%^&*()")

	f.Fuzz(func(_ *testing.T, input string) {
		// Call NormalizeDigest because we don't care about the result, only that it doesn't panic
		_ = digest.NormalizeDigest(newNopLogger(), input)
	})
}

// FuzzDigestsMatch fuzzes the digestsMatch function to ensure robust handling of malformed inputs.
// It tests arrays of digest strings and remote digest strings with various edge cases.
func FuzzDigestsMatch(f *testing.F) {
	// Seed with matching digests
	f.Add(`["repo@sha256:abc123"]`, "sha256:abc123")
	// Seed with non-matching digests
	f.Add(`["repo@sha256:abc123"]`, "sha256:def456")
	// Seed with empty array
	f.Add(`[]`, "sha256:abc123")
	// Seed with malformed local digest (no @)
	f.Add(`["malformed-digest"]`, "sha256:abc123")
	// Seed with invalid digest format
	f.Add(`["repo@invalid-digest"]`, "sha256:abc123")
	// Seed with multiple digests, one matching
	f.Add(`["repo1@sha256:abc123", "repo2@sha256:def456"]`, "sha256:abc123")
	// Seed with empty part after @
	f.Add(`["repo@"]`, "sha256:abc123")
	// Seed with empty remote digest
	f.Add(`["repo@sha256:abc123"]`, "")
	// Seed with invalid remote digest
	f.Add(`["repo@sha256:abc123"]`, "invalid-remote")
	// Seed with malformed JSON
	f.Add(`invalid json`, "sha256:abc123")
	// Seed with nil-like empty
	f.Add(`null`, "sha256:abc123")

	f.Fuzz(func(_ *testing.T, localJSON, remote string) {
		var localDigests []string
		// Ignore unmarshal errors to test robustness with malformed JSON
		json.Unmarshal([]byte(localJSON), &localDigests)
		// Call DigestsMatch because we don't care about the result, only that it doesn't panic
		digest.DigestsMatch(newNopLogger(), localDigests, remote)
	})
}

// FuzzBuildManifestURL fuzzes the buildManifestURL function to ensure robust URL construction.
// It tests image reference strings and host override strings with various edge cases.
func FuzzBuildManifestURL(f *testing.F) {
	// Seed with known good and bad inputs
	f.Add("ghcr.io/k6io/operator:latest", "")
	f.Add("docker.io/library/alpine:latest", "registry.example.com")
	f.Add("localhost:5000/myimage:v1.0", "127.0.0.1:5000")
	f.Add("invalid image ref", "")
	f.Add("", "example.com")
	f.Add("example.com/test/image:", "")
	f.Add("http://invalid url with spaces/test/image:latest", "bad host")
	f.Add(strings.Repeat("a", 1000), strings.Repeat("b", 1000))
	f.Add("lscr.io/test/image:latest", "") // Special case for lscr.io
	f.Add("registry.example.com/image", "overridden.host")

	f.Fuzz(func(t *testing.T, imageRef, hostOverride string) {
		// Create a mock container with the fuzzed image reference
		mockContainer := mockActions.CreateMockContainer(
			"mock-id",
			"mock-name",
			imageRef,
			time.Now(),
		)

		// Call buildManifestURL because we don't care about the result, only that it doesn't panic
		_, _, _, _ = digest.BuildManifestURL(newNopLogger(), mockContainer, registryConfig.Registry{}, hostOverride)
	})
}

// FuzzHandleManifestResponse fuzzes the HandleManifestResponse function to test for crashes or unexpected behavior
// with malformed HTTP responses, status codes, headers, and redirect URLs.
func FuzzHandleManifestResponse(f *testing.F) {
	// Seed with valid responses
	f.Add(
		200,
		"HEAD",
		"example.com",
		"challenge.com",
		"example.com",
		"sha256:d68e1e532088964195ad3a0a71526bc2f11a78de0def85629beb75e2265f0547",
		false,
	)
	f.Add(200, "GET", "example.com", "", "example.com", `{"digest": "sha256:abc123"}`, true)
	f.Add(
		302,
		"HEAD",
		"example.com",
		"challenge.com",
		"example.com",
		"https://redirect.example.com/v2/repo/manifests/tag",
		false,
	)

	// Seed with malformed headers
	f.Add(200, "HEAD", "example.com", "", "example.com", "", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "invalid-digest", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "sha256:", false)
	f.Add(200, "HEAD", "example.com", "", "example.com", "sha256:short", false)

	// Seed with malformed status codes
	f.Add(404, "HEAD", "example.com", "", "example.com", "", false)
	f.Add(500, "GET", "example.com", "", "example.com", "", false)
	f.Add(301, "HEAD", "example.com", "", "example.com", "invalid://url", false)

	// Seed with edge cases
	f.Add(302, "HEAD", "example.com", "challenge.com", "example.com", "", true)
	f.Add(401, "HEAD", "example.com", "challenge.com", "challenge.com", "", false)
	f.Add(404, "HEAD", "example.com", "challenge.com", "challenge.com", "", false)

	f.Fuzz(
		func(_ *testing.T, statusCode int, method, originalHost, challengeHost, currentHost, headerValue string, redirected bool) {
			// Create a parsed URL for testing
			parsedURL, err := url.Parse("https://example.com/v2/test/manifests/latest")
			if err != nil {
				return // Skip if URL parsing fails
			}

			// Create a mock HTTP response with fuzzed status code and headers
			resp := &http.Response{
				StatusCode: statusCode,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
				Request:    &http.Request{URL: parsedURL},
			}

			// Set headers based on status code
			if statusCode >= 300 && statusCode < 400 {
				// For redirects, set Location header
				resp.Header.Set("Location", headerValue)
			} else {
				// For success/error, set digest header
				resp.Header.Set(digest.ContentDigestHeader, headerValue)
				// For GET requests, set body if it's a JSON response
				if method == "GET" && strings.HasPrefix(headerValue, "{") {
					resp.Body = io.NopCloser(bytes.NewReader([]byte(headerValue)))
					resp.Header.Set("Content-Type", "application/json")
				}
			}

			// Call HandleManifestResponse because we don't care about the result, only that it doesn't panic
			_, _, _, _ = digest.HandleManifestResponse(newNopLogger(), resp,
				method,
				originalHost,
				challengeHost,
				redirected,
				parsedURL,
				currentHost,
			)
		},
	)
}
