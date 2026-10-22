package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
)

// mustNewRequest creates an HTTP GET request with the test's context and fails
// the test on error.
//
// Parameters:
//   - t: Test that owns the request.
//   - url: URL to request.
//
// Returns:
//   - *http.Request: GET request for url.
func mustNewRequest(t *testing.T, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)

	return req
}

// Test_registryClient_Do verifies that the registry client returns the
// server's response, and wraps the error of a request that fails.
func Test_registryClient_Do(t *testing.T) {
	t.Parallel()

	t.Run("successful request returns response", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
			httpstub.RespondWith(http.StatusOK, "", nil),
		))

		resp, err := (&registryClient{client: server.Client()}).Do(mustNewRequest(t, server.URL()+"/v2/"))
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("request failure returns error", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.Close()

		resp, err := (&registryClient{client: server.Client()}).Do(mustNewRequest(t, server.URL()+"/v2/")) //nolint:bodyclose // The request fails, so there is no response.
		require.ErrorContains(t, err, "failed to execute HTTP request")
		assert.Nil(t, resp)
	})

	t.Run("request failure leaves the redirect target's query out of the error", func(t *testing.T) {
		t.Parallel()

		target := httpstub.New(t)
		target.Close()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.RespondWith(http.StatusTemporaryRedirect, "",
			http.Header{"Location": {target.URL() + "/blob?sig=secret"}}))

		resp, err := (&registryClient{client: server.Client()}).Do(mustNewRequest(t, server.URL()+"/v2/")) //nolint:bodyclose // The request fails, so there is no response.
		require.ErrorContains(t, err, target.URL()+"/blob")
		assert.NotContains(t, err.Error(), "sig=secret")
		assert.Nil(t, resp)
	})

	t.Run("unparseable redirect leaves the raw Location out of the error", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.RespondWith(http.StatusTemporaryRedirect, "",
			http.Header{"Location": {"https://cdn.example.com/%zz?sig=secret"}}))

		resp, err := (&registryClient{client: server.Client()}).Do(mustNewRequest(t, server.URL()+"/v2/")) //nolint:bodyclose // The request fails, so there is no response.
		require.ErrorIs(t, err, errUnparseableRedirect)
		assert.NotContains(t, err.Error(), "sig=secret")
		assert.Nil(t, resp)
	})

	t.Run("canceled request keeps its cause", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL()+"/v2/", nil)
		require.NoError(t, err)

		resp, err := (&registryClient{client: server.Client()}).Do(req) //nolint:bodyclose // The request fails, so there is no response.
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, resp)
	})
}

// TestRedactURL verifies that URLs for logs and errors lose their query and
// user information, which can hold signed parameters or credentials.
func TestRedactURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://cdn.example.com/blob", RedactURL("https://cdn.example.com/blob?sig=secret&exp=1"))
	assert.Equal(t, "https://cdn.example.com/blob", RedactURL("https://user:secret@cdn.example.com/blob?sig=secret"))
	assert.Equal(t, "https://registry.example.com/v2/a", RedactURL("https://registry.example.com/v2/a"))
	assert.Equal(t, "<unparseable>", RedactURL("http://[::1"))
}

// Test_configureTLS verifies that configureTLS applies the minimum TLS version
// from the settings, and falls back to TLS 1.2 with a warning for a version it
// does not accept.
func Test_configureTLS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tlsConfig      *tls.Config
		minVersion     string
		wantMin        uint16
		wantWarnSubstr string
	}{
		{
			name: "existing config retains min version when no override configured",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
			},
			minVersion: "",
			wantMin:    tls.VersionTLS10,
		},
		{
			name: "valid override sets min version to TLS 1.2",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
			},
			minVersion: "TLS1.2",
			wantMin:    tls.VersionTLS12,
		},
		{
			name: "invalid override defaults to TLS 1.2 and warns",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS10,
			},
			minVersion:     "TLS1.0",
			wantMin:        tls.VersionTLS12,
			wantWarnSubstr: "Invalid WATCHTOWER_REGISTRY_TLS_MIN_VERSION",
		},
		{
			name: "unknown override lowers a higher version to TLS 1.2 and warns",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
			minVersion:     "TLS9.9",
			wantMin:        tls.VersionTLS12,
			wantWarnSubstr: "Invalid WATCHTOWER_REGISTRY_TLS_MIN_VERSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logBuf bytes.Buffer

			log := zerolog.New(&logBuf).Level(zerolog.WarnLevel)

			configureTLS(&log, tt.tlsConfig, registryConfig.Registry{TLSMinVersion: tt.minVersion})

			if tt.tlsConfig != nil {
				assert.Equal(t, tt.wantMin, tt.tlsConfig.MinVersion)
			}

			if tt.wantWarnSubstr != "" {
				assert.Contains(t, logBuf.String(), tt.wantWarnSubstr)
			} else {
				assert.Empty(t, logBuf.String())
			}
		})
	}
}

// Test_configureTLS_TLSSkip verifies that configureTLS disables certificate
// verification only when the settings skip TLS verification.
func Test_configureTLS_TLSSkip(t *testing.T) {
	t.Parallel()

	t.Run("enabled with non-nil logger emits debug", func(t *testing.T) {
		t.Parallel()

		var logBuf bytes.Buffer

		log := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		configureTLS(&log, tlsConfig, registryConfig.Registry{TLSSkip: true})

		assert.True(t, tlsConfig.InsecureSkipVerify)
		assert.Contains(t, logBuf.String(), "TLS certificate verification disabled via WATCHTOWER_REGISTRY_TLS_SKIP")
	})

	t.Run("enabled with nil logger does not panic", func(t *testing.T) {
		t.Parallel()

		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		assert.NotPanics(t, func() {
			configureTLS(nil, tlsConfig, registryConfig.Registry{TLSSkip: true})
		})
		assert.True(t, tlsConfig.InsecureSkipVerify)
	})

	t.Run("disabled leaves verification enabled", func(t *testing.T) {
		t.Parallel()

		var logBuf bytes.Buffer

		log := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		configureTLS(&log, tlsConfig, registryConfig.Registry{})

		assert.False(t, tlsConfig.InsecureSkipVerify)
		assert.NotContains(t, logBuf.String(), "TLS certificate verification disabled")
	})
}

// Test_buildRegistryTransport verifies that the registry transport uses the
// TLS configuration it is given.
func Test_buildRegistryTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tlsConfig *tls.Config
		wantNil   bool
	}{
		{
			name:      "returns non-nil transport with nil TLS config",
			tlsConfig: nil,
			wantNil:   false,
		},
		{
			name: "returns non-nil transport with TLS config",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildRegistryTransport(tt.tlsConfig)
			if tt.wantNil {
				assert.Nil(t, got)

				return
			}

			require.NotNil(t, got)

			if tt.tlsConfig != nil {
				assert.NotNil(t, got.TLSClientConfig)
			}
		})
	}
}

// Test_buildRegistryClient verifies that the registry client has a transport
// for any TLS configuration.
func Test_buildRegistryClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tlsConfig *tls.Config
		wantNil   bool
	}{
		{
			name:      "returns non-nil client with nil TLS config",
			tlsConfig: nil,
			wantNil:   false,
		},
		{
			name: "returns non-nil client with TLS config",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildRegistryClient(tt.tlsConfig)
			if tt.wantNil {
				assert.Nil(t, got)

				return
			}

			require.NotNil(t, got)
			assert.NotNil(t, got.Transport)
		})
	}
}

// TestNewAuthClient verifies that NewAuthClient shares one client between calls
// with the same TLS settings, and builds each client from the settings it is
// given rather than from those of an earlier call, including when it is called
// concurrently.
func TestNewAuthClient(t *testing.T) {
	t.Parallel()

	transportTLS := func(t *testing.T, client Client) *tls.Config {
		t.Helper()

		registry, ok := client.(*registryClient)
		require.True(t, ok, "unexpected client type %T", client)

		transport, ok := registry.client.Transport.(*http.Transport)
		require.True(t, ok, "unexpected transport type %T", registry.client.Transport)
		require.NotNil(t, transport.TLSClientConfig)

		return transport.TLSClientConfig
	}

	t.Run("same settings share a client", func(t *testing.T) {
		t.Parallel()

		settings := registryConfig.Registry{TLSMinVersion: "TLS1.2"}

		first := NewAuthClient(nil, settings)
		second := NewAuthClient(nil, settings)

		assert.Same(t, first, second)
	})

	t.Run("each client follows its settings", func(t *testing.T) {
		t.Parallel()

		verified := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{}))
		skipped := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{TLSSkip: true}))
		strict := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "TLS1.3"}))

		assert.False(t, verified.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS12), verified.MinVersion)
		assert.True(t, skipped.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS13), strict.MinVersion)
	})

	t.Run("settings that resolve to the same TLS version share a client", func(t *testing.T) {
		t.Parallel()

		tls12 := NewAuthClient(nil, registryConfig.Registry{})

		for _, name := range []string{"TLS1.2", "tls1.2", "TLS9.9"} {
			assert.Same(t, tls12, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: name}), name)
		}

		tls13 := NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "TLS1.3"})

		assert.Same(t, tls13, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "tls1.3"}))
		assert.NotSame(t, tls12, tls13)
	})

	t.Run("cooldown platform shares the TLS client", func(t *testing.T) {
		t.Parallel()

		platform := registryConfig.Registry{CooldownPlatform: registryConfig.Platform{OS: "windows"}}

		assert.Same(t, NewAuthClient(nil, registryConfig.Registry{}), NewAuthClient(nil, platform))
	})

	t.Run("concurrent calls share a client", func(t *testing.T) {
		t.Parallel()

		const callers = 100

		clients := make([]Client, callers)

		var wg sync.WaitGroup

		for i := range callers {
			wg.Go(func() {
				clients[i] = NewAuthClient(nil, registryConfig.Registry{})
			})
		}

		wg.Wait()

		for i := 1; i < callers; i++ {
			assert.Same(t, clients[0], clients[i])
		}
	})

	t.Run("shared client makes requests", func(t *testing.T) {
		t.Parallel()

		server := httpstub.New(t)
		server.AppendHandlers(httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/get")),
			httpstub.RespondWith(http.StatusOK, "", nil),
		))

		resp, err := NewAuthClient(nil, registryConfig.Registry{}).Do(mustNewRequest(t, server.URL()+"/get"))
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestScheme verifies that registry requests use plain HTTP only when the
// settings skip TLS verification.
func TestScheme(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https", Scheme(registryConfig.Registry{}))
	assert.Equal(t, "http", Scheme(registryConfig.Registry{TLSSkip: true}))
}
