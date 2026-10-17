package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerClient "github.com/moby/moby/client"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("CooldownError", func() {
	ginkgo.Describe("Error()", func() {
		ginkgo.When("the wrapped error is non-nil", func() {
			ginkgo.It("returns the wrapped error message", func() {
				inner := errors.New("inner failure")
				e := &CooldownError{Age: "1h", Delay: "2h", err: inner}
				gomega.Expect(e.Error()).To(gomega.Equal("inner failure"))
			})
		})

		ginkgo.When("the wrapped error is nil", func() {
			ginkgo.It("returns the ErrImageCooldown message", func() {
				e := &CooldownError{Age: "1h", Delay: "2h", Remaining: "1h", err: nil}
				gomega.Expect(e.Error()).To(gomega.Equal(ErrImageCooldown.Error()))
			})
		})
	})

	ginkgo.Describe("Unwrap()", func() {
		ginkgo.It("returns the wrapped error when non-nil", func() {
			inner := errors.New("wrapped")
			e := &CooldownError{err: inner}
			gomega.Expect(e.Unwrap()).To(gomega.Equal(inner))
		})

		ginkgo.It("returns nil when no wrapped error", func() {
			e := &CooldownError{}
			gomega.Expect(e.Unwrap()).To(gomega.Succeed())
		})
	})

	ginkgo.Describe("Is()", func() {
		ginkgo.It("matches when wrapped error is ErrImageCooldown", func() {
			e := &CooldownError{err: ErrImageCooldown}
			gomega.Expect(e.Is(ErrImageCooldown)).To(gomega.BeTrue())
		})

		ginkgo.It("does not match ErrImageCooldown with unrelated wrapped error", func() {
			e := &CooldownError{err: errors.New("some other error")}
			gomega.Expect(e.Is(ErrImageCooldown)).To(gomega.BeFalse())
		})

		ginkgo.It("does not match an unrelated target", func() {
			unrelated := errors.New("unrelated")
			e := &CooldownError{err: errors.New("some other error")}
			gomega.Expect(e.Is(unrelated)).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("errors.Is sentinel extraction", func() {
		ginkgo.It("allows errors.Is(err, ErrImageCooldown) on *CooldownError", func() {
			err := &CooldownError{
				Age:       "2 hours",
				Delay:     "1 day",
				Remaining: "22 hours",
				err:       ErrImageCooldown,
			}
			gomega.Expect(errors.Is(err, ErrImageCooldown)).To(gomega.BeTrue())
		})

		ginkgo.It("allows errors.As to extract *CooldownError with all fields", func() {
			err := &CooldownError{
				Age:       "2 hours",
				Delay:     "1 day",
				Remaining: "22 hours",
				err:       ErrImageCooldown,
			}

			var cooldownErr *CooldownError
			gomega.Expect(errors.As(err, &cooldownErr)).To(gomega.BeTrue())
			gomega.Expect(cooldownErr.Age).To(gomega.Equal("2 hours"))
			gomega.Expect(cooldownErr.Delay).To(gomega.Equal("1 day"))
			gomega.Expect(cooldownErr.Remaining).To(gomega.Equal("22 hours"))
		})
	})
})

var _ = ginkgo.Describe("isOutsideCooldown", func() {
	var (
		mockClient *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		mockClient, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		gomega.Expect(err).To(gomega.Succeed())
		mockServer.AppendHandlers(APIVersionPingHandler())
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("no cooldown delay is configured", func() {
		ginkgo.It("returns true with no registry calls", func() {
			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(outside).To(gomega.BeTrue())
		})
	})

	ginkgo.When("cooldown delay is zero", func() {
		ginkgo.It("returns true immediately", func() {
			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{
					CooldownDelay: 0,
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(outside).To(gomega.BeTrue())
		})
	})

	ginkgo.When("container is monitor-only", func() {
		ginkgo.It("returns true and bypasses the cooldown check", func() {
			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{
					MonitorOnly:   true,
					CooldownDelay: 24 * time.Hour,
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(outside).To(gomega.BeTrue())
		})
	})

	ginkgo.When("container has no-pull enabled", func() {
		ginkgo.It("returns true and bypasses the cooldown check", func() {
			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{
					NoPull:        true,
					CooldownDelay: 24 * time.Hour,
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(outside).To(gomega.BeTrue())
		})
	})

	ginkgo.When("GetPullOptions fails (registry auth unavailable)", func() {
		ginkgo.It("returns false with a CooldownError wrapping the pull options error", func() {
			log, _ := captureLog(zerolog.DebugLevel)

			i := newImageClient(mockClient, log, registryConfig.Registry{}, ratelimit.New())

			imageName := "localhost/unsupported-image-format:latest"
			c := MockContainer(WithImageName(imageName))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{
					CooldownDelay: 1 * time.Hour,
				},
			)
			gomega.Expect(outside).To(gomega.BeFalse())
			gomega.Expect(err).To(gomega.HaveOccurred())

			var cooldownErr *CooldownError
			gomega.Expect(errors.As(err, &cooldownErr)).To(gomega.BeTrue())
			gomega.Expect(cooldownErr.Delay).ToNot(gomega.BeEmpty())
		})
	})

	ginkgo.When("FetchImageCreationTime fails (registry manifest returns 404)", func() {
		ginkgo.It("returns false with a CooldownError reflecting the age fetch failure", func() {
			log, _ := captureLog(zerolog.DebugLevel)

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("/v2/")),
					ghttp.RespondWith(http.StatusNotFound, "not found"),
				),
			)

			i := newImageClient(mockClient, log, registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{
					CooldownDelay: 1 * time.Hour,
				},
			)
			gomega.Expect(outside).To(gomega.BeFalse())
			gomega.Expect(err).To(gomega.HaveOccurred())

			var cooldownErr *CooldownError
			gomega.Expect(errors.As(err, &cooldownErr)).To(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("PullImage cooldown gate", func() {
	var (
		mockClient *dockerClient.Client
		mockServer *ghttp.Server
	)

	ginkgo.BeforeEach(func() {
		mockServer = ghttp.NewServer()

		var err error

		mockClient, err = dockerClient.New(
			dockerClient.WithHost(mockServer.URL()),
			dockerClient.WithHTTPClient(mockServer.HTTPTestServer.Client()),
		)
		gomega.Expect(err).To(gomega.Succeed())
		mockServer.AppendHandlers(APIVersionPingHandler())
		mockServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", gomega.MatchRegexp(`/info$`)),
				ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{}),
			),
		)
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("image is stale and cooldown age fetch fails", func() {
		ginkgo.It("returns a CooldownError without pulling layers", func() {
			log, _ := captureLog(zerolog.DebugLevel)

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", gomega.MatchRegexp("/v2/")),
					ghttp.RespondWith(http.StatusNotFound, "not found"),
				),
			)

			i := newImageClient(mockClient, log, registryConfig.Registry{}, ratelimit.New())
			// Use an explicit registry host so local-only 404 handling does not apply.
			// Digest failure must fall through to the cooldown gate.
			c := MockContainer(
				WithImageName("registry.example.com/test:latest"),
				WithRepoDigests([]string{"registry.example.com/test@sha256:12345"}),
			)

			err := i.PullImage(context.Background(), c, warnAuto, types.UpdateParams{
				CooldownDelay: 1 * time.Hour,
			})
			gomega.Expect(err).To(gomega.HaveOccurred())

			var cooldownErr *CooldownError
			gomega.Expect(errors.As(err, &cooldownErr)).To(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("CheckLocalImageCooldown", func() {
	ginkgo.It("defers when the local image is younger than the cooldown", func() {
		c := MockContainer(WithImageName("app:latest"))
		c.imageInfo.Created = time.Now().Add(-time.Hour).Format(time.RFC3339Nano)

		err := CheckLocalImageCooldown(c, types.UpdateParams{CooldownDelay: 24 * time.Hour})
		gomega.Expect(err).To(gomega.MatchError(ErrImageCooldown))
	})

	ginkgo.It("allows rebuilds when the local image is older than the cooldown", func() {
		c := MockContainer(WithImageName("app:latest"))
		c.imageInfo.Created = time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)

		err := CheckLocalImageCooldown(c, types.UpdateParams{CooldownDelay: 24 * time.Hour})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})
})

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

	plain := newImageClient(nil, testLog(), registryConfig.Registry{TLSSkip: true}, ratelimit.New())

	outside, err := plain.isOutsideCooldown(t.Context(), container, params)
	require.NoError(t, err)
	assert.True(t, outside, "an image older than the cooldown was deferred")

	secure := newImageClient(nil, testLog(), registryConfig.Registry{}, ratelimit.New())

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
