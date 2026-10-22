package git

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestClassifyHosts(t *testing.T) {
	t.Parallel()

	t.Run("skips empty hostname", func(t *testing.T) {
		t.Parallel()

		client := newDetectClient(t, http.NewServeMux())
		got := client.ClassifyHosts(t.Context(), []url.URL{{Scheme: "https", Path: "/gitlab"}})
		assert.Empty(t, got)
		assert.Empty(t, client.origins)
	})

	t.Run("stores origin and skips built-in hosts", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := &Client{log: &nop, origins: nil}

		origin := httpsOrigin("github.com")
		got := client.ClassifyHosts(t.Context(), []url.URL{origin})
		assert.NotContains(t, got, "github.com")
		require.Contains(t, client.origins, "github.com")
		assert.Equal(t, origin, client.origins["github.com"])
	})

	t.Run("keeps existing host mapping", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Github-Request-Id", "abc")
			w.WriteHeader(http.StatusOK)
		})

		client := newDetectClient(t, mux)
		client.opts.Hosts = map[string]string{
			"git.example.com": types.GitHostGitea,
			"other.example":   types.GitHostGitLab,
		}

		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("git.example.com")})
		assert.Equal(t, types.GitHostGitea, got["git.example.com"])
		assert.Equal(t, types.GitHostGitLab, got["other.example"])
	})

	t.Run("unclassified when no fingerprint matches", func(t *testing.T) {
		t.Parallel()

		client := newDetectClient(t, http.NewServeMux())
		got := client.ClassifyHosts(t.Context(), []url.URL{httpsOrigin("git.unknown.example")})
		assert.Empty(t, got["git.unknown.example"])
		assert.Equal(t, got, client.opts.Hosts)
	})
}

func TestDetectKind(t *testing.T) {
	t.Parallel()

	t.Run("gitea version", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		})

		client := newDetectClient(t, mux)
		assert.Equal(t, types.GitHostGitea, client.detectKind(t.Context(), httpsOrigin("git.example.com"), ""))
	})

	t.Run("gitlab metadata", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/metadata", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"16.8.1","revision":"abc","enterprise":false}`))
		})

		client := newDetectClient(t, mux)
		assert.Equal(t, types.GitHostGitLab, client.detectKind(t.Context(), httpsOrigin("gitlab.internal"), ""))
	})

	t.Run("gitlab version path after metadata 404", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v4/version", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"16.8.1","revision":"cbe7d4e7"}`))
		})

		client := newDetectClient(t, mux)
		assert.Equal(t, types.GitHostGitLab, client.detectKind(t.Context(), httpsOrigin("gitlab.internal"), ""))
	})

	t.Run("github headers", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Github-Request-Id", "abc123")
			w.WriteHeader(http.StatusOK)
		})

		client := newDetectClient(t, mux)
		assert.Equal(t, types.GitHostGitHub, client.detectKind(t.Context(), httpsOrigin("github.company.com"), ""))
	})

	t.Run("401 with matching body still classifies", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		})

		client := newDetectClient(t, mux)
		assert.Equal(t, types.GitHostGitea, client.detectKind(t.Context(), httpsOrigin("git.example.com"), ""))
	})

	t.Run("unusable status is skipped", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		})

		client := newDetectClient(t, mux)
		assert.Empty(t, client.detectKind(t.Context(), httpsOrigin("git.example.com"), ""))
	})

	t.Run("probe error continues", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})}

		assert.Empty(t, client.detectKind(t.Context(), httpsOrigin("git.example.com"), ""))
	})

	t.Run("cancelled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := newDetectClient(t, http.NewServeMux())
		assert.Empty(t, client.detectKind(ctx, httpsOrigin("git.example.com"), ""))
	})
}

func TestProbeAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil endpoint", func(t *testing.T) {
		t.Parallel()

		_, err := (&Client{}).probeAPI(t.Context(), nil, "")
		require.ErrorIs(t, err, errEmptyProbeURL)
	})

	t.Run("empty host", func(t *testing.T) {
		t.Parallel()

		_, err := (&Client{}).probeAPI(t.Context(), &url.URL{Scheme: "https", Path: "/api/v1/version"}, "")
		require.ErrorIs(t, err, errEmptyProbeURL)
	})

	t.Run("returns status headers and body", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Empty(t, r.Header.Get("Authorization"))
			w.Header().Set("X-Test", "ok")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		}))
		t.Cleanup(srv.Close)

		nop := zerolog.Nop()
		client := New(&nop, Options{Token: "tok"})
		client.http = srv.Client()

		endpoint, err := url.Parse(srv.URL + "/api/v1/version")
		require.NoError(t, err)

		got, err := client.probeAPI(t.Context(), endpoint, "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, got.Status)
		assert.Equal(t, "ok", got.Header.Get("X-Test"))
		assert.JSONEq(t, `{"version":"1.22.0"}`, string(got.Body))
	})

	t.Run("http do error", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		})}

		endpoint, err := url.Parse("https://git.example.com/api/v1/version")
		require.NoError(t, err)

		_, err = client.probeAPI(t.Context(), endpoint, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "http get:")
	})

	t.Run("body read error", func(t *testing.T) {
		t.Parallel()

		nop := zerolog.Nop()
		client := New(&nop, Options{})
		client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(errReader{}),
			}, nil
		})}

		endpoint, err := url.Parse("https://git.example.com/api/v1/version")
		require.NoError(t, err)

		_, err = client.probeAPI(t.Context(), endpoint, "")
		require.Error(t, err)
		assert.ErrorContains(t, err, "read body:")
	})
}

func TestProbeUsable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "ok", status: http.StatusOK, want: true},
		{name: "unauthorized", status: http.StatusUnauthorized, want: true},
		{name: "forbidden", status: http.StatusForbidden, want: true},
		{name: "not found", status: http.StatusNotFound},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "zero", status: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, probeUsable(tt.status))
		})
	}
}

func TestMatchGiteaAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "version", body: `{"version":"1.22.0"}`, want: true},
		{name: "forgejo version", body: `{"version":"8.0.0+gitea-1.22.0"}`, want: true},
		{name: "empty version", body: `{"version":""}`},
		{name: "empty object", body: `{}`},
		{name: "invalid json", body: `not-json`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, matchGiteaAPI(apiProbe{Body: []byte(tt.body)}))
		})
	}
}

func TestMatchGitLabAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "metadata", body: `{"version":"16.8.1","revision":"abc","enterprise":false}`, want: true},
		{name: "version and revision", body: `{"version":"16.8.1","revision":"cbe7d4e7"}`, want: true},
		{name: "version and enterprise", body: `{"version":"16.8.1","enterprise":true}`, want: true},
		{name: "version only", body: `{"version":"16.8.1"}`},
		{name: "revision only", body: `{"revision":"abc"}`},
		{name: "invalid json", body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, matchGitLabAPI(apiProbe{Body: []byte(tt.body)}))
		})
	}
}

func TestMatchGitHubAPI(t *testing.T) {
	t.Parallel()

	t.Run("request id header", func(t *testing.T) {
		t.Parallel()

		header := make(http.Header)
		header.Set("X-Github-Request-Id", "abc123")
		assert.True(t, matchGitHubAPI(apiProbe{Header: header}))
	})

	t.Run("enterprise version header", func(t *testing.T) {
		t.Parallel()

		header := make(http.Header)
		header.Set("X-Github-Enterprise-Version", "3.12.0")
		assert.True(t, matchGitHubAPI(apiProbe{Header: header}))
	})

	t.Run("current_user_url body", func(t *testing.T) {
		t.Parallel()

		assert.True(t, matchGitHubAPI(apiProbe{
			Body: []byte(`{"current_user_url":"https://github.company.com/api/v3/user"}`),
		}))
	})

	t.Run("empty body", func(t *testing.T) {
		t.Parallel()

		assert.False(t, matchGitHubAPI(apiProbe{Header: make(http.Header), Body: []byte(`{}`)}))
	})

	t.Run("invalid json without headers", func(t *testing.T) {
		t.Parallel()

		assert.False(t, matchGitHubAPI(apiProbe{Header: make(http.Header), Body: []byte(`not-json`)}))
	})
}

// newDetectClient returns a client that rewrites probe hosts to srv.
//
// Parameters:
//   - t: Test handle.
//   - handler: HTTP handler served by httptest.
//
// Returns:
//   - *Client: Client whose HTTP transport targets the test server.
func newDetectClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	nop := zerolog.Nop()
	client := New(&nop, Options{})
	client.http = rewriteHostClient(srv)

	return client
}

// errReader fails on the first Read.
type errReader struct{}

// Read implements io.Reader and always fails.
//
// Parameters:
//   - p: Unused destination buffer.
//
// Returns:
//   - int: Zero bytes.
//   - error: Persistent read failure.
func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read fail")
}
