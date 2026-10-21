package container

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/internal/testutil/httpstub"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestCooldownError(t *testing.T) {
	t.Parallel()

	t.Run("Error()", func(t *testing.T) {
		t.Parallel()

		t.Run("the wrapped error is non-nil", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the wrapped error message", func(t *testing.T) {
				t.Parallel()

				inner := errors.New("inner failure")
				e := &CooldownError{Age: "1h", Delay: "2h", err: inner}
				assert.Equal(t, "inner failure", e.Error())
			})
		})

		t.Run("the wrapped error is nil", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the ErrImageCooldown message", func(t *testing.T) {
				t.Parallel()

				e := &CooldownError{Age: "1h", Delay: "2h", Remaining: "1h", err: nil}
				assert.Equal(t, ErrImageCooldown.Error(), e.Error())
			})
		})
	})

	t.Run("Unwrap()", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the wrapped error when non-nil", func(t *testing.T) {
			t.Parallel()

			inner := errors.New("wrapped")
			e := &CooldownError{err: inner}
			assert.Equal(t, inner, e.Unwrap())
		})

		t.Run("returns nil when no wrapped error", func(t *testing.T) {
			t.Parallel()

			e := &CooldownError{}
			require.NoError(t, e.Unwrap())
		})
	})

	t.Run("Is()", func(t *testing.T) {
		t.Parallel()

		t.Run("matches when wrapped error is ErrImageCooldown", func(t *testing.T) {
			t.Parallel()

			e := &CooldownError{err: ErrImageCooldown}
			assert.True(t, e.Is(ErrImageCooldown))
		})

		t.Run("does not match ErrImageCooldown with unrelated wrapped error", func(t *testing.T) {
			t.Parallel()

			e := &CooldownError{err: errors.New("some other error")}
			assert.False(t, e.Is(ErrImageCooldown))
		})

		t.Run("does not match an unrelated target", func(t *testing.T) {
			t.Parallel()

			unrelated := errors.New("unrelated")
			e := &CooldownError{err: errors.New("some other error")}
			assert.False(t, e.Is(unrelated))
		})
	})

	t.Run("errors.Is sentinel extraction", func(t *testing.T) {
		t.Parallel()

		t.Run("allows errors.Is(err, ErrImageCooldown) on *CooldownError", func(t *testing.T) {
			t.Parallel()

			err := &CooldownError{
				Age:       "2 hours",
				Delay:     "1 day",
				Remaining: "22 hours",
				err:       ErrImageCooldown,
			}
			assert.ErrorIs(t, err, ErrImageCooldown)
		})

		t.Run("allows errors.As to extract *CooldownError with all fields", func(t *testing.T) {
			t.Parallel()

			err := &CooldownError{
				Age:       "2 hours",
				Delay:     "1 day",
				Remaining: "22 hours",
				err:       ErrImageCooldown,
			}

			var cooldownErr *CooldownError
			require.ErrorAs(t, err, &cooldownErr)
			assert.Equal(t, "2 hours", cooldownErr.Age)
			assert.Equal(t, "1 day", cooldownErr.Delay)
			assert.Equal(t, "22 hours", cooldownErr.Remaining)
		})
	})
}

// notFoundRegistry starts a stub registry that answers every request with 404,
// so fetching an image's creation time from it fails.
//
// Parameters:
//   - t: Test that owns the registry.
//
// Returns:
//   - string: Host and port of the registry.
func notFoundRegistry(t *testing.T) string {
	t.Helper()

	server := httpstub.New(t)
	notFound := httpstub.RespondWith(http.StatusNotFound, "not found", nil)
	server.RouteToHandler(http.MethodGet, httpstub.PathRegexp(".*"), notFound)
	server.RouteToHandler(http.MethodHead, httpstub.PathRegexp(".*"), notFound)

	return server.Addr()
}

// Test_isOutsideCooldown verifies that the cooldown gate lets a pull go ahead
// when no cooldown is configured, the delay is zero, or the container is
// monitor-only or no-pull, and that a failure to fetch the image's creation
// time holds the pull back with a CooldownError.
func Test_isOutsideCooldown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		// when describes the configuration, and it the expected result.
		when, it string
		params   types.UpdateParams
	}{
		{when: "no cooldown delay is configured", it: "returns true with no registry calls", params: types.UpdateParams{}},
		{when: "cooldown delay is zero", it: "returns true immediately", params: types.UpdateParams{CooldownDelay: 0}},
		{
			when:   "container is monitor-only",
			it:     "returns true and bypasses the cooldown check",
			params: types.UpdateParams{MonitorOnly: true, CooldownDelay: 24 * time.Hour},
		},
		{
			when:   "container has no-pull enabled",
			it:     "returns true and bypasses the cooldown check",
			params: types.UpdateParams{NoPull: true, CooldownDelay: 24 * time.Hour},
		},
	}

	for _, tt := range tests {
		t.Run(tt.when, func(t *testing.T) {
			t.Parallel()

			t.Run(tt.it, func(t *testing.T) {
				t.Parallel()

				i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

				outside, err := i.isOutsideCooldown(t.Context(), MockContainer(WithImageName("test:latest")), tt.params)
				require.NoError(t, err)
				assert.True(t, outside)
			})
		})
	}

	t.Run("FetchImageCreationTime fails (registry manifest returns 404)", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false with a CooldownError reflecting the age fetch failure", func(t *testing.T) {
			t.Parallel()

			// The test registry serves plain HTTP.
			i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			outside, err := i.isOutsideCooldown(t.Context(), MockContainer(WithImageName(notFoundRegistry(t)+"/test:latest")), types.UpdateParams{
				CooldownDelay: 1 * time.Hour,
			})
			assert.False(t, outside)

			var cooldownErr *CooldownError
			require.ErrorAs(t, err, &cooldownErr)
		})
	})
}

// Test_isOutsideCooldown_pullOptionsFailure verifies that when the registry
// credentials cannot be read, the cooldown gate holds the pull back with a
// CooldownError for the pull options failure, before any registry request.
//
// The test sets environment variables, so it does not run in parallel.
func Test_isOutsideCooldown_pullOptionsFailure(t *testing.T) {
	t.Run("GetPullOptions fails (registry auth unavailable)", func(t *testing.T) {
		t.Run("returns false with a CooldownError wrapping the pull options error", func(t *testing.T) {
			// A Docker config that cannot be parsed, with no environment
			// credentials to fall back on, makes the credential lookup fail.
			configDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte("{not json"), 0o600))
			t.Setenv("DOCKER_CONFIG", configDir)
			t.Setenv("REPO_USER", "")
			t.Setenv("REPO_PASS", "")

			i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

			outside, err := i.isOutsideCooldown(t.Context(), MockContainer(WithImageName("registry.example.com/app:latest")), types.UpdateParams{
				CooldownDelay: 1 * time.Hour,
			})
			assert.False(t, outside)

			var cooldownErr *CooldownError
			require.ErrorAs(t, err, &cooldownErr)
			assert.NotEmpty(t, cooldownErr.Delay)
			assert.Contains(t, cooldownErr.Error(), "could not get pull options for")
		})
	})
}

// TestPullImageCooldownGate verifies that a stale image whose creation time
// cannot be fetched is held back with a CooldownError instead of being pulled.
func TestPullImageCooldownGate(t *testing.T) {
	t.Parallel()

	t.Run("image is stale and cooldown age fetch fails", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a CooldownError without pulling layers", func(t *testing.T) {
			t.Parallel()

			// An explicit registry host keeps local-only 404 handling from
			// applying, so the digest failure falls through to the cooldown gate.
			// ImagePull has no expectation, so a pull fails the test.
			image := notFoundRegistry(t) + "/test:latest"
			c := MockContainer(
				WithImageName(image),
				WithRepoDigests([]string{image + "@sha256:12345"}),
			)

			// The test registry serves plain HTTP.
			i := newTestImageClient(newImageAPI(t), newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

			err := i.PullImage(t.Context(), c, warnAuto, types.UpdateParams{CooldownDelay: 1 * time.Hour})

			var cooldownErr *CooldownError
			require.ErrorAs(t, err, &cooldownErr)
		})
	})
}

func TestCheckLocalImageCooldown(t *testing.T) {
	t.Parallel()

	t.Run("defers when the local image is younger than the cooldown", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithImageName("app:latest"))
		c.imageInfo.Created = time.Now().Add(-time.Hour).Format(time.RFC3339Nano)

		err := CheckLocalImageCooldown(c, types.UpdateParams{CooldownDelay: 24 * time.Hour})
		require.ErrorIs(t, err, ErrImageCooldown)
	})

	t.Run("allows rebuilds when the local image is older than the cooldown", func(t *testing.T) {
		t.Parallel()

		c := MockContainer(WithImageName("app:latest"))
		c.imageInfo.Created = time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)

		err := CheckLocalImageCooldown(c, types.UpdateParams{CooldownDelay: 24 * time.Hour})
		require.NoError(t, err)
	})
}

// TestIsOutsideCooldown_RegistrySettings verifies that the cooldown check
// fetches the image creation time with the image client's registry settings,
// so a registry served over plain HTTP is reachable only when the settings
// skip TLS.
func TestIsOutsideCooldown_RegistrySettings(t *testing.T) {
	t.Parallel()

	const configDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	created := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case "/v2/org/app/manifests/1.0":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			fmt.Fprintf(w, `{
				"schemaVersion": 2,
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"config": {"mediaType": "application/vnd.docker.container.image.v1+json", "digest": %q, "size": 2}
			}`, configDigest)
		case "/v2/org/app/blobs/" + configDigest:
			fmt.Fprintf(w, `{"created": %q}`, created)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	container := MockContainer(WithImageName(strings.TrimPrefix(server.URL, "http://") + "/org/app:1.0"))
	params := types.UpdateParams{CooldownDelay: time.Hour}

	plain := newImageClient(nil, newTestLogger(t), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

	outside, err := plain.isOutsideCooldown(t.Context(), container, params)
	require.NoError(t, err)
	assert.True(t, outside, "an image older than the cooldown was deferred")

	secure := newImageClient(nil, newTestLogger(t), registryConfig.Registry{}, ratelimit.New())

	outside, err = secure.isOutsideCooldown(t.Context(), container, params)
	require.Error(t, err, "an HTTPS request reached the plain HTTP registry")
	assert.False(t, outside)
}

// TestEvalImageAge verifies when an image is eligible for an update under a
// cooldown, including a creation time in the future, which counts as a new
// image and becomes eligible a cooldown after its creation time.
func TestEvalImageAge(t *testing.T) {
	t.Parallel()

	const delay = 24 * time.Hour

	tests := []struct {
		name string
		// created is the creation time relative to now.
		created time.Duration
		// wantEligibleIn is how long until the image is eligible. Zero means eligible now.
		wantEligibleIn time.Duration
	}{
		{name: "older than the cooldown", created: -48 * time.Hour},
		{name: "within the cooldown", created: -time.Hour, wantEligibleIn: 23 * time.Hour},
		{name: "a few minutes in the future", created: 5 * time.Minute, wantEligibleIn: delay + 5*time.Minute},
		{name: "far in the future", created: 365 * 24 * time.Hour, wantEligibleIn: delay + 365*24*time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				eligible, err := evalImageAge(time.Now().Add(tt.created), delay, nopLog())

				if tt.wantEligibleIn == 0 {
					require.NoError(t, err)
					assert.True(t, eligible)

					return
				}

				assert.False(t, eligible)
				require.ErrorIs(t, err, ErrImageCooldown)

				cooldownErr, ok := errors.AsType[*CooldownError](err)
				require.True(t, ok)
				assert.Equal(t, time.Now().Add(tt.wantEligibleIn), cooldownErr.EligibleAt)
			})
		})
	}
}

// TestCheckLocalImageCooldown_FutureCreationTime verifies that a local image
// whose creation time is in the future is held back by the cooldown.
func TestCheckLocalImageCooldown_FutureCreationTime(t *testing.T) {
	t.Parallel()

	c := MockContainer(WithImageName("app:latest"))
	c.imageInfo.Created = time.Now().Add(time.Hour).Format(time.RFC3339Nano)

	err := CheckLocalImageCooldown(c, types.UpdateParams{CooldownDelay: 24 * time.Hour})
	require.ErrorIs(t, err, ErrImageCooldown)
}
