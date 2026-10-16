package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
)

func Test_registryClient_Do(t *testing.T) {
	tests := []struct {
		name       string
		client     *registryClient
		req        *http.Request
		wantBody   string
		wantStatus int
		wantErr    bool
	}{
		{
			name: "successful request returns response",
			client: &registryClient{
				client: &http.Client{
					Transport: &mockRoundTripper{
						response: &http.Response{
							StatusCode: http.StatusOK,
							Body:       http.NoBody,
						},
					},
				},
			},
			req:        mustNewRequest(t, "http://example.com/v2/"),
			wantStatus: http.StatusOK,
		},
		{
			name: "request failure returns error",
			client: &registryClient{
				client: &http.Client{
					Transport: &mockRoundTripper{
						err: assert.AnError,
					},
				},
			},
			req:     mustNewRequest(t, "http://example.com/v2/"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.client.Do(tt.req)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

// Test_configureTLS verifies that ConfigureTLS applies the minimum TLS version
// from the settings, and falls back to TLS 1.2 with a warning for a version it
// does not accept.
func Test_configureTLS(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer

			log := zerolog.New(&logBuf).Level(zerolog.WarnLevel)

			ConfigureTLS(&log, tt.tlsConfig, registryConfig.Registry{TLSMinVersion: tt.minVersion})

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

// Test_configureTLS_TLSSkip verifies that ConfigureTLS disables certificate
// verification only when the settings skip TLS verification.
func Test_configureTLS_TLSSkip(t *testing.T) {
	t.Run("enabled with non-nil logger emits debug", func(t *testing.T) {
		var logBuf bytes.Buffer

		log := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		ConfigureTLS(&log, tlsConfig, registryConfig.Registry{TLSSkip: true})

		assert.True(t, tlsConfig.InsecureSkipVerify)
		assert.Contains(t, logBuf.String(), "TLS certificate verification disabled via WATCHTOWER_REGISTRY_TLS_SKIP")
	})

	t.Run("enabled with nil logger does not panic", func(t *testing.T) {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		assert.NotPanics(t, func() {
			ConfigureTLS(nil, tlsConfig, registryConfig.Registry{TLSSkip: true})
		})
		assert.True(t, tlsConfig.InsecureSkipVerify)
	})

	t.Run("disabled leaves verification enabled", func(t *testing.T) {
		var logBuf bytes.Buffer

		log := zerolog.New(&logBuf).Level(zerolog.DebugLevel)
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}

		ConfigureTLS(&log, tlsConfig, registryConfig.Registry{})

		assert.False(t, tlsConfig.InsecureSkipVerify)
		assert.NotContains(t, logBuf.String(), "TLS certificate verification disabled")
	})
}

func Test_buildRegistryTransport(t *testing.T) {
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

func Test_buildRegistryClient(t *testing.T) {
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
// given rather than from those of an earlier call.
func TestNewAuthClient(t *testing.T) {
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
		settings := registryConfig.Registry{TLSMinVersion: "TLS1.2"}

		first := NewAuthClient(nil, settings)
		second := NewAuthClient(nil, settings)

		assert.Same(t, first, second)
	})

	t.Run("each client follows its settings", func(t *testing.T) {
		verified := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{}))
		skipped := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{TLSSkip: true}))
		strict := transportTLS(t, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "TLS1.3"}))

		assert.False(t, verified.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS12), verified.MinVersion)
		assert.True(t, skipped.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS13), strict.MinVersion)
	})

	t.Run("settings that resolve to the same TLS version share a client", func(t *testing.T) {
		tls12 := NewAuthClient(nil, registryConfig.Registry{})

		for _, name := range []string{"TLS1.2", "tls1.2", "TLS9.9"} {
			assert.Same(t, tls12, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: name}), name)
		}

		tls13 := NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "TLS1.3"})

		assert.Same(t, tls13, NewAuthClient(nil, registryConfig.Registry{TLSMinVersion: "tls1.3"}))
		assert.NotSame(t, tls12, tls13)
	})

	t.Run("cooldown platform shares the TLS client", func(t *testing.T) {
		platform := registryConfig.Registry{CooldownPlatform: registryConfig.Platform{OS: "windows"}}

		assert.Same(t, NewAuthClient(nil, registryConfig.Registry{}), NewAuthClient(nil, platform))
	})
}

// TestScheme verifies that registry requests use plain HTTP only when the
// settings skip TLS verification.
func TestScheme(t *testing.T) {
	assert.Equal(t, "https", Scheme(registryConfig.Registry{}))
	assert.Equal(t, "http", Scheme(registryConfig.Registry{TLSSkip: true}))
}

// mockRoundTripper is a test helper that implements http.RoundTripper.
type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

// mustNewRequest creates an HTTP GET request and fails the test on error.
func mustNewRequest(t *testing.T, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	require.NoError(t, err)

	return req
}
