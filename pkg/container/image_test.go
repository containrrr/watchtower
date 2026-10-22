package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cerrdefs "github.com/containerd/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/digest"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// rateLimitedMessage is the error a daemon reports for a pull that a registry
// rate limited.
const rateLimitedMessage = "toomanyrequests: retry-after: 2h, allowed: 44000/minute"

// newTestImageClient returns an image client on api with a daemon info cache of
// its own, so tests never share a cached Info answer.
//
// Parameters:
//   - api: Docker API the client calls.
//   - log: Logger for the client.
//   - settings: Registry request settings.
//   - limiter: Registry rate limiter.
//
// Returns:
//   - imageClient: Client ready for the test.
func newTestImageClient(api dockerClient.APIClient, log *zerolog.Logger, settings registryConfig.Registry, limiter *ratelimit.Limiter) imageClient {
	client := newImageClient(api, log, settings, limiter)
	client.daemonInfo = &daemonInfoCache{}

	return client
}

// newImageAPI returns a mock Docker API whose daemon reports no registry mirrors.
//
// Parameters:
//   - t: Test that owns the mock.
//
// Returns:
//   - *mockContainer.MockAPIClient: Mock answering Info with an empty daemon configuration.
func newImageAPI(t *testing.T) *mockContainer.MockAPIClient {
	t.Helper()

	api := mockContainer.NewMockAPIClient(t)
	api.EXPECT().Info(mock.Anything, mock.Anything).Return(dockerClient.SystemInfoResult{}, nil).Maybe()

	return api
}

// pullResponse returns a mock image pull response whose progress stream ends with
// waitErr, nil for a pull that finished.
//
// Parameters:
//   - t: Test that owns the mock.
//   - waitErr: Error the progress stream ends with.
//
// Returns:
//   - *mockContainer.MockImagePullResponse: Response the pull waits on and closes once.
func pullResponse(t *testing.T, waitErr error) *mockContainer.MockImagePullResponse {
	t.Helper()

	response := mockContainer.NewMockImagePullResponse(t)
	response.EXPECT().Wait(mock.Anything).Return(waitErr).Once()
	response.EXPECT().Close().Return(nil).Once()

	return response
}

// failingRegistry starts a stub registry that answers every request with 500, so
// a digest check against it fails and the pull goes ahead.
//
// Parameters:
//   - t: Test that owns the registry.
//
// Returns:
//   - string: Host and port of the registry.
func failingRegistry(t *testing.T) string {
	t.Helper()

	server := httpstub.New(t)
	server.AllowUnhandledRequests()

	return server.Addr()
}

// digestRegistry starts a stub registry that needs no authentication and reports
// remoteDigest for the latest tag of test/image.
//
// Parameters:
//   - t: Test that owns the registry.
//   - remoteDigest: Digest the registry reports.
//
// Returns:
//   - *httpstub.Server: The registry.
func digestRegistry(t *testing.T, remoteDigest string) *httpstub.Server {
	t.Helper()

	server := httpstub.New(t)
	server.AppendHandlers(
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodGet, httpstub.Path("/v2/")),
			httpstub.RespondWith(http.StatusOK, "", nil),
		),
		httpstub.CombineHandlers(
			httpstub.VerifyRequest(t, http.MethodHead, httpstub.Path("/v2/test/image/manifests/latest")),
			httpstub.RespondWith(http.StatusOK, "", http.Header{digest.ContentDigestHeader: {remoteDigest}}),
		),
	)

	return server
}

// runningImage returns a container that runs imageName with the image ID
// imageID and the given repo digests.
//
// Parameters:
//   - imageName: Image reference the container runs.
//   - imageID: ID of the container's image.
//   - repoDigests: Repo digests of the container's image, or none to keep the default.
//
// Returns:
//   - *Container: The container.
func runningImage(imageName, imageID string, repoDigests ...string) *Container {
	options := []MockContainerUpdate{
		WithImageName(imageName),
		func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
			container.Image = imageID
			image.ID = imageID
		},
	}

	if repoDigests != nil {
		options = append(options, WithRepoDigests(repoDigests))
	}

	return MockContainer(options...)
}

// inspected returns the inspect result of an image with id and repoDigests.
//
// Parameters:
//   - id: Image ID.
//   - repoDigests: The image's repo digests.
//
// Returns:
//   - dockerClient.ImageInspectResult: The inspect result.
func inspected(id string, repoDigests ...string) dockerClient.ImageInspectResult {
	return dockerClient.ImageInspectResult{ID: id, RepoDigests: repoDigests}
}

// waitFor returns the value ch delivers within timeout, and fails the test when
// none arrives.
//
// Parameters:
//   - t: Test that receives the failure.
//   - ch: Channel to receive from.
//   - timeout: How long to wait.
//
// Returns:
//   - T: The received value.
func waitFor[T any](t *testing.T, ch <-chan T, timeout time.Duration) T {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(timeout):
		require.FailNow(t, "timed out waiting for a value", "after %s", timeout)

		var zero T

		return zero
	}
}

// TestImageClient_warnOnHeadFailed verifies when a failed HEAD request is
// reported as a warning: always, never, or, in auto mode, only for registries
// that count HEAD requests against a rate limit.
func TestImageClient_warnOnHeadFailed(t *testing.T) {
	t.Parallel()

	containerUnknown := MockContainer(WithImageName("unknown.repo/prefix/imagename:latest"))
	containerKnown := MockContainer(WithImageName("docker.io/prefix/imagename:latest"))
	c := imageClient{}

	t.Run(`warn on head failure is set to "always"`, func(t *testing.T) {
		t.Parallel()

		t.Run("should always return true", func(t *testing.T) {
			t.Parallel()

			assert.True(t, c.warnOnHeadFailed(containerUnknown, warnAlways))
			assert.True(t, c.warnOnHeadFailed(containerKnown, warnAlways))
		})
	})

	t.Run(`warn on head failure is set to "auto"`, func(t *testing.T) {
		t.Parallel()

		t.Run("should return false for unknown repos", func(t *testing.T) {
			t.Parallel()

			assert.False(t, c.warnOnHeadFailed(containerUnknown, warnAuto))
		})

		t.Run("should return true for known repos", func(t *testing.T) {
			t.Parallel()

			assert.True(t, c.warnOnHeadFailed(containerKnown, warnAuto))
		})
	})

	t.Run(`warn on head failure is set to "never"`, func(t *testing.T) {
		t.Parallel()

		t.Run("should never return true", func(t *testing.T) {
			t.Parallel()

			assert.False(t, c.warnOnHeadFailed(containerUnknown, warnNever))
			assert.False(t, c.warnOnHeadFailed(containerKnown, warnNever))
		})
	})
}

// TestImageClient_shouldSkipPull verifies that a failed digest check is logged at
// the warning strategy's level and kept out of notifications, since the pull
// that follows decides the outcome.
func TestImageClient_shouldSkipPull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy WarningStrategy
		level    string
	}{
		{name: "warning strategy always", strategy: warnAlways, level: "warn"},
		{name: "warning strategy never", strategy: warnNever, level: "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Without image info the digest check fails before any registry request.
			noImageInfo := NewContainer(nil, &dockerContainer.InspectResponse{
				ID:     "container_id",
				Name:   "kavita",
				Config: &dockerContainer.Config{Image: "ghcr.io/linuxserver/kavita:latest"},
			}, nil)

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			i := newTestImageClient(newImageAPI(t), log, registryConfig.Registry{}, ratelimit.New())

			skip, err := i.shouldSkipPull(t.Context(), noImageInfo, "", tt.strategy)
			require.NoError(t, err)
			assert.False(t, skip)
			assert.Regexp(t, fmt.Sprintf(`level=%s msg="Digest retrieval failed, falling back to full pull".* notify=no`, tt.level), logbuf.String())
		})
	}
}

// TestImageClient_PullImage verifies that a digest-pinned image is never pulled,
// that pull failures are classified as authentication, missing image, or other
// failures, and that a missing or failing registry mirror configuration falls
// back to the canonical registry.
//
// Pulls share package-level slots per registry host, so the test does not run
// in parallel.
func TestImageClient_PullImage(t *testing.T) {
	t.Run("the image consist of a pinned hash", func(t *testing.T) {
		t.Run("should gracefully fail with a useful message for bare sha256", func(t *testing.T) {
			i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())
			pinned := MockContainer(WithImageName("sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b"))

			err := i.PullImage(t.Context(), pinned, warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPinnedImage)
		})

		t.Run("should gracefully fail for repository-qualified digest", func(t *testing.T) {
			i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())
			pinned := MockContainer(WithImageName("nginx@sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b"))

			err := i.PullImage(t.Context(), pinned, warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPinnedImage)
		})
	})

	t.Run("pulling an image that requires authentication", func(t *testing.T) {
		t.Run("should log at Warn level and return errPullImageUnauthorized for auth failures", func(t *testing.T) {
			image := failingRegistry(t) + "/app:latest"

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).
				Return(nil, cerrdefs.ErrUnauthenticated.WithMessage("unauthorized: authentication required")).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			i := newTestImageClient(api, log, registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), MockContainer(WithImageName(image), WithRepoDigests([]string{image + "@sha256:abc"})), warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPullImageUnauthorized)
			require.ErrorContains(t, err, "authentication required")
			require.NotErrorIs(t, err, errPullImageFailed)
			assert.Contains(t, logbuf.String(), "level=warn")
			assert.Contains(t, logbuf.String(), "Image pull failed: authentication required")
		})
	})

	t.Run("pulling an image that does not exist in registry", func(t *testing.T) {
		t.Run("should log at Debug level and return errPullImageNotFound for not found errors", func(t *testing.T) {
			image := failingRegistry(t) + "/nonexistent:latest"

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).
				Return(nil, cerrdefs.ErrNotFound.WithMessage("manifest for "+image+" not found")).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			i := newTestImageClient(api, log, registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), MockContainer(WithImageName(image), WithRepoDigests([]string{image + "@sha256:def"})), warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPullImageNotFound)
			require.ErrorContains(t, err, "image not found")
			require.NotErrorIs(t, err, errPullImageFailed)
			assert.Contains(t, logbuf.String(), "Image pull failed: image not found in registry")
		})
	})

	t.Run("pulling an image with a server error", func(t *testing.T) {
		t.Run("should log at Debug level and return errPullImageFailed for other errors", func(t *testing.T) {
			image := failingRegistry(t) + "/app:latest"

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).
				Return(nil, errors.New("Error response from daemon: internal server error")).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			i := newTestImageClient(api, log, registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), MockContainer(WithImageName(image), WithRepoDigests([]string{image + "@sha256:ghi"})), warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPullImageFailed)
			require.ErrorContains(t, err, "failed to pull image")
			require.NotErrorIs(t, err, errPullImageUnauthorized)
			require.NotErrorIs(t, err, errPullImageNotFound)
			assert.Contains(t, logbuf.String(), "Failed to initiate image pull")
		})
	})

	t.Run("pulling an image with registry mirrors configured", func(t *testing.T) {
		t.Run("should use canonical host when mirror Info() returns no mirrors", func(t *testing.T) {
			image := failingRegistry(t) + "/app:latest"

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).
				Return(nil, cerrdefs.ErrUnauthenticated.WithMessage("unauthorized")).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), MockContainer(WithImageName(image), WithRepoDigests([]string{image + "@sha256:abc"})), warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPullImageUnauthorized)
		})

		t.Run("should handle Info() API failure gracefully and fall back to canonical", func(t *testing.T) {
			image := failingRegistry(t) + "/app:latest"

			api := mockContainer.NewMockAPIClient(t)
			api.EXPECT().Info(mock.Anything, mock.Anything).Return(dockerClient.SystemInfoResult{}, errors.New("daemon error")).Maybe()
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).
				Return(nil, cerrdefs.ErrUnauthenticated.WithMessage("unauthorized")).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), MockContainer(WithImageName(image), WithRepoDigests([]string{image + "@sha256:abc"})), warnAuto, types.UpdateParams{})
			require.ErrorIs(t, err, errPullImageUnauthorized)
		})
	})
}

// TestImageClient_performImagePull verifies that a pull waits for its progress
// stream, that a rate limit reported when the pull starts or in its stream is a
// rate-limit error, that pulls back off and retry after a short rate limit, and
// that pulls queue per registry host and credential scope, with a host in
// rate-limit cooldown holding back only its own pulls.
//
// Pulls share package-level slots per registry host and check timing, so the
// test does not run in parallel.
func TestImageClient_performImagePull(t *testing.T) {
	t.Run("draining an image pull progress stream", func(t *testing.T) {
		t.Run("waits for a successful JSON progress stream", func(t *testing.T) {
			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "registry.example.com/app:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, nil), nil).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			require.NoError(t, i.performImagePull(t.Context(), "registry.example.com/app:latest", dockerClient.ImagePullOptions{}))
		})

		t.Run("returns a rate-limit error when the stream reports toomanyrequests", func(t *testing.T) {
			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, errors.New(rateLimitedMessage)), nil).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			err := i.performImagePull(t.Context(), "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{})
			assert.True(t, ratelimit.Is(err), "unexpected error %v", err)
		})

		t.Run("returns a rate-limit error when ImagePull fails with toomanyrequests", func(t *testing.T) {
			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(nil, errors.New(rateLimitedMessage)).Once()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			err := i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{})
			assert.True(t, ratelimit.Is(err), "unexpected error %v", err)
		})

		t.Run("backs off and succeeds when the daemon reports a sub-millisecond retry-after", func(t *testing.T) {
			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(nil, errors.New("toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute")).Times(2)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, nil), nil).Once()

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			started := time.Now()

			require.NoError(t, i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}))
			// Two bucket waits of at least 100ms and 150ms separate the three pulls.
			assert.GreaterOrEqual(t, time.Since(started), 250*time.Millisecond)
		})
	})

	t.Run("the pull slot context is canceled", func(t *testing.T) {
		t.Run("does not acquire a slot", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			require.ErrorIs(t, acquirePullSlot(ctx, "ghcr.io"), context.Canceled)
		})

		t.Run("uses a distinct slot per registry host", func(t *testing.T) {
			ghcr := pullSlotFor("ghcr.io")
			hub := pullSlotFor("index.docker.io")

			assert.NotEqual(t, ghcr, hub, "ghcr.io and index.docker.io share a pull slot")
			assert.Equal(t, ghcr, pullSlotFor("ghcr.io"), "ghcr.io has more than one pull slot")
		})

		t.Run("uses distinct slots for anonymous and authenticated GHCR pulls", func(t *testing.T) {
			anon := ratelimit.Scope("ghcr.io", false)
			authed := ratelimit.Scope("ghcr.io", true)

			assert.Equal(t, "ghcr.io|anon", anon)
			assert.Equal(t, "ghcr.io", authed)
			assert.NotEqual(t, authed, anon)

			authEntered := make(chan struct{})
			releaseAuth := make(chan struct{})

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{RegistryAuth: "e30="}).
				RunAndReturn(func(ctx context.Context, _ string, _ dockerClient.ImagePullOptions) (dockerClient.ImagePullResponse, error) {
					close(authEntered)

					select {
					case <-releaseAuth:
					case <-ctx.Done():
					}

					return pullResponse(t, nil), nil
				}).Once()
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, nil), nil).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			authDone := make(chan error, 1)

			go func() {
				authDone <- i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{RegistryAuth: "e30="})
			}()

			waitFor(t, authEntered, 5*time.Second)

			anonDone := make(chan error, 1)

			go func() {
				anonDone <- i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{})
			}()

			require.NoError(t, waitFor(t, anonDone, time.Second))
			close(releaseAuth)
			require.NoError(t, waitFor(t, authDone, 5*time.Second))
		})
	})

	t.Run("a host is in rate-limit cooldown", func(t *testing.T) {
		t.Run("lets pulls to other registry hosts proceed during the wait", func(t *testing.T) {
			// An anonymous GHCR pull waits on the anonymous limiter key.
			limiter := ratelimit.New()
			limiter.Observe(ratelimit.Scope("ghcr.io", false), &ratelimit.Error{RetryAfter: 800 * time.Millisecond})

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, nil), nil).Once()
			api.EXPECT().ImagePull(mock.Anything, "registry.example.com/app:latest", dockerClient.ImagePullOptions{}).
				Return(pullResponse(t, nil), nil).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, limiter)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			blocked := make(chan error, 1)

			go func() {
				blocked <- i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{})
			}()

			time.Sleep(50 * time.Millisecond)

			started := time.Now()

			require.NoError(t, i.performImagePull(ctx, "registry.example.com/app:latest", dockerClient.ImagePullOptions{}))
			assert.Less(t, time.Since(started), 300*time.Millisecond)

			// The ghcr.io pull is still waiting out the cooldown.
			select {
			case err := <-blocked:
				require.FailNow(t, "the ghcr.io pull finished during its cooldown", "error %v", err)
			case <-time.After(300 * time.Millisecond):
			}

			require.NoError(t, waitFor(t, blocked, 3*time.Second))
		})

		t.Run("does not start a queued same-host pull during Retry-After cooldown", func(t *testing.T) {
			firstEntered := make(chan struct{})
			releaseFirst := make(chan struct{})
			secondStarted := make(chan time.Time, 1)

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				RunAndReturn(func(context.Context, string, dockerClient.ImagePullOptions) (dockerClient.ImagePullResponse, error) {
					close(firstEntered)
					<-releaseFirst

					return nil, errors.New("toomanyrequests: retry-after: 1s, allowed: 44000/minute")
				}).Once()
			// The rate limiter retries the first pull once its cooldown ends.
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{}).
				RunAndReturn(func(context.Context, string, dockerClient.ImagePullOptions) (dockerClient.ImagePullResponse, error) {
					return pullResponse(t, nil), nil
				}).Maybe()
			api.EXPECT().ImagePull(mock.Anything, "ghcr.io/linuxserver/radarr:latest", dockerClient.ImagePullOptions{}).
				RunAndReturn(func(context.Context, string, dockerClient.ImagePullOptions) (dockerClient.ImagePullResponse, error) {
					secondStarted <- time.Now()

					return pullResponse(t, nil), nil
				}).Once()

			i := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			firstErr := make(chan error, 1)

			go func() {
				firstErr <- i.performImagePull(ctx, "ghcr.io/linuxserver/nginx:latest", dockerClient.ImagePullOptions{})
			}()

			waitFor(t, firstEntered, 5*time.Second)

			secondErr := make(chan error, 1)

			go func() {
				secondErr <- i.performImagePull(ctx, "ghcr.io/linuxserver/radarr:latest", dockerClient.ImagePullOptions{})
			}()

			time.Sleep(50 * time.Millisecond)

			released := time.Now()

			close(releaseFirst)

			started := waitFor(t, secondStarted, 3*time.Second)
			assert.GreaterOrEqual(t, started.Sub(released), 800*time.Millisecond)

			waitFor(t, firstErr, 3*time.Second)
			require.NoError(t, waitFor(t, secondErr, 3*time.Second))
		})
	})
}

// TestImageClient_RemoveImageByID verifies that an image is removed with its
// untagged parents, which are logged at debug level, that removal is skipped for
// an image an active container uses or when containers cannot be listed, and
// that a missing image or a canceled context is reported.
func TestImageClient_RemoveImageByID(t *testing.T) {
	t.Parallel()

	removeOptions := dockerClient.ImageRemoveOptions{Force: true, PruneChildren: true}

	t.Run("debug logging is enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should log removed and untagged images", func(t *testing.T) {
			t.Parallel()

			imageA := randomSHA256()
			imageAParent := randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).Return(dockerClient.ContainerListResult{}, nil).Once()
			api.EXPECT().ImageRemove(mock.Anything, imageA, removeOptions).Return(dockerClient.ImageRemoveResult{Items: []dockerImage.DeleteResponse{
				{Untagged: imageA},
				{Deleted: imageA},
				{Deleted: imageAParent},
			}}, nil).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			require.NoError(t, c.RemoveImageByID(t.Context(), types.ImageID(imageA), "test-image"))

			shortA := types.ImageID(imageA).ShortID()
			shortAParent := types.ImageID(imageAParent).ShortID()

			assert.Regexp(t, fmt.Sprintf(`msg="Image removal details" deleted="%s, %s" image_id=%s image_name=test-image untagged=%s`, shortA, shortAParent, shortA, shortA), logbuf.String())
			assert.Regexp(t, fmt.Sprintf(`msg="Cleaned up old image" image_id=%s image_name=test-image`, shortA), logbuf.String())
		})
	})

	t.Run("image is not found", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error", func(t *testing.T) {
			t.Parallel()

			image := randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).Return(dockerClient.ContainerListResult{}, nil).Once()
			api.EXPECT().ImageRemove(mock.Anything, image, removeOptions).
				Return(dockerClient.ImageRemoveResult{}, cerrdefs.ErrNotFound.WithMessage("No such image")).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			err := c.RemoveImageByID(t.Context(), types.ImageID(image), "test-image")
			assert.True(t, cerrdefs.IsNotFound(err), "unexpected error %v", err)
		})
	})

	t.Run("image is used by an active container", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip removal and return ErrImageInUse for each active state", func(t *testing.T) {
			t.Parallel()

			for _, state := range []string{"running", "restarting", "paused", "created"} {
				t.Run(state, func(t *testing.T) {
					t.Parallel()

					imageA := randomSHA256()

					api := newImageAPI(t)
					api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).Return(dockerClient.ContainerListResult{
						Items: []dockerContainer.Summary{{ImageID: imageA, State: dockerContainer.ContainerState(state)}},
					}, nil).Once()

					c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

					require.ErrorIs(t, c.RemoveImageByID(t.Context(), types.ImageID(imageA), "test-image"), ErrImageInUse)
				})
			}
		})
	})

	t.Run("ContainerList API fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error and not proceed with removal", func(t *testing.T) {
			t.Parallel()

			api := newImageAPI(t)
			api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).
				Return(dockerClient.ContainerListResult{}, errors.New("Error response from daemon: Internal server error")).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			err := c.RemoveImageByID(t.Context(), types.ImageID(randomSHA256()), "test-image")
			require.ErrorContains(t, err, "cannot verify image usage")
		})
	})

	t.Run("the context is canceled", func(t *testing.T) {
		t.Parallel()

		t.Run("should return context.Canceled error without warning", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			api := newImageAPI(t)
			api.EXPECT().ContainerList(mock.Anything, dockerClient.ContainerListOptions{All: true}).
				Return(dockerClient.ContainerListResult{}, context.Canceled).Once()

			log, logbuf := newCapturedLogger(zerolog.WarnLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			require.ErrorIs(t, c.RemoveImageByID(ctx, types.ImageID(randomSHA256()), "test-image"), context.Canceled)
			assert.NotContains(t, logbuf.String(), "Failed to list containers for image usage check")
		})
	})
}

// TestImageClient_IsContainerStale_noPull verifies that, with no-pull, a container
// is stale only when its image tag now points at another local image, with the
// latest digest taken from that image's repo digests, that a failed inspection is
// an error, and that a canceled context stops the check.
func TestImageClient_IsContainerStale_noPull(t *testing.T) {
	t.Parallel()

	t.Run("no newer local image exists", func(t *testing.T) {
		t.Parallel()

		t.Run("should return false and current image ID", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(currentImageID), nil).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			stale, latestID, _, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.False(t, stale)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
			assert.Contains(t, logbuf.String(), `msg="Skipping image pull due to no-pull setting - checking local image only"`)
			assert.Contains(t, logbuf.String(), `msg="No new image found"`)
		})
	})

	t.Run("a newer local image exists", func(t *testing.T) {
		t.Parallel()

		t.Run("should return true and new image ID", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			newImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(newImageID), nil).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			stale, latestID, _, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, types.ImageID(newImageID), latestID)
			assert.Contains(t, logbuf.String(), `msg="Skipping image pull due to no-pull setting - checking local image only"`)
			assert.Contains(t, logbuf.String(), `msg="Found new image"`)
		})

		t.Run("should log the OCI version transition when labelled", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			newImage := inspected("sha256:" + randomSHA256())
			newImage.Config = &dockerspec.DockerOCIImageConfig{}
			newImage.Config.Labels = map[string]string{oci.VersionLabel: "v0.16.0"}

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(newImage, nil).Once()

			log, logbuf := newCapturedLogger(zerolog.InfoLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			running := MockContainer(
				WithImageName("test-image:latest"),
				WithImageLabels(map[string]string{oci.VersionLabel: "v0.15.0"}),
				func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
					container.Image = currentImageID
					image.ID = currentImageID
				},
			)

			stale, _, _, err := c.IsContainerStale(t.Context(), running, types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Contains(t, logbuf.String(), `msg="Found new image"`)
			assert.Contains(t, logbuf.String(), `old_version=v0.15.0`)
			assert.Contains(t, logbuf.String(), `new_version=v0.16.0`)
		})
	})

	t.Run("a newer local image exists with a repo digest", func(t *testing.T) {
		t.Parallel()

		t.Run("should return the latest digest from RepoDigests", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			newImageID := "sha256:" + randomSHA256()
			newDigest := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(newImageID, "test-image:latest@"+newDigest), nil).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, newImageID, string(latestID))
			assert.Equal(t, newDigest, latestDigest)
		})
	})

	t.Run("a newer local image exists with empty RepoDigests", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an empty latest digest", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			newImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(newImageID), nil).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, newImageID, string(latestID))
			assert.Empty(t, latestDigest)
		})
	})

	t.Run("a newer local image exists with malformed RepoDigests", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an empty latest digest when no @ separator", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			newImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(newImageID, "malformed-digest-no-at-separator"), nil).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, newImageID, string(latestID))
			assert.Empty(t, latestDigest)
		})
	})

	t.Run("image inspection fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should return an error", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
				Return(dockerClient.ImageInspectResult{}, cerrdefs.ErrNotFound.WithMessage("No such image")).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			stale, latestID, _, err := c.IsContainerStale(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.Error(t, err)
			assert.False(t, stale)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
			assert.Contains(t, logbuf.String(), `msg="Skipping image pull due to no-pull setting - checking local image only"`)
			assert.Contains(t, logbuf.String(), `msg="Failed to inspect latest image"`)
		})
	})

	t.Run("the context is canceled", func(t *testing.T) {
		t.Parallel()

		t.Run("should return context.Canceled error", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			currentImageID := "sha256:" + randomSHA256()

			// The Docker client answers a request made with a canceled context
			// with the context's error.
			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").
				RunAndReturn(func(ctx context.Context, _ string, _ ...dockerClient.ImageInspectOption) (dockerClient.ImageInspectResult, error) {
					return dockerClient.ImageInspectResult{}, ctx.Err()
				}).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			_, _, _, err := c.IsContainerStale(ctx, runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true}, "")
			require.ErrorIs(t, err, context.Canceled)
		})
	})
}

// TestImageClient_IsContainerStale_pull verifies the staleness check that pulls:
// a registry digest that differs from the running image's is pulled and the new
// image's digest reported, a matching digest skips the pull while the local tag
// is still compared, and an image without repo digests is never pulled.
//
// Pulls share package-level slots per registry host, so the test does not run
// in parallel.
func TestImageClient_IsContainerStale_pull(t *testing.T) {
	t.Run("a newer image is available with a repo digest", func(t *testing.T) {
		t.Run("should return the latest digest after pulling", func(t *testing.T) {
			localDigest := "sha256:" + randomSHA256()
			newDigest := "sha256:" + randomSHA256()
			currentImageID := "sha256:" + randomSHA256()
			newImageID := "sha256:" + randomSHA256()

			registry := digestRegistry(t, newDigest)
			image := registry.Addr() + "/test/image:latest"

			api := newImageAPI(t)
			api.EXPECT().ImagePull(mock.Anything, image, mock.Anything).Return(pullResponse(t, nil), nil).Once()
			api.EXPECT().ImageInspect(mock.Anything, image).Return(inspected(newImageID, registry.Addr()+"/test/image@"+newDigest), nil).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage(image, currentImageID, registry.Addr()+"/test/image@"+localDigest), types.UpdateParams{}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, newImageID, string(latestID))
			assert.Equal(t, newDigest, latestDigest)
		})
	})

	t.Run("the registry digest matches the running image", func(t *testing.T) {
		t.Run("should treat the container as up-to-date without pulling", func(t *testing.T) {
			localDigest := "sha256:" + randomSHA256()
			currentImageID := "sha256:" + randomSHA256()

			registry := digestRegistry(t, localDigest)
			image := registry.Addr() + "/test/image:latest"

			// The digest matches, so PullImage skips ImagePull, and HasNewImage
			// still inspects the local image by name.
			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, image).Return(inspected(currentImageID, registry.Addr()+"/test/image@"+localDigest), nil).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage(image, currentImageID, registry.Addr()+"/test/image@"+localDigest), types.UpdateParams{}, "")
			require.NoError(t, err)
			assert.False(t, stale)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
			assert.Equal(t, localDigest, latestDigest)
			assert.Contains(t, logbuf.String(), "Digest match, skipping pull")
		})
	})

	// Empty RepoDigests skips the registry pull, but HasNewImage still compares
	// the running image ID to the local tag.
	t.Run("container image has empty RepoDigests but the local tag points at a different ID", func(t *testing.T) {
		t.Run("skips pull and still reports the container as stale", func(t *testing.T) {
			currentImageID := "sha256:" + randomSHA256()
			newerLocalImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "registry.example.com/app:latest").Return(inspected(newerLocalImageID), nil).Once()

			log, logbuf := newCapturedLogger(zerolog.DebugLevel)
			c := newTestImageClient(api, log, registryConfig.Registry{}, ratelimit.New())

			stale, latestID, latestDigest, err := c.IsContainerStale(t.Context(), runningImage("registry.example.com/app:latest", currentImageID, []string{}...), types.UpdateParams{}, "")
			require.NoError(t, err)
			assert.True(t, stale)
			assert.Equal(t, newerLocalImageID, string(latestID))
			assert.Empty(t, latestDigest)
			assert.Contains(t, logbuf.String(), "empty RepoDigests")
			assert.Contains(t, logbuf.String(), "Digest match, skipping pull")
			assert.Contains(t, logbuf.String(), "Found new image")
		})
	})
}

// TestIsImagePinnedByDigest verifies which image references are pinned by a
// content digest, alone or with a repository, registry, port, or tag.
func TestIsImagePinnedByDigest(t *testing.T) {
	t.Parallel()

	// Valid 64-hex digest accepted by distribution/reference + go-digest.
	const fullDigest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	tests := []struct {
		name       string
		imageName  string
		wantPinned bool
	}{
		{name: "empty", imageName: "", wantPinned: false},
		{name: "tag only", imageName: "nginx:latest", wantPinned: false},
		{name: "untagged name (not a digest)", imageName: "nginx", wantPinned: false},
		{name: "substring sha256 in tag name", imageName: "myapp-sha256-build:1.0", wantPinned: false},
		{name: "bare content digest", imageName: fullDigest, wantPinned: true},
		{name: "repo@digest", imageName: "nginx@" + fullDigest, wantPinned: true},
		{name: "org/repo@digest", imageName: "library/nginx@" + fullDigest, wantPinned: true},
		{name: "registry/repo@digest", imageName: "registry.example.com/org/app@" + fullDigest, wantPinned: true},
		{name: "registry with port@digest", imageName: "registry.example.com:5000/app@" + fullDigest, wantPinned: true},
		{name: "tag and digest", imageName: "nginx:1.27@" + fullDigest, wantPinned: true},
		{name: "fully qualified tag and digest", imageName: "docker.io/library/nginx:latest@" + fullDigest, wantPinned: true},
		// Parse fails. String fallback treats explicit @sha256: as pinned.
		{name: "malformed empty digest still pinned", imageName: "nginx@sha256:", wantPinned: true},
		{name: "at-sha256 without algorithm still not bare pin", imageName: "nginx@deadbeef", wantPinned: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantPinned, IsImagePinnedByDigest(tt.imageName))
		})
	}
}

// TestImageClient_CheckContainerUpdate verifies the update check that does not
// pull: a pinned image or an image without repo digests has no update, no-pull
// checks only the local image, and otherwise the registry digest decides.
func TestImageClient_CheckContainerUpdate(t *testing.T) {
	t.Parallel()

	t.Run("image is pinned by digest", func(t *testing.T) {
		t.Parallel()

		for name, ref := range map[string]string{
			"reports no update for bare sha256 without contacting the registry": "sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b",
			"reports no update for repo@sha256 without contacting the registry": "nginx@sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b",
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				container := MockContainer(WithImageName(ref))
				c := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

				available, latestID, latestDigest, err := c.CheckContainerUpdate(t.Context(), container, types.UpdateParams{})
				require.NoError(t, err)
				assert.False(t, available)
				assert.Equal(t, string(container.ImageID()), string(latestID))
				assert.Empty(t, latestDigest)
			})
		}
	})

	t.Run("no-pull is enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("uses local image inspection only", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()

			api := newImageAPI(t)
			api.EXPECT().ImageInspect(mock.Anything, "test-image:latest").Return(inspected(currentImageID), nil).Once()

			c := newTestImageClient(api, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			available, latestID, _, err := c.CheckContainerUpdate(t.Context(), runningImage("test-image:latest", currentImageID), types.UpdateParams{NoPull: true})
			require.NoError(t, err)
			assert.False(t, available)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
		})
	})

	t.Run("local image has empty RepoDigests", func(t *testing.T) {
		t.Parallel()

		t.Run("treats the image as up-to-date without pulling", func(t *testing.T) {
			t.Parallel()

			currentImageID := "sha256:" + randomSHA256()
			c := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			available, latestID, latestDigest, err := c.CheckContainerUpdate(t.Context(), runningImage("local-build:latest", currentImageID, []string{}...), types.UpdateParams{})
			require.NoError(t, err)
			assert.False(t, available)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
			assert.Empty(t, latestDigest)
		})
	})

	t.Run("registry digest comparison is performed", func(t *testing.T) {
		t.Parallel()

		t.Run("reports no update when remote digest matches local RepoDigests", func(t *testing.T) {
			t.Parallel()

			localDigest := "sha256:" + randomSHA256()
			currentImageID := "sha256:" + randomSHA256()

			registry := digestRegistry(t, localDigest)
			image := registry.Addr() + "/test/image:latest"

			// The test registry serves plain HTTP.
			c := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			available, latestID, latestDigest, err := c.CheckContainerUpdate(t.Context(), runningImage(image, currentImageID, registry.Addr()+"/test/image@"+localDigest), types.UpdateParams{})
			require.NoError(t, err)
			assert.False(t, available)
			assert.Equal(t, types.ImageID(currentImageID), latestID)
			assert.Equal(t, localDigest, latestDigest)
			assert.Len(t, registry.ReceivedRequests(), 2)
		})

		t.Run("reports update available when remote digest differs from local RepoDigests", func(t *testing.T) {
			t.Parallel()

			localDigest := "sha256:" + randomSHA256()
			remoteDigest := "sha256:" + randomSHA256()
			currentImageID := "sha256:" + randomSHA256()

			registry := digestRegistry(t, remoteDigest)
			image := registry.Addr() + "/test/image:latest"

			// The test registry serves plain HTTP.
			c := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			available, latestID, latestDigest, err := c.CheckContainerUpdate(t.Context(), runningImage(image, currentImageID, registry.Addr()+"/test/image@"+localDigest), types.UpdateParams{})
			require.NoError(t, err)
			assert.True(t, available)
			assert.Equal(t, types.ImageID(""), latestID)
			assert.Equal(t, remoteDigest, latestDigest)
			assert.Len(t, registry.ReceivedRequests(), 2)
		})
	})
}

// TestExtractImageDigest verifies that the digest reported for an image comes
// from the repo digest matching the image's repository, falling back to the
// first valid repo digest, and is empty when no repo digest has one.
func TestExtractImageDigest(t *testing.T) {
	t.Parallel()

	t.Run("RepoDigests is empty", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an empty string", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, ExtractImageDigest(nil, ""))
			assert.Empty(t, ExtractImageDigest([]string{}, ""))
		})
	})

	t.Run("RepoDigests contain no @ separator", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an empty string", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, ExtractImageDigest([]string{"malformed-without-at", "sha256:abcdef"}, ""))
		})
	})

	t.Run("RepoDigests contain valid entries", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			repoDigests []string
			imageName   string
			want        string
		}{
			{
				name:        "returns the digest from the first matching entry",
				repoDigests: []string{"nginx:latest@sha256:0123456789abcdef"},
				want:        "sha256:0123456789abcdef",
			},
			{
				name:        "skips malformed entries before the first valid one",
				repoDigests: []string{"malformed-no-at", "nginx:latest@sha256:0123456789abcdef"},
				want:        "sha256:0123456789abcdef",
			},
			{
				name:        "prefers the RepoDigest matching the image name",
				repoDigests: []string{"other.io/foo@sha256:aaa", "docker.io/library/nginx@sha256:bbb"},
				imageName:   "nginx:latest",
				want:        "sha256:bbb",
			},
			{
				name:        "preserves the registry port when matching a tagged image name",
				repoDigests: []string{"registry.example.com:5000/nginx@sha256:def456", "docker.io/library/nginx@sha256:bbb"},
				imageName:   "registry.example.com:5000/nginx:latest",
				want:        "sha256:def456",
			},
			{
				name:        "matches the preferred RepoDigest when imageName is in digest form",
				repoDigests: []string{"docker.io/library/nginx@sha256:bbb", "other.io/foo@sha256:aaa"},
				imageName:   "nginx@sha256:abc123",
				want:        "sha256:bbb",
			},
			{
				name:        "falls back to the first valid digest when no name matches",
				repoDigests: []string{"other.io/foo@sha256:aaa", "other.io/bar@sha256:bbb"},
				imageName:   "nginx:latest",
				want:        "sha256:aaa",
			},
			{
				name:        "matches fully-qualified image names to short RepoDigest paths",
				repoDigests: []string{"library/nginx@sha256:ccc"},
				imageName:   "docker.io/library/nginx:latest",
				want:        "sha256:ccc",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.want, ExtractImageDigest(tt.repoDigests, tt.imageName))
			})
		}
	})
}

// TestImageClient_isOutsideCooldown_beforePull verifies that the cooldown gate
// before a pull lets the pull go ahead without registry calls when no cooldown
// is configured, and when the container is monitor-only or no-pull.
func TestImageClient_isOutsideCooldown_beforePull(t *testing.T) {
	t.Parallel()

	t.Run("no cooldown delay is configured", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true (safe to pull) with no registry calls", func(t *testing.T) {
			t.Parallel()

			i := newTestImageClient(nil, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			outside, err := i.isOutsideCooldown(t.Context(), MockContainer(WithImageName("test:latest")), types.UpdateParams{})
			require.NoError(t, err)
			assert.True(t, outside)
		})
	})

	t.Run("container is monitor-only or no-pull", func(t *testing.T) {
		t.Parallel()

		t.Run("returns true (bypasses cooldown check)", func(t *testing.T) {
			t.Parallel()

			i := newTestImageClient(nil, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			outside, err := i.isOutsideCooldown(t.Context(), MockContainer(WithImageName("test:latest")), types.UpdateParams{
				MonitorOnly:   true,
				CooldownDelay: 24 * time.Hour,
			})
			require.NoError(t, err)
			assert.True(t, outside)
		})
	})
}
