package git

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestApplyAuth(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()

		(&Client{opts: Options{Token: "tok"}}).applyAuth(nil, "")
	})

	t.Run("token is bearer", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Token: "tok", Username: "user", Password: "pw"}}).applyAuth(req, "git.example.com")
		assert.Equal(t, "Bearer tok", req.Header.Get("Authorization"))
	})

	t.Run("basic auth", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Username: "octocat", Password: "pw"}}).applyAuth(req, "git.example.com")
		user, pass, ok := req.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "octocat", user)
		assert.Equal(t, "pw", pass)
	})

	t.Run("username only", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Username: "octocat"}}).applyAuth(req, "git.example.com")
		user, pass, ok := req.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "octocat", user)
		assert.Empty(t, pass)
	})

	t.Run("no credentials", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com", nil)
		require.NoError(t, err)

		(&Client{}).applyAuth(req, "git.example.com")
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("foreign host gets no token", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://evil.example/api", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Token: "secret", Username: "user", Password: "pw"}}).applyAuth(req, "git.example.com")
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("foreign gitea host gets no token", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://evil.example/api/v1/version", nil)
		require.NoError(t, err)

		(&Client{opts: Options{
			Token: "secret",
			Hosts: map[string]string{"git.example.com": types.GitHostGitea},
		}}).applyAuth(req, "git.example.com")
		assert.Empty(t, req.Header.Get("Authorization"))
	})

	t.Run("github api host", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.github.com/repos/org/app", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Token: "secret"}}).applyAuth(req, "github.com")
		assert.Equal(t, "Bearer secret", req.Header.Get("Authorization"))
	})

	t.Run("http gets no token", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://git.example.com/api", nil)
		require.NoError(t, err)

		(&Client{opts: Options{Token: "secret"}}).applyAuth(req, "git.example.com")
		assert.Empty(t, req.Header.Get("Authorization"))
	})
}

// TestApplyAuthProviderSchemes verifies that each provider receives its expected token authorization scheme.
func TestApplyAuthProviderSchemes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cloneHost string
		path      string
		hosts     map[string]string
		want      string
	}{
		{name: "github", cloneHost: "github.com", path: "/api", want: "Bearer tok"},
		{name: "gitlab", cloneHost: "gitlab.com", path: "/api", want: "Bearer tok"},
		{name: "gitea", cloneHost: "codeberg.org", path: "/api", want: "token tok"},
		{
			name:      "self-hosted gitea",
			cloneHost: "git.example.com",
			path:      "/api",
			hosts:     map[string]string{"git.example.com": types.GitHostGitea},
			want:      "token tok",
		},
		{name: "gitea api path", cloneHost: "git.example.com", path: "/api/v1/version", want: "token tok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+tt.cloneHost+tt.path, nil)
			require.NoError(t, err)

			(&Client{opts: Options{Token: "tok", Hosts: tt.hosts}}).applyAuth(req, tt.cloneHost)
			assert.Equal(t, tt.want, req.Header.Get("Authorization"))
		})
	}
}

func TestOriginOf(t *testing.T) {
	t.Parallel()

	t.Run("nil client defaults to https host", func(t *testing.T) {
		t.Parallel()

		var client *Client

		got := client.originOf("Git.Example.COM")
		assert.Equal(t, "https", got.Scheme)
		assert.Equal(t, "git.example.com", got.Host)
	})

	t.Run("stored origin", func(t *testing.T) {
		t.Parallel()

		stored := url.URL{Scheme: "https", Host: "git.example.com:8443", Path: "/gitlab"}
		client := &Client{origins: map[string]url.URL{"git.example.com": stored}}
		assert.Equal(t, stored, client.originOf("[Git.Example.COM]"))
	})

	t.Run("ipv6 default", func(t *testing.T) {
		t.Parallel()

		got := (&Client{}).originOf("2001:db8::1")
		assert.Equal(t, "https", got.Scheme)
		assert.Equal(t, "[2001:db8::1]", got.Host)
	})
}

func TestHTTPHost(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "git.example.com", httpHost("git.example.com"))
	assert.Equal(t, "[2001:db8::1]", httpHost("2001:db8::1"))
}

func TestGithubAPI(t *testing.T) {
	t.Parallel()

	t.Run("github.com uses api.github.com", func(t *testing.T) {
		t.Parallel()

		got := (&Client{}).githubAPIAt(url.URL{}, "GitHub.com", "repos", "org", "app")
		assert.Equal(t, "https://api.github.com/repos/org/app", got.String())
	})

	t.Run("enterprise uses origin", func(t *testing.T) {
		t.Parallel()

		client := &Client{origins: map[string]url.URL{
			"github.company.com": {Scheme: "https", Host: "github.company.com", Path: "/ghe"},
		}}
		got := client.githubAPIAt(url.URL{}, "github.company.com", "repos", "org", "app")
		assert.Equal(t, "https://github.company.com/ghe/api/v3/repos/org/app", got.String())
	})

	t.Run("enterprise default origin", func(t *testing.T) {
		t.Parallel()

		got := (&Client{}).githubAPIAt(url.URL{}, "github.company.com", "user")
		assert.Equal(t, "https://github.company.com/api/v3/user", got.String())
	})
}

func TestGitlabAPI(t *testing.T) {
	t.Parallel()

	client := &Client{origins: map[string]url.URL{
		"gitlab.internal": {Scheme: "https", Host: "gitlab.internal:8443", Path: "/gitlab"},
	}}
	assert.Equal(t, "https://gitlab.internal:8443/gitlab/api/v4/projects",
		client.gitlabAPIAt(url.URL{}, "gitlab.internal", "projects").String())
	assert.Equal(t, "https://gitlab.com/api/v4/version",
		(&Client{}).gitlabAPIAt(url.URL{}, "gitlab.com", "version").String())
}

func TestGiteaAPI(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://git.example.com/api/v1/version",
		(&Client{}).giteaAPIAt(url.URL{}, "git.example.com", "version").String())
	assert.Equal(t, "https://codeberg.org/api/v1/repos/org/app",
		(&Client{}).giteaAPIAt(url.URL{}, "codeberg.org", "repos", "org", "app").String())
}

func TestRejectCrossHostRedirect(t *testing.T) {
	t.Parallel()

	same, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com/b", nil)
	require.NoError(t, err)
	prev, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://git.example.com/a", nil)
	require.NoError(t, err)
	require.NoError(t, rejectCrossHostRedirect(same, []*http.Request{prev}))

	other, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://evil.example/b", nil)
	require.NoError(t, err)
	require.ErrorIs(t, rejectCrossHostRedirect(other, []*http.Request{prev}), errCrossHostRedirect)

	downgrade, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://git.example.com/b", nil)
	require.NoError(t, err)
	require.ErrorIs(t, rejectCrossHostRedirect(downgrade, []*http.Request{prev}), errCrossHostRedirect)
}

func TestNewGitHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("default transport", func(t *testing.T) {
		t.Parallel()

		got := newGitHTTPClient(5*time.Second, false, nil)
		assert.Equal(t, 5*time.Second, got.Timeout)
		assert.Nil(t, got.Transport)
	})

	t.Run("insecure skip verify", func(t *testing.T) {
		t.Parallel()

		got := newGitHTTPClient(time.Second, true, nil)
		tr, ok := got.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, tr.TLSClientConfig)
		assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS12), tr.TLSClientConfig.MinVersion)
	})

	t.Run("ca bundle", func(t *testing.T) {
		t.Parallel()

		got := newGitHTTPClient(time.Second, false, []byte("not-a-real-pem"))
		tr, ok := got.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, tr.TLSClientConfig)
		assert.Nil(t, tr.TLSClientConfig.RootCAs)
		assert.False(t, tr.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("ca bundle augments system roots", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		t.Cleanup(server.Close)
		bundle := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})

		got := newGitHTTPClient(time.Second, false, bundle)
		tr, ok := got.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, tr.TLSClientConfig)
		require.NotNil(t, tr.TLSClientConfig.RootCAs)

		systemPool, systemErr := x509.SystemCertPool()

		var expected *x509.CertPool
		if systemErr == nil && systemPool != nil {
			expected = systemPool
		} else {
			expected = x509.NewCertPool()
		}

		require.True(t, expected.AppendCertsFromPEM(bundle))
		assert.True(t, expected.Equal(tr.TLSClientConfig.RootCAs))
	})

	t.Run("insecure TLS with invalid ca bundle", func(t *testing.T) {
		t.Parallel()

		got := newGitHTTPClient(time.Second, true, []byte("not-a-real-pem"))
		tr, ok := got.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, tr.TLSClientConfig)
		assert.Nil(t, tr.TLSClientConfig.RootCAs)
		assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
	})
}

func TestToHostname(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "git.example.com", toHostname("Git.Example.COM"))
	assert.Equal(t, "2001:db8::1", toHostname("[2001:db8::1]"))
	assert.Equal(t, "git.example.com", toHostname("[git.example.com]"))
	assert.Empty(t, toHostname(""))
}

// TestSanitizeRepository verifies that repository sanitization removes credentials and details from supported remote formats.
func TestSanitizeRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty"},
		{name: "absolute local path", raw: "/srv/git/app.git", want: "/srv/git/app.git"},
		{name: "relative local path", raw: "../repos/app.git", want: "../repos/app.git"},
		{name: "local path query", raw: "../repos/app.git?token=secret", want: "../repos/app.git"},
		{name: "windows local path", raw: `C:\repos\app.git`, want: `C:\repos\app.git`},
		{name: "file URL", raw: "file:///srv/git/app.git?token=secret#fragment", want: "file:///srv/git/app.git"},
		{
			name: "HTTPS URL",
			raw:  "https://user:password-secret@github.com/org/app.git?token=query-secret#fragment-secret",
			want: "https://github.com/org/app.git",
		},
		{
			name: "SSH URL",
			raw:  "ssh://git@gitlab.example.com/group/app.git?token=query-secret#fragment-secret",
			want: "ssh://gitlab.example.com/group/app.git",
		},
		{
			name: "SCP URL",
			raw:  "git@github.com:org/app.git?token=query-secret#fragment-secret",
			want: "git@github.com:org/app.git",
		},
		{name: "malformed URL", raw: "https://user:secret@%zz/repo.git"},
		{name: "unsafe SCP userinfo", raw: "token@github.com:org/app.git"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sanitizeRepository(tt.raw))
		})
	}
}

// TestSanitizeRepositoryMessage verifies that repository messages redact secrets and retain a safe repository identifier.
func TestSanitizeRepositoryMessage(t *testing.T) {
	t.Parallel()

	repo := "git@github.com:org/app.git?token=query-secret#fragment-secret"
	got := sanitizeRepositoryMessage(repo, "request failed for "+repo+" with query-secret and fragment-secret")

	assert.Contains(t, got, "git@github.com:org/app.git")
	assert.NotContains(t, got, "query-secret")
	assert.NotContains(t, got, "fragment-secret")

	t.Run("does not redact the sanitized repository", func(t *testing.T) {
		t.Parallel()

		raw := "https://user:token@token.example/org/app.git?query=secret"
		got := sanitizeRepositoryMessage(raw, "request failed for "+raw)

		assert.Contains(t, got, "https://token.example/org/app.git")
		assert.NotContains(t, got, "user:token")
		assert.NotContains(t, got, "query=secret")
	})

	t.Run("does not treat a username without a password as a secret", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, repositorySecrets("https://user@git.example.com/org/app.git"))
	})
}

func TestJoinEscaped(t *testing.T) {
	t.Parallel()

	t.Run("nil base", func(t *testing.T) {
		t.Parallel()

		got := joinEscaped(nil, "projects", "group/app")
		assert.Empty(t, got.String())
	})

	t.Run("escapes slashes in segments", func(t *testing.T) {
		t.Parallel()

		base := &url.URL{Scheme: "https", Host: "gitlab.com", Path: "/api/v4"}
		got := joinEscaped(base, "projects", "group/app", "repository", "commits", "main")
		assert.Equal(t, "https://gitlab.com/api/v4/projects/group%2Fapp/repository/commits/main", got.String())
	})

	t.Run("trims trailing slash on base", func(t *testing.T) {
		t.Parallel()

		base := &url.URL{Scheme: "https", Host: "gitlab.com", Path: "/api/v4/"}
		got := joinEscaped(base, "version")
		assert.Equal(t, "https://gitlab.com/api/v4/version", got.String())
	})

	t.Run("falls back to join path when parse fails", func(t *testing.T) {
		t.Parallel()

		base := &url.URL{Scheme: ":", Host: "x.com"}
		got := joinEscaped(base, "projects", "group/app")
		assert.NotNil(t, got)
		assert.Contains(t, got.Path, "projects")
	})
}
