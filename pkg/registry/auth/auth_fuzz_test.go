package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// mockClient is a mock implementation of the Client interface for fuzz testing.
type mockClient struct {
	body []byte
}

func (m *mockClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(m.body)),
	}, nil
}

// FuzzGetBearerToken fuzzes the JSON unmarshaling in getBearerToken to ensure robust parsing
// of token responses from registries. It tests various JSON inputs including valid responses,
// malformed JSON, missing fields, and edge cases to prevent crashes or unexpected behavior.
func FuzzGetBearerToken(f *testing.F) {
	// Seed with valid JSON responses
	f.Add([]byte(`{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"}`))
	f.Add([]byte(`{"token":""}`))
	f.Add([]byte(`{"token":null}`))

	// Seed with malformed JSON
	f.Add([]byte(`{"token":"test"`))
	f.Add([]byte(`invalid json`))
	f.Add([]byte(`{"token":}`))
	f.Add([]byte(`{"token": "test",}`))

	// Seed with missing fields
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"other":"field"}`))
	f.Add([]byte(`[]`))

	// Seed with edge cases
	f.Add([]byte(`{"token":123}`))
	f.Add([]byte(`{"token":true}`))
	f.Add([]byte(`{"token":{"nested":"object"}}`))
	f.Add([]byte(`{"token":["array"]}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add(
		[]byte(
			`{"token":"very long token string that might cause issues if not handled properly in some parsers but json should handle it fine"}`,
		),
	)

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		initTokenCache(newNopLogger())
		tokenCache.InvalidateAll()

		// Attempt to unmarshal the JSON data into a TokenResponse struct
		var tokenResponse types.TokenResponse

		_ = json.Unmarshal(jsonData, &tokenResponse)

		// Call getBearerToken with a mock client that returns the fuzzed JSON data as the response body
		// to exercise the auth package logic
		ctx := context.Background()

		imageRef, err := reference.ParseNormalizedNamed("test/image")
		if err != nil {
			t.Fatal(err)
		}

		challenge := `bearer realm="https://test.com/token",service="test.com"`
		registryAuth := ""
		client := &mockClient{body: jsonData}
		_, _ = getBearerToken(newNopLogger(), ctx, challenge, imageRef, registryAuth, client, registryConfig.Registry{})
	})
}

// FuzzTransformAuth fuzzes the TransformAuth function to test for crashes or unexpected behavior
// with various base64-encoded inputs. It ensures robust decoding and unmarshaling of JSON credentials,
// covering valid inputs, malformed base64, invalid JSON, and edge cases.
func FuzzTransformAuth(f *testing.F) {
	// Seed with valid base64-encoded JSON credentials
	f.Add(
		"eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzcyJ9",
	) // {"username":"testuser","password":"testpass"}
	// Seed with valid base64 but incomplete JSON (missing password)
	f.Add("eyJ1c2VybmFtZSI6InVzZXIifQ==") // {"username":"user"}
	// Seed with valid base64 but password only
	f.Add("eyJwYXNzd29yZCI6InBhc3MifQ==") // {"password":"pass"}
	// Seed with valid base64 but empty JSON
	f.Add("e30=") // {}
	// Seed with invalid JSON (malformed)
	f.Add("eyJ1c2VybmFtZSI6InVzZXIi") // {"username":"user" (missing closing)
	// Seed with invalid base64
	f.Add("notbase64")
	f.Add("aGVsbG8=") // "hello" base64, not JSON
	// Seed with empty string
	f.Add("")
	// Seed with edge cases
	f.Add("dGVzdA==") // "test" base64
	f.Add(
		"eyJ1c2VybmFtZSI6bnVsbCwgcGFzc3dvcmQiOiJwYXNzIn0=",
	) // {"username":null, "password":"pass"}
	f.Fuzz(func(_ *testing.T, input string) {
		// Call TransformAuth because we don't care about the result, only that it doesn't panic
		TransformAuth(newNopLogger(), input)
	})
}

// FuzzGetAuthURL fuzzes the getAuthURL function to test for crashes or unexpected behavior
// with malformed authentication challenge strings. It ensures robust URL construction from
// challenge headers, covering valid challenges, malformed headers, missing fields, and edge cases.
func FuzzGetAuthURL(f *testing.F) {
	// Parse a fixed image reference for fuzzing
	imageRef, err := reference.ParseNormalizedNamed("ghcr.io/test/image")
	if err != nil {
		panic(err)
	}

	// Seed with valid challenge headers
	f.Add(
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull"`,
	)
	f.Add(
		`bearer realm="https://registry.example.com/token",service="registry.example.com",scope="repository:user/repo:pull"`,
	)
	f.Add(
		`bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/alpine:pull"`,
	)

	// Seed with malformed headers - missing fields
	f.Add(`bearer realm="https://ghcr.io/token"`)      // Missing service
	f.Add(`bearer service="ghcr.io"`)                  // Missing realm
	f.Add(`bearer scope="repository:test/image:pull"`) // Missing realm and service

	// Seed with empty or invalid headers
	f.Add(``)                   // Empty header
	f.Add(`basic realm="test"`) // Wrong auth type
	f.Add(`bearer`)             // Just bearer
	f.Add(`bearer realm=""`)    // Empty realm
	f.Add(`bearer service=""`)  // Empty service

	// Seed with malformed syntax
	f.Add(
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull",`,
	) // Trailing comma
	f.Add(
		`bearer realm="https://ghcr.io/token" service="ghcr.io"`,
	) // Wrong separator
	f.Add(
		`bearer realm="https://ghcr.io/token",service="ghcr.io",invalidkey`,
	) // Valueless key
	f.Add(
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope=`,
	) // Empty scope

	// Seed with edge cases
	f.Add(
		`BEARER realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull"`,
	) // Uppercase
	f.Add(
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull",extra="value"`,
	) // Extra fields
	f.Add(
		`bearer realm="http://localhost:5000/token",service="localhost:5000",scope="repository:test/image:pull"`,
	) // Local registry

	f.Fuzz(func(_ *testing.T, challenge string) {
		// Call getAuthURL because we don't care about the result, only that it doesn't panic
		_, _ = getAuthURL(newNopLogger(), challenge, imageRef, "", registryConfig.Registry{})
	})
}

// FuzzComputeTokenExpiry fuzzes the computeTokenExpiry function to test for
// panics or unexpected behavior with various TokenResponse inputs.
func FuzzComputeTokenExpiry(f *testing.F) {
	f.Add([]byte(`{"expires_in":3600}`))
	f.Add([]byte(`{"expires_in":0}`))
	f.Add([]byte(`{"expires_in":-1}`))
	f.Add([]byte(`{"issued_at":"2024-01-01T00:00:00Z"}`))
	f.Add([]byte(`{"issued_at":"not-a-date"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))

	f.Fuzz(func(_ *testing.T, jsonData []byte) {
		var tokenResponse types.TokenResponse

		_ = json.Unmarshal(jsonData, &tokenResponse)

		_ = computeTokenExpiry(newNopLogger(), &tokenResponse)
	})
}

// FuzzReadBearerTokenWithExpiry fuzzes the readBearerTokenWithExpiry function
// to test for panics or unexpected behavior with various JSON inputs.
func FuzzReadBearerTokenWithExpiry(f *testing.F) {
	f.Add([]byte(`{"token":"test","expires_in":3600}`))
	f.Add([]byte(`{"token":""}`))
	f.Add([]byte(`{"token":null}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`invalid`))
	f.Add([]byte(``))

	f.Fuzz(func(_ *testing.T, jsonData []byte) {
		_, _, _ = readBearerTokenWithExpiry(newNopLogger(), bytes.NewReader(jsonData), "test/image")
	})
}

// FuzzParseChallenge fuzzes the parseChallenge function to test for panics
// with various challenge header inputs.
func FuzzParseChallenge(f *testing.F) {
	f.Add(`bearer realm="https://ghcr.io/token",service="ghcr.io"`)
	f.Add(`BEARER realm="https://ghcr.io/token"`)
	f.Add(``)
	f.Add(`bearer`)
	f.Add(`bearer realm=""`)
	f.Add(`basic realm="test"`)

	f.Fuzz(func(_ *testing.T, header string) {
		_ = parseChallenge(header)
	})
}

// FuzzExtractChallengeHost fuzzes the extractChallengeHost function to test
// for panics with various realm URL inputs.
func FuzzExtractChallengeHost(f *testing.F) {
	f.Add("https://ghcr.io/token")
	f.Add("http://localhost:5000/token")
	f.Add("ghcr.io/token")
	f.Add("")
	f.Add("https://registry.example.com:5000/token")

	f.Fuzz(func(_ *testing.T, realm string) {
		_ = extractChallengeHost(newNopLogger(), realm)
	})
}

// FuzzResolveEndpoint checks that a resolved registry endpoint is a host that
// parses back unchanged from the URL built with it, with an http or https
// scheme, and that every rejected endpoint is an invalid registry endpoint.
//
// The seeds cover http and https URLs with and without ports, a
// scheme-relative URL, bare hosts, IPv4 and IPv6 addresses with ports, an empty
// endpoint, and endpoints with spaces, paths, queries, user info, a bad port,
// and an unsupported scheme.
func FuzzResolveEndpoint(f *testing.F) {
	for _, seed := range []string{
		"https://mirror.example.com",
		"http://mirror.example.com:5000",
		"//mirror.example.com",
		"mirror.example.com",
		"mirror.example.com:5000",
		"127.0.0.1:5000",
		"[::1]:5000",
		"",
		"bad host",
		"https://bad host",
		"mirror.example.com/v2",
		"mirror.example.com?x=1",
		"user@mirror.example.com",
		"mirror.example.com:abc",
		"ftp://mirror.example.com",
	} {
		f.Add(seed, false)
	}

	f.Fuzz(func(t *testing.T, endpoint string, tlsSkip bool) {
		scheme := "https"
		if tlsSkip {
			scheme = "http"
		}

		host, gotScheme, err := ResolveEndpoint(endpoint, "index.docker.io", scheme)
		if err != nil {
			require.ErrorIs(t, err, errInvalidEndpoint)
			assert.Empty(t, host)
			assert.Empty(t, gotScheme)

			return
		}

		assert.Contains(t, []string{"http", "https"}, gotScheme)

		if endpoint == "" {
			assert.Equal(t, "index.docker.io", host)

			return
		}

		require.NotEmpty(t, host)

		built := url.URL{Scheme: gotScheme, Host: host, Path: "/v2/"}
		parsed, parseErr := url.Parse(built.String())
		require.NoError(t, parseErr, "URL %q built from endpoint %q does not parse", built.String(), endpoint)
		assert.Equal(t, host, parsed.Host)
	})
}
