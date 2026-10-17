package container

import (
	"context"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onsi/gomega/ghttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerRegistry "github.com/moby/moby/api/types/registry"
	dockerSystem "github.com/moby/moby/api/types/system"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
)

func Test_imageClient_buildMirrorEndpoints(t *testing.T) {
	hubMirrors := &dockerSystem.Info{
		RegistryConfig: &dockerRegistry.ServiceConfig{
			Mirrors: []string{"https://mirror.example.com"},
		},
	}

	tests := []struct {
		name     string
		info     *dockerSystem.Info
		imageRef string
		want     []string
	}{
		{
			name: "nil info returns nil",
			info: nil,
			want: nil,
		},
		{
			name: "nil RegistryConfig returns nil",
			info: &dockerSystem.Info{},
			want: nil,
		},
		{
			name: "no mirrors configured returns nil",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{},
				},
			},
			imageRef: "nginx:latest",
			want:     nil,
		},
		{
			name: "global mirrors applied to docker hub image",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"https://mirror.example.com"},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name:     "docker.io image uses hub mirrors",
			info:     hubMirrors,
			imageRef: "docker.io/library/nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name:     "index.docker.io image uses hub mirrors",
			info:     hubMirrors,
			imageRef: "index.docker.io/library/nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name: "ghcr image ignores hub mirrors",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"https://global-mirror.example.com"},
				},
			},
			imageRef: "ghcr.io/org/app:latest",
			want:     nil,
		},
		{
			name:     "lscr image ignores hub mirrors",
			info:     hubMirrors,
			imageRef: "lscr.io/linuxserver/radarr:latest",
			want:     nil,
		},
		{
			name:     "quay image ignores hub mirrors",
			info:     hubMirrors,
			imageRef: "quay.io/org/app:latest",
			want:     nil,
		},
		{
			name: "multiple mirrors tried in order",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{
						"https://primary-mirror.example.com",
						"https://backup-mirror.example.com",
					},
				},
			},
			imageRef: "nginx:latest",
			want: []string{
				"https://primary-mirror.example.com",
				"https://backup-mirror.example.com",
				"",
			},
		},
		{
			name: "whitespace-only mirrors are skipped",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"  ", "https://mirror.example.com", "   "},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name: "empty mirrors are skipped",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"", "https://mirror.example.com", ""},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name: "canonical host always appended as final fallback",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"https://mirror.example.com"},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://mirror.example.com", ""},
		},
		{
			name: "mirror URL with path and query is kept verbatim",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"https://mirror.example.com/v2/?foo=bar#baz"},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://mirror.example.com/v2/?foo=bar#baz", ""},
		},
		{
			name: "ipv6 mirror address is supported",
			info: &dockerSystem.Info{
				RegistryConfig: &dockerRegistry.ServiceConfig{
					Mirrors: []string{"https://[2001:db8::1]:5000"},
				},
			},
			imageRef: "nginx:latest",
			want:     []string{"https://[2001:db8::1]:5000", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := imageClient{log: testLog()}
			got := c.buildMirrorEndpoints(tt.info, tt.imageRef)
			assert.Equal(t, tt.want, got)
		})
	}
}

// resetDaemonInfoCache clears the process-wide Docker Info cache, so each
// test counts its own Info() calls.
func resetDaemonInfoCache() {
	sharedDaemonInfoCache.mu.Lock()
	defer sharedDaemonInfoCache.mu.Unlock()

	sharedDaemonInfoCache.info = nil
	sharedDaemonInfoCache.expires = time.Time{}
	sharedDaemonInfoCache.fetched = false
	sharedDaemonInfoCache.inflight = nil
}

// Test_newImageClient_nilLogger_noMirrors verifies resolveRegistryMirrorConfig does not
// panic when the image client logger is nil and Docker reports no registry mirrors.
func Test_newImageClient_nilLogger_noMirrors(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	server := ghttp.NewServer()
	t.Cleanup(server.Close)

	var infoPath string

	server.AppendHandlers(APIVersionPingHandler())
	server.RouteToHandler("GET", regexp.MustCompile(`^/v\d+(\.\d+)*/info$`), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))

		if r != nil {
			infoPath = r.URL.Path
		}
	})

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.HTTPTestServer.Client()),
	)
	require.NoError(t, err)

	c := newImageClient(api, nil, registryConfig.Registry{})

	assert.NotPanics(t, func() {
		got := c.resolveRegistryMirrorConfig(context.Background())
		assert.Nil(t, got)
	})

	assert.Regexp(t, `^/v\d+(\.\d+)*/info$`, infoPath, "resolveRegistryMirrorConfig must call the versioned /info endpoint")
}

func TestResolveRegistryMirrorConfig_CachesInfo(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	server := ghttp.NewServer()
	t.Cleanup(server.Close)

	var infoCalls int

	server.AppendHandlers(APIVersionPingHandler())
	server.RouteToHandler("GET", regexp.MustCompile(`^/v\d+(\.\d+)*/info$`), func(w http.ResponseWriter, _ *http.Request) {
		infoCalls++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"RegistryConfig":{"Mirrors":["https://mirror.example.com"]}}`))
	})

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.HTTPTestServer.Client()),
	)
	require.NoError(t, err)

	first := newImageClient(api, testLog(), registryConfig.Registry{})
	second := newImageClient(api, testLog(), registryConfig.Registry{})

	gotFirst := first.resolveRegistryMirrorConfig(context.Background())
	gotSecond := second.resolveRegistryMirrorConfig(context.Background())

	require.NotNil(t, gotFirst)
	require.NotNil(t, gotSecond)
	assert.Equal(t, 1, infoCalls)
	assert.Equal(t, gotFirst.RegistryConfig.Mirrors, gotSecond.RegistryConfig.Mirrors)
}

func TestResolveRegistryMirrorConfig_InfoErrorNotCached(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	server := ghttp.NewServer()
	t.Cleanup(server.Close)

	var infoCalls int

	server.AppendHandlers(APIVersionPingHandler())
	server.RouteToHandler("GET", regexp.MustCompile(`^/v\d+(\.\d+)*/info$`), func(w http.ResponseWriter, _ *http.Request) {
		infoCalls++

		w.WriteHeader(http.StatusInternalServerError)
	})

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.HTTPTestServer.Client()),
	)
	require.NoError(t, err)

	client := newImageClient(api, testLog(), registryConfig.Registry{})

	assert.Nil(t, client.resolveRegistryMirrorConfig(context.Background()))
	assert.Nil(t, client.resolveRegistryMirrorConfig(context.Background()))
	assert.Equal(t, 2, infoCalls)
}

func TestResolveRegistryMirrorConfig_NilDaemonInfoUsesShared(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	server := ghttp.NewServer()
	t.Cleanup(server.Close)

	var infoCalls int

	server.AppendHandlers(APIVersionPingHandler())
	server.RouteToHandler("GET", regexp.MustCompile(`^/v\d+(\.\d+)*/info$`), func(w http.ResponseWriter, _ *http.Request) {
		infoCalls++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"RegistryConfig":{"Mirrors":["https://mirror.example.com"]}}`))
	})

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.HTTPTestServer.Client()),
	)
	require.NoError(t, err)

	client := imageClient{api: api, log: testLog()}

	got := client.resolveRegistryMirrorConfig(context.Background())
	require.NotNil(t, got)
	assert.Equal(t, 1, infoCalls)
}

func TestResolveRegistryMirrorConfig_CoalescesConcurrentFetches(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	server := ghttp.NewServer()
	t.Cleanup(server.Close)

	started := make(chan struct{})
	release := make(chan struct{})

	var infoCalls atomic.Int32

	server.AppendHandlers(APIVersionPingHandler())
	server.RouteToHandler("GET", regexp.MustCompile(`^/v\d+(\.\d+)*/info$`), func(w http.ResponseWriter, _ *http.Request) {
		if infoCalls.Add(1) == 1 {
			close(started)
			<-release
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"RegistryConfig":{"Mirrors":["https://mirror.example.com"]}}`))
	})

	api, err := dockerClient.New(
		dockerClient.WithHost(server.URL()),
		dockerClient.WithHTTPClient(server.HTTPTestServer.Client()),
	)
	require.NoError(t, err)

	firstClient := newImageClient(api, testLog(), registryConfig.Registry{})
	secondClient := newImageClient(api, testLog(), registryConfig.Registry{})

	var (
		wg          sync.WaitGroup
		first, next *dockerSystem.Info
	)

	wg.Go(func() {
		first = firstClient.resolveRegistryMirrorConfig(context.Background())
	})
	wg.Go(func() {
		next = secondClient.resolveRegistryMirrorConfig(context.Background())
	})

	<-started
	require.Eventually(t, func() bool {
		sharedDaemonInfoCache.mu.Lock()
		defer sharedDaemonInfoCache.mu.Unlock()

		return sharedDaemonInfoCache.inflight != nil
	}, time.Second, 5*time.Millisecond)
	// Let the second caller park on the in-flight fetch before unblocking Info().
	time.Sleep(20 * time.Millisecond)

	close(release)
	wg.Wait()

	require.NotNil(t, first)
	require.NotNil(t, next)
	assert.Equal(t, int32(1), infoCalls.Load())
}
