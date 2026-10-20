package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	mockAuth "github.com/nicholas-fedor/watchtower/pkg/registry/auth/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// maxFuzzTokenResponseSize is the largest token response body that
// readBearerTokenWithExpiry reads.
const maxFuzzTokenResponseSize = 1 << 20

// expectedToken returns the token that readBearerTokenWithExpiry reads from a
// token response body, or the error it must wrap for the body.
//
// Parameters:
//   - body: Token response body.
//
// Returns:
//   - string: The token, or the access token when the token is empty.
//   - error: Sentinel the error must wrap, nil when the body is a JSON token response.
func expectedToken(body []byte) (string, error) {
	if len(body) > maxFuzzTokenResponseSize {
		return "", errFailedDecodeResponse
	}

	var response types.TokenResponse

	err := json.Unmarshal(body, &response)
	if err != nil {
		return "", errFailedUnmarshalBearerResponse
	}

	if response.Token != "" {
		return response.Token, nil
	}

	return response.AccessToken, nil
}

// FuzzGetBearerToken checks that a bearer token fetched from a token response
// is "Bearer " followed by the response's token, or its access_token when the
// token is empty, and that a response that is not a JSON token response, or
// that has neither token, is an error.
//
// The seeds cover valid responses, malformed JSON, missing fields, a token of
// another type, null, an empty body, and a long token.
func FuzzGetBearerToken(f *testing.F) {
	for _, seed := range []string{
		`{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"}`,
		`{"access_token":"fallback"}`,
		`{"token":""}`,
		`{"token":null}`,
		`{"token":"test"`,
		`invalid json`,
		`{"token":}`,
		`{"token": "test",}`,
		`{}`,
		`{"other":"field"}`,
		`[]`,
		`{"token":123}`,
		`{"token":true}`,
		`{"token":{"nested":"object"}}`,
		`{"token":["array"]}`,
		`null`,
		``,
		`{"token":"very long token string that might cause issues if not handled properly in some parsers but json should handle it fine"}`,
	} {
		f.Add([]byte(seed))
	}

	imageRef, err := reference.ParseNormalizedNamed("test/image")
	require.NoError(f, err)

	f.Fuzz(func(t *testing.T, body []byte) {
		// Each input starts without a cached token for the fixed realm.
		initTokenCache(newNopLogger())
		tokenCache.InvalidateAll()

		client := mockAuth.NewMockClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
		}).Once()

		challenge := `bearer realm="https://test.com/token",service="test.com"`

		got, err := getBearerToken(newNopLogger(), t.Context(), challenge, imageRef, "", client, registryConfig.Registry{})

		want, wantErr := expectedToken(body)
		if wantErr == nil && want == "" {
			wantErr = errFailedDecodeResponse
		}

		if wantErr != nil {
			require.ErrorIs(t, err, wantErr)
			assert.Empty(t, got)

			return
		}

		require.NoError(t, err)
		assert.Equal(t, "Bearer "+want, got)
	})
}

// FuzzTransformAuth checks that TransformAuth turns base64 JSON credentials,
// in standard or URL-safe encoding, with a password or identity token into a
// base64 "username:password" credential, and returns any other input as it is.
//
// The seeds cover credentials with a username and password, with only a
// username or a password, an empty object, truncated JSON, input that is not
// base64, base64 text that is not JSON, an empty string, and a null username.
func FuzzTransformAuth(f *testing.F) {
	for _, seed := range []string{
		"eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzcyJ9", // {"username":"testuser","password":"testpass"}
		"eyJ1c2VybmFtZSI6InVzZXIifQ==",                                 // {"username":"user"}
		"eyJwYXNzd29yZCI6InBhc3MifQ==",                                 // {"password":"pass"}
		"e30=",                                                         // {}
		"eyJ1c2VybmFtZSI6InVzZXIi",                                     // {"username":"user"
		"notbase64",
		"aGVsbG8=", // hello
		"",
		"dGVzdA==", // test
		"eyJ1c2VybmFtZSI6bnVsbCwgcGFzc3dvcmQiOiJwYXNzIn0=", // {"username":null, "password":"pass"}
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		want := input

		decoded, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(input)
		}

		var credentials types.RegistryCredentials
		if input != "" && err == nil && json.Unmarshal(decoded, &credentials) == nil {
			password := credentials.Password
			if password == "" {
				password = credentials.IdentityToken
			}

			if password != "" {
				want = base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + password))
			}
		}

		assert.Equal(t, want, TransformAuth(newNopLogger(), input))
	})
}

// FuzzTransformAuthRoundTrip checks that credentials Docker encodes as base64
// JSON, in standard or URL-safe encoding, come back from TransformAuth as
// "username:password" when they have a password, and unchanged otherwise.
//
// The seeds cover a username and password, a password alone, a username
// alone, empty credentials, and characters that differ between the encodings.
func FuzzTransformAuthRoundTrip(f *testing.F) {
	f.Add("user", "pass", false)
	f.Add("", "token", true)
	f.Add("user", "", false)
	f.Add("", "", true)
	f.Add("u:s>e?r", "p??a>ss", true)

	f.Fuzz(func(t *testing.T, username, password string, urlEncoding bool) {
		// JSON carries each invalid UTF-8 byte as a replacement character, so
		// such credentials do not come back as they were.
		if !utf8.ValidString(username) || !utf8.ValidString(password) {
			t.Skip("credentials are not valid UTF-8")
		}

		raw, err := json.Marshal(types.RegistryCredentials{Username: username, Password: password})
		require.NoError(t, err)

		encoding := base64.StdEncoding
		if urlEncoding {
			encoding = base64.URLEncoding
		}

		encoded := encoding.EncodeToString(raw)

		got := TransformAuth(newNopLogger(), encoded)
		if password == "" {
			assert.Equal(t, encoded, got)

			return
		}

		decoded, err := base64.StdEncoding.DecodeString(got)
		require.NoError(t, err)
		assert.Equal(t, username+":"+password, string(decoded))
	})
}

// FuzzGetAuthURL checks that a token URL built from a challenge is an HTTP or
// HTTPS URL with a host name, a service, and the image's pull scope, and that
// any challenge it rejects is an invalid challenge or realm error.
//
// The seeds cover valid challenges for several registries, challenges missing
// the realm or service, empty and non-bearer headers, a trailing comma, a
// wrong separator, a valueless key, an empty scope, an uppercase scheme, extra
// fields, a local registry, and a realm with no host name.
func FuzzGetAuthURL(f *testing.F) {
	for _, seed := range []string{
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull"`,
		`bearer realm="https://registry.example.com/token",service="registry.example.com",scope="repository:user/repo:pull"`,
		`bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/alpine:pull"`,
		`bearer realm="https://ghcr.io/token"`,
		`bearer service="ghcr.io"`,
		`bearer scope="repository:test/image:pull"`,
		``,
		`basic realm="test"`,
		`bearer`,
		`bearer realm=""`,
		`bearer service=""`,
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull",`,
		`bearer realm="https://ghcr.io/token" service="ghcr.io"`,
		`bearer realm="https://ghcr.io/token",service="ghcr.io",invalidkey`,
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope=`,
		`BEARER realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull"`,
		`bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:test/image:pull",extra="value"`,
		`bearer realm="http://localhost:5000/token",service="localhost:5000",scope="repository:test/image:pull"`,
		`BeArer reAlm=http://:`,
	} {
		f.Add(seed)
	}

	imageRef, err := reference.ParseNormalizedNamed("ghcr.io/test/image")
	require.NoError(f, err)

	f.Fuzz(func(t *testing.T, challenge string) {
		got, err := getAuthURL(newNopLogger(), challenge, imageRef, "", registryConfig.Registry{})
		if err != nil {
			assert.Nil(t, got)

			if !errors.Is(err, errInvalidChallengeHeader) {
				require.ErrorIs(t, err, errInvalidRealmURL)
			}

			return
		}

		require.NotNil(t, got)
		assert.Contains(t, []string{"http", "https"}, got.Scheme)
		assert.NotEmpty(t, got.Hostname(), "token URL %q names no host", got)

		query := got.Query()
		assert.NotEmpty(t, query.Get("service"))
		assert.Equal(t, "repository:test/image:pull", query.Get("scope"))

		_, err = url.Parse(got.String())
		require.NoError(t, err)
	})
}

// FuzzComputeTokenExpiry checks that a token expires expires_in seconds, or
// the default TTL when expires_in is not positive, after its issued_at time,
// or after the current time when issued_at is missing or not RFC 3339, and
// never before that time, with an expires_in too long for a duration capped
// at the longest whole number of seconds a duration holds.
//
// The seeds cover a positive, zero, negative, and overflowing expires_in, with
// and without a valid issued_at, and an issued_at that is not a time.
func FuzzComputeTokenExpiry(f *testing.F) {
	f.Add(3600, "")
	f.Add(0, "")
	f.Add(-1, "")
	f.Add(0, "2024-01-01T00:00:00Z")
	f.Add(3600, "2024-01-01T00:00:00Z")
	f.Add(60, "not-a-date")
	f.Add(math.MaxInt, "")
	f.Add(math.MaxInt, "2024-01-01T00:00:00Z")

	f.Fuzz(func(t *testing.T, expiresIn int, issuedAt string) {
		const maxSeconds = int64(math.MaxInt64) / int64(time.Second)

		ttl := defaultTokenTTL
		if expiresIn > 0 {
			ttl = time.Duration(min(int64(expiresIn), maxSeconds)) * time.Second
		}

		before := time.Now()
		got := computeTokenExpiry(newNopLogger(), &types.TokenResponse{ExpiresIn: expiresIn, IssuedAt: issuedAt})
		after := time.Now()

		issued, err := time.Parse(time.RFC3339, issuedAt)
		if issuedAt != "" && err == nil {
			assert.False(t, got.Before(issued), "expiry %v is before issued_at %v", got, issued)
			assert.True(t, got.Equal(issued.Add(ttl)), "expiry %v, want %v", got, issued.Add(ttl))

			return
		}

		assert.False(t, got.Before(before), "expiry %v is before the request at %v", got, before)
		assert.False(t, got.Before(before.Add(ttl)), "expiry %v is before %v", got, before.Add(ttl))
		assert.False(t, got.After(after.Add(ttl)), "expiry %v is after %v", got, after.Add(ttl))
	})
}

// FuzzReadBearerTokenWithExpiry checks that the token read from a response body
// is its token, or its access_token when the token is empty, with an expiry,
// and that a body that is not a JSON token response is an error.
//
// The seeds cover a token with expires_in, an empty and a null token, an empty
// object, an array, input that is not JSON, and an empty body.
func FuzzReadBearerTokenWithExpiry(f *testing.F) {
	for _, seed := range []string{
		`{"token":"test","expires_in":3600}`,
		`{"access_token":"fallback"}`,
		`{"token":""}`,
		`{"token":null}`,
		`{}`,
		`[]`,
		`invalid`,
		``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, body []byte) {
		got, expiresAt, err := readBearerTokenWithExpiry(newNopLogger(), bytes.NewReader(body), "test/image")

		want, wantErr := expectedToken(body)
		if wantErr != nil {
			require.ErrorIs(t, err, wantErr)
			assert.Empty(t, got)
			assert.True(t, expiresAt.IsZero())

			return
		}

		require.NoError(t, err)
		assert.Equal(t, want, got)
		assert.False(t, expiresAt.IsZero())
	})
}

// FuzzParseChallenge checks that the realm, service, and scope parsed from a
// header are unquoted parts of it, and that a header that is not a bearer
// challenge yields none.
//
// The seeds cover a bearer challenge in lower and upper case, an empty header,
// a bare scheme, an empty realm, and a Basic challenge.
func FuzzParseChallenge(f *testing.F) {
	f.Add(`bearer realm="https://ghcr.io/token",service="ghcr.io"`)
	f.Add(`BEARER realm="https://ghcr.io/token"`)
	f.Add(``)
	f.Add(`bearer`)
	f.Add(`bearer realm=""`)
	f.Add(`basic realm="test"`)

	f.Fuzz(func(t *testing.T, header string) {
		// The parser reads the header as runes, so invalid UTF-8 comes back as
		// replacement characters rather than as part of the header.
		if !utf8.ValidString(header) {
			t.Skip("header is not valid UTF-8")
		}

		got := parseChallenge(header)

		scheme := strings.ToLower(strings.TrimSpace(header))
		if scheme != "bearer" && !strings.HasPrefix(scheme, "bearer ") && !strings.HasPrefix(scheme, "bearer\t") {
			assert.Equal(t, challengeValues{}, got)

			return
		}

		for _, value := range []string{got.realm, got.service, got.scope} {
			assert.Contains(t, header, value)
			assert.False(t, strings.HasPrefix(value, `"`) || strings.HasSuffix(value, `"`), "value %q keeps a quote", value)
		}
	})
}

// FuzzExtractChallengeHost checks that a host is extracted only from an HTTP or
// HTTPS realm URL that names a host, and that it has no path, user info, query,
// or fragment.
//
// The seeds cover HTTPS and HTTP realms, a realm with a port, a realm with a
// port but no host name, a realm without a scheme, and an empty realm.
func FuzzExtractChallengeHost(f *testing.F) {
	f.Add("https://ghcr.io/token")
	f.Add("http://localhost:5000/token")
	f.Add("ghcr.io/token")
	f.Add("")
	f.Add("https://registry.example.com:5000/token")
	f.Add("http://:5000/token")

	f.Fuzz(func(t *testing.T, realm string) {
		got := extractChallengeHost(newNopLogger(), realm)
		if got == "" {
			return
		}

		scheme := strings.ToLower(strings.TrimSpace(realm))
		assert.True(t, strings.HasPrefix(scheme, "http://") || strings.HasPrefix(scheme, "https://"), "host %q from realm %q", got, realm)
		assert.False(t, strings.ContainsAny(got, "/@?#"), "host %q from realm %q", got, realm)

		parsed, err := url.Parse(strings.TrimSpace(realm))
		require.NoError(t, err)
		assert.NotEmpty(t, parsed.Hostname(), "host %q from realm %q names no host", got, realm)
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
