package container

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerRegistry "github.com/moby/moby/api/types/registry"
	dockerSystem "github.com/moby/moby/api/types/system"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
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
			c := imageClient{log: newTestLogger(t)}
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

// mirrorInfoResult returns a Docker Info result that lists one registry mirror.
//
// Returns:
//   - dockerClient.SystemInfoResult: Info with a single mirror.
func mirrorInfoResult() dockerClient.SystemInfoResult {
	return dockerClient.SystemInfoResult{
		Info: dockerSystem.Info{
			RegistryConfig: &dockerRegistry.ServiceConfig{
				Mirrors: []string{"https://mirror.example.com"},
			},
		},
	}
}

// Test_newImageClient_nilLogger_noMirrors verifies resolveRegistryMirrorConfig does not
// panic when the image client logger is nil and Docker reports no registry mirrors.
func Test_newImageClient_nilLogger_noMirrors(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(dockerClient.SystemInfoResult{}, nil).
		Once()

	c := newImageClient(api, nil, registryConfig.Registry{}, ratelimit.New())

	assert.NotPanics(t, func() {
		assert.Nil(t, c.resolveRegistryMirrorConfig(context.Background()))
	})
}

// TestResolveRegistryMirrorConfig_CachesInfo verifies that image clients share
// one successful Docker Info result instead of each asking the daemon.
func TestResolveRegistryMirrorConfig_CachesInfo(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(mirrorInfoResult(), nil).
		Once()

	first := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())
	second := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

	gotFirst := first.resolveRegistryMirrorConfig(context.Background())
	gotSecond := second.resolveRegistryMirrorConfig(context.Background())

	require.NotNil(t, gotFirst)
	require.NotNil(t, gotSecond)
	assert.Equal(t, gotFirst.RegistryConfig.Mirrors, gotSecond.RegistryConfig.Mirrors)
}

// TestResolveRegistryMirrorConfig_InfoErrorNotCached verifies that a failed
// Docker Info call is not cached, so the next resolution asks the daemon again.
func TestResolveRegistryMirrorConfig_InfoErrorNotCached(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(dockerClient.SystemInfoResult{}, errors.New("daemon unavailable")).
		Times(2)

	client := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

	assert.Nil(t, client.resolveRegistryMirrorConfig(context.Background()))
	assert.Nil(t, client.resolveRegistryMirrorConfig(context.Background()))
}

// TestResolveRegistryMirrorConfig_NilDaemonInfoUsesShared verifies that an image
// client built without a cache uses the process-wide one.
func TestResolveRegistryMirrorConfig_NilDaemonInfoUsesShared(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
		Return(mirrorInfoResult(), nil).
		Once()

	client := imageClient{api: api, log: newTestLogger(t)}

	got := client.resolveRegistryMirrorConfig(context.Background())
	require.NotNil(t, got)

	sharedDaemonInfoCache.mu.Lock()
	defer sharedDaemonInfoCache.mu.Unlock()

	assert.True(t, sharedDaemonInfoCache.fetched)
}

// TestResolveRegistryMirrorConfig_CoalescesConcurrentFetches verifies that a
// caller arriving while Docker Info is in flight waits for that fetch instead
// of starting its own.
func TestResolveRegistryMirrorConfig_CoalescesConcurrentFetches(t *testing.T) {
	resetDaemonInfoCache()
	t.Cleanup(resetDaemonInfoCache)

	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})

		api := mockContainer.NewMockAPIClient(t)
		api.EXPECT().Info(mock.Anything, dockerClient.InfoOptions{}).
			RunAndReturn(func(context.Context, dockerClient.InfoOptions) (dockerClient.SystemInfoResult, error) {
				<-release

				return mirrorInfoResult(), nil
			}).
			Once()

		firstClient := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())
		secondClient := newImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

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

		// One caller is blocked in Info and the other on the in-flight fetch.
		synctest.Wait()
		close(release)
		wg.Wait()

		require.NotNil(t, first)
		require.NotNil(t, next)
	})
}
