package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	"github.com/rs/zerolog"

	cerrdefs "github.com/containerd/errdefs"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerClient "github.com/moby/moby/client"
	gomegaTypes "github.com/onsi/gomega/types"

	registryConfig "github.com/nicholas-fedor/watchtower/internal/config/registry"
	"github.com/nicholas-fedor/watchtower/internal/util"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/registry/digest"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("the client", func() {
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
		mockServer.RouteToHandler("GET", regexp.MustCompile(`/info$`), ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{}))
	})
	ginkgo.AfterEach(func() {
		mockServer.Close()
	})
	ginkgo.Describe("warnOnHeadFailed", func() {
		containerUnknown := MockContainer(WithImageName("unknown.repo/prefix/imagename:latest"))
		containerKnown := MockContainer(WithImageName("docker.io/prefix/imagename:latest"))

		ginkgo.When(`warn on head failure is set to "always"`, func() {
			c := imageClient{}

			ginkgo.It("should always return true", func() {
				gomega.Expect(c.warnOnHeadFailed(containerUnknown, warnAlways)).To(gomega.BeTrue())
				gomega.Expect(c.warnOnHeadFailed(containerKnown, warnAlways)).To(gomega.BeTrue())
			})
		})
		ginkgo.When(`warn on head failure is set to "auto"`, func() {
			c := imageClient{}

			ginkgo.It("should return false for unknown repos", func() {
				gomega.Expect(c.warnOnHeadFailed(containerUnknown, warnAuto)).To(gomega.BeFalse())
			})
			ginkgo.It("should return true for known repos", func() {
				gomega.Expect(c.warnOnHeadFailed(containerKnown, warnAuto)).To(gomega.BeTrue())
			})
		})
		ginkgo.When(`warn on head failure is set to "never"`, func() {
			c := imageClient{}

			ginkgo.It("should never return true", func() {
				gomega.Expect(c.warnOnHeadFailed(containerUnknown, warnNever)).To(gomega.BeFalse())
				gomega.Expect(c.warnOnHeadFailed(containerKnown, warnNever)).To(gomega.BeFalse())
			})
		})
	})
	ginkgo.Describe("shouldSkipPull", func() {
		// The fallback entry is logged at the strategy's level and kept out of
		// notifications, since the pull that follows decides the outcome.
		ginkgo.DescribeTable("logs a failed digest check without notifying",
			func(strategy WarningStrategy, level string) {
				// Without image info the digest check fails before any registry request.
				noImageInfo := NewContainer(nil, &dockerContainer.InspectResponse{
					ID:     "container_id",
					Name:   "kavita",
					Config: &dockerContainer.Config{Image: "ghcr.io/linuxserver/kavita:latest"},
				}, nil)

				log, logbuf := captureLog(zerolog.DebugLevel)
				i := newImageClient(mockClient, log, registryConfig.Registry{}, ratelimit.New())

				skip, err := i.shouldSkipPull(context.Background(), noImageInfo, "", strategy)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(skip).To(gomega.BeFalse())
				gomega.Eventually(logbuf).Should(gbytes.Say(
					`level=%s msg="Digest retrieval failed, falling back to full pull".* notify=no`,
					level,
				))
			},
			ginkgo.Entry("warning strategy always", warnAlways, "warn"),
			ginkgo.Entry("warning strategy never", warnNever, "debug"),
		)
	})
	ginkgo.When("pulling the latest image", func() {
		ginkgo.When("the image consist of a pinned hash", func() {
			ginkgo.It("should gracefully fail with a useful message for bare sha256", func() {
				i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
				pinnedContainer := MockContainer(
					WithImageName(
						"sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b",
					),
				)
				err := i.PullImage(context.Background(), pinnedContainer, warnAuto, types.UpdateParams{})
				gomega.Expect(err).
					To(gomega.MatchError(`image is pinned with sha256, skipping pull`))
			})
			ginkgo.It("should gracefully fail for repository-qualified digest", func() {
				i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
				pinnedContainer := MockContainer(
					WithImageName(
						"nginx@sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b",
					),
				)
				err := i.PullImage(context.Background(), pinnedContainer, warnAuto, types.UpdateParams{})
				gomega.Expect(err).
					To(gomega.MatchError(`image is pinned with sha256, skipping pull`))
			})
		})
	})
	ginkgo.When("pulling an image that requires authentication", func() {
		ginkgo.It("should log at Warn level and return errPullImageUnauthorized for auth failures", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusUnauthorized, `{"message":"unauthorized: authentication required"}`),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			pullContainer := MockContainer(
				WithImageName("private-registry.io/app:latest"),
				WithRepoDigests([]string{"private-registry.io/app@sha256:abc"}),
			)

			log, logbuf := captureLog(zerolog.DebugLevel)
			i.log = log

			err := i.PullImage(context.Background(), pullContainer, warnAuto, types.UpdateParams{})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("authentication required"))
			gomega.Expect(errors.Is(err, errPullImageUnauthorized)).To(gomega.BeTrue())
			gomega.Expect(errors.Is(err, errPullImageFailed)).To(gomega.BeFalse())
			gomega.Eventually(logbuf).Should(gbytes.Say(`level=warn`))
			gomega.Eventually(logbuf).Should(gbytes.Say(`Image pull failed: authentication required`))
		})
	})
	ginkgo.When("pulling an image that does not exist in registry", func() {
		ginkgo.It("should log at Debug level and return errPullImageNotFound for not found errors", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusNotFound, `{"message":"manifest for registry.example.com/nonexistent:latest not found"}`),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			pullContainer := MockContainer(
				WithImageName("registry.example.com/nonexistent:latest"),
				WithRepoDigests([]string{"registry.example.com/nonexistent@sha256:def"}),
			)

			log, logbuf := captureLog(zerolog.DebugLevel)
			i.log = log

			err := i.PullImage(context.Background(), pullContainer, warnAuto, types.UpdateParams{})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("image not found"))
			gomega.Expect(errors.Is(err, errPullImageNotFound)).To(gomega.BeTrue())
			gomega.Expect(errors.Is(err, errPullImageFailed)).To(gomega.BeFalse())
			gomega.Eventually(logbuf).Should(gbytes.Say(`Image pull failed: image not found in registry`))
		})
	})
	ginkgo.When("pulling an image with a server error", func() {
		ginkgo.It("should log at Debug level and return errPullImageFailed for other errors", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusInternalServerError, `{"message":"internal server error"}`),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			pullContainer := MockContainer(
				WithImageName("registry.example.com/app:latest"),
				WithRepoDigests([]string{"registry.example.com/app@sha256:ghi"}),
			)

			log, logbuf := captureLog(zerolog.DebugLevel)
			i.log = log

			err := i.PullImage(context.Background(), pullContainer, warnAuto, types.UpdateParams{})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("failed to pull image"))
			gomega.Expect(errors.Is(err, errPullImageFailed)).To(gomega.BeTrue())
			gomega.Expect(errors.Is(err, errPullImageUnauthorized)).To(gomega.BeFalse())
			gomega.Expect(errors.Is(err, errPullImageNotFound)).To(gomega.BeFalse())
			gomega.Eventually(logbuf).Should(gbytes.Say(`Failed to initiate image pull`))
		})
	})

	ginkgo.When("draining an image pull progress stream", func() {
		ginkgo.It("waits for a successful JSON progress stream", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusOK, `{"status":"Pulling fs layer"}`+"\n"+`{"status":"Download complete"}`+"\n"),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			err := i.performImagePull(
				context.Background(),
				"registry.example.com/app:latest",
				dockerClient.ImagePullOptions{},
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("returns a rate-limit error when the stream reports toomanyrequests", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(
						http.StatusOK,
						`{"status":"Pulling fs layer"}`+"\n"+
							`{"errorDetail":{"message":"toomanyrequests: retry-after: 2h, allowed: 44000/minute"},"error":"toomanyrequests: retry-after: 2h, allowed: 44000/minute"}`+"\n",
					),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			err := i.performImagePull(
				context.Background(),
				"ghcr.io/linuxserver/nginx:latest",
				dockerClient.ImagePullOptions{},
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(ratelimit.Is(err)).To(gomega.BeTrue())
		})
		ginkgo.It("returns a rate-limit error when ImagePull fails with toomanyrequests", func() {
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusTooManyRequests, `{"message":"toomanyrequests: retry-after: 2h, allowed: 44000/minute"}`),
				),
			)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			err := i.performImagePull(
				ctx,
				"ghcr.io/linuxserver/nginx:latest",
				dockerClient.ImagePullOptions{},
			)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(ratelimit.Is(err)).To(gomega.BeTrue())
		})
		ginkgo.It("backs off and succeeds when the daemon reports a sub-millisecond retry-after", func() {
			var pulls atomic.Int32

			mockServer.AllowUnhandledRequests = true
			mockServer.RouteToHandler("POST", regexp.MustCompile(`/images/create`), func(w http.ResponseWriter, _ *http.Request) {
				if pulls.Add(1) <= 2 {
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"message":"toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute"}`))

					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"Download complete"}` + "\n"))
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())

			started := time.Now()
			err := i.performImagePull(
				ctx,
				"ghcr.io/linuxserver/nginx:latest",
				dockerClient.ImagePullOptions{},
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pulls.Load()).To(gomega.Equal(int32(3)))
			// Two bucket waits of at least 100ms and 150ms separate the three pulls.
			gomega.Expect(time.Since(started)).To(gomega.BeNumerically(">=", 250*time.Millisecond))
		})
	})

	ginkgo.When("the pull slot context is canceled", func() {
		ginkgo.It("does not acquire a slot", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := acquirePullSlot(ctx, "ghcr.io")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue())
		})
		ginkgo.It("uses a distinct slot per registry host", func() {
			ghcr := pullSlotFor("ghcr.io")
			hub := pullSlotFor("index.docker.io")
			gomega.Expect(ghcr).NotTo(gomega.BeIdenticalTo(hub))
			gomega.Expect(pullSlotFor("ghcr.io")).To(gomega.BeIdenticalTo(ghcr))
		})
		ginkgo.It("uses distinct slots for anonymous and authenticated GHCR pulls", func() {
			anon := ratelimit.Scope("ghcr.io", false)
			authed := ratelimit.Scope("ghcr.io", true)

			gomega.Expect(anon).To(gomega.Equal("ghcr.io|anon"))
			gomega.Expect(authed).To(gomega.Equal("ghcr.io"))
			gomega.Expect(anon).NotTo(gomega.Equal(authed))

			authEntered := make(chan struct{})
			releaseAuth := make(chan struct{})
			complete := `{"status":"Download complete"}` + "\n"

			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					func(w http.ResponseWriter, req *http.Request) {
						close(authEntered)

						select {
						case <-releaseAuth:
						case <-req.Context().Done():
						}

						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(complete))
					},
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusOK, complete),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			authDone := make(chan error, 1)
			go func() {
				authDone <- i.performImagePull(
					ctx,
					"ghcr.io/linuxserver/nginx:latest",
					dockerClient.ImagePullOptions{RegistryAuth: "e30="},
				)
			}()

			gomega.Eventually(authEntered).Should(gomega.BeClosed())

			anonDone := make(chan error, 1)
			go func() {
				anonDone <- i.performImagePull(
					ctx,
					"ghcr.io/linuxserver/nginx:latest",
					dockerClient.ImagePullOptions{},
				)
			}()

			gomega.Eventually(anonDone, "1s").Should(gomega.Receive(gomega.BeNil()))
			close(releaseAuth)
			gomega.Eventually(authDone).Should(gomega.Receive(gomega.BeNil()))
		})
	})

	ginkgo.When("a host is in rate-limit cooldown", func() {
		ginkgo.It("lets pulls to other registry hosts proceed during the wait", func() {
			// An anonymous GHCR pull waits on the anonymous limiter key.
			limiter := ratelimit.New()
			limiter.Observe(ratelimit.Scope("ghcr.io", false), &ratelimit.Error{RetryAfter: 800 * time.Millisecond})

			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusOK, `{"status":"Download complete"}`+"\n"),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusOK, `{"status":"Download complete"}`+"\n"),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, limiter)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			blocked := make(chan error, 1)
			go func() {
				blocked <- i.performImagePull(
					ctx,
					"ghcr.io/linuxserver/nginx:latest",
					dockerClient.ImagePullOptions{},
				)
			}()

			time.Sleep(50 * time.Millisecond)

			started := time.Now()
			err := i.performImagePull(
				ctx,
				"registry.example.com/app:latest",
				dockerClient.ImagePullOptions{},
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(time.Since(started)).To(gomega.BeNumerically("<", 300*time.Millisecond))

			// The ghcr.io pull is still waiting out the cooldown.
			gomega.Consistently(blocked, 300*time.Millisecond).ShouldNot(gomega.Receive())
			gomega.Eventually(blocked, 3*time.Second).Should(gomega.Receive(gomega.BeNil()))
		})
		ginkgo.It("does not start a queued same-host pull during Retry-After cooldown", func() {
			firstEntered := make(chan struct{})
			releaseFirst := make(chan struct{})
			secondStarted := make(chan time.Time, 1)

			var pulls atomic.Int32

			mockServer.AllowUnhandledRequests = true
			mockServer.RouteToHandler("POST", regexp.MustCompile(`/images/create`), func(w http.ResponseWriter, _ *http.Request) {
				if pulls.Add(1) == 1 {
					close(firstEntered)
					<-releaseFirst
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte(`{"message":"toomanyrequests: retry-after: 1s, allowed: 44000/minute"}`))

					return
				}

				select {
				case secondStarted <- time.Now():
				default:
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"Download complete"}` + "\n"))
			})

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			firstErr := make(chan error, 1)
			go func() {
				firstErr <- i.performImagePull(
					ctx,
					"ghcr.io/linuxserver/nginx:latest",
					dockerClient.ImagePullOptions{},
				)
			}()

			gomega.Eventually(firstEntered).Should(gomega.BeClosed())

			secondErr := make(chan error, 1)
			go func() {
				secondErr <- i.performImagePull(
					ctx,
					"ghcr.io/linuxserver/radarr:latest",
					dockerClient.ImagePullOptions{},
				)
			}()

			time.Sleep(50 * time.Millisecond)

			released := time.Now()

			close(releaseFirst)

			var started time.Time
			gomega.Eventually(secondStarted, 3*time.Second).Should(gomega.Receive(&started))
			gomega.Expect(started.Sub(released)).To(gomega.BeNumerically(">=", 800*time.Millisecond))

			gomega.Eventually(firstErr, 3*time.Second).Should(gomega.Receive())
			gomega.Eventually(secondErr, 3*time.Second).Should(gomega.Receive(gomega.BeNil()))
		})
	})

	ginkgo.When("pulling an image with registry mirrors configured", func() {
		ginkgo.It("should use canonical host when mirror Info() returns no mirrors", func() {
			// The RouteToHandler for /info already returns empty JSON (no RegistryConfig).
			// This tests the fallback path: no mirrors configured, should use canonical host.
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusUnauthorized, `{"message":"unauthorized"}`),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			pullContainer := MockContainer(
				WithImageName("private-registry.io/app:latest"),
				WithRepoDigests([]string{"private-registry.io/app@sha256:abc"}),
			)

			err := i.PullImage(context.Background(), pullContainer, warnAuto, types.UpdateParams{})
			// Should proceed to pull (shouldSkipPull returns false since no mirrors and
			// canonical registry request fails), resulting in errPullImageUnauthorized.
			gomega.Expect(errors.Is(err, errPullImageUnauthorized)).To(gomega.BeTrue())
		})

		ginkgo.It("should handle Info() API failure gracefully and fall back to canonical", func() {
			// Override the /info RouteToHandler by appending a failing handler first.
			// Since RouteToHandler always matches, we need to reset it to return an error.
			mockServer.RouteToHandler("GET", regexp.MustCompile(`/info$`),
				ghttp.RespondWith(http.StatusInternalServerError, "daemon error"),
			)
			mockServer.AllowUnhandledRequests = true
			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", gomega.MatchRegexp("/images/create")),
					ghttp.RespondWith(http.StatusUnauthorized, `{"message":"unauthorized"}`),
				),
			)

			i := newImageClient(mockClient, testLog(), registryConfig.Registry{}, ratelimit.New())
			pullContainer := MockContainer(
				WithImageName("private-registry.io/app:latest"),
				WithRepoDigests([]string{"private-registry.io/app@sha256:abc"}),
			)

			err := i.PullImage(context.Background(), pullContainer, warnAuto, types.UpdateParams{})
			// Info() fails, so resolveRegistryMirrorConfig returns nil, buildMirrorEndpoints
			// returns nil, and shouldSkipPull falls back to canonical host.
			gomega.Expect(errors.Is(err, errPullImageUnauthorized)).To(gomega.BeTrue())
		})
	})

	ginkgo.When("removing a image", func() {
		ginkgo.When("debug logging is enabled", func() {
			ginkgo.It("should log removed and untagged images", func() {
				imageA := util.GenerateRandomSHA256()
				imageAParent := util.GenerateRandomSHA256()
				images := map[string][]string{imageA: {imageAParent}}
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp("/containers/json")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []dockerContainer.Summary{}),
					),
					mockContainer.RemoveImageHandler(images),
				)

				c := &client{log: testLog(), api: mockClient}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				gomega.Expect(c.RemoveImageByID(context.Background(), types.ImageID(imageA), "test-image")).
					To(gomega.Succeed())
				shortA := types.ImageID(imageA).ShortID()
				shortAParent := types.ImageID(imageAParent).ShortID()
				expectedDeleted := shortA + ", " + shortAParent
				gomega.Eventually(logbuf).
					Should(gbytes.Say(`msg="Image removal details" deleted="%s" image_id=%s image_name=%s untagged=%s`, expectedDeleted, shortA, "test-image", shortA))
				gomega.Eventually(logbuf).
					Should(gbytes.Say(`msg="Cleaned up old image" image_id=%v image_name=%v`, shortA, "test-image"))
			})
		})
		ginkgo.When("image is not found", func() {
			ginkgo.It("should return an error", func() {
				image := util.GenerateRandomSHA256()

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp("/containers/json")),
						ghttp.RespondWithJSONEncoded(http.StatusOK, []dockerContainer.Summary{}),
					),
					mockContainer.RemoveImageHandler(nil),
				)

				c := &client{log: testLog(), api: mockClient}
				err := c.RemoveImageByID(context.Background(), types.ImageID(image), "test-image")
				gomega.Expect(cerrdefs.IsNotFound(err)).To(gomega.BeTrue())
			})
		})
		ginkgo.When("image is used by an active container", func() {
			// Test cases for all active container states.
			// These states are defined in pkg/container/image.go lines 216-217.
			ginkgo.DescribeTable("should skip removal and return ErrImageInUse for each active state",
				func(tc struct {
					names string
					state string
				},
				) {
					imageA := util.GenerateRandomSHA256()

					mockServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", gomega.MatchRegexp("/containers/json")),
							ghttp.RespondWithJSONEncoded(http.StatusOK, []dockerContainer.Summary{
								{
									ImageID: imageA,
									State:   dockerContainer.ContainerState(tc.state),
								},
							}),
						),
					)

					c := &client{log: testLog(), api: mockClient}

					log, _ := captureLog(zerolog.InfoLevel)
					c.log = log

					err := c.RemoveImageByID(context.Background(), types.ImageID(imageA), "test-image")
					gomega.Expect(err).To(gomega.MatchError(ErrImageInUse))
				},
				ginkgo.Entry("running", struct {
					names string
					state string
				}{names: "running", state: "running"}),
				ginkgo.Entry("restarting", struct {
					names string
					state string
				}{names: "restarting", state: "restarting"}),
				ginkgo.Entry("paused", struct {
					names string
					state string
				}{names: "paused", state: "paused"}),
				ginkgo.Entry("created", struct {
					names string
					state string
				}{names: "created", state: "created"}),
			)
		})
		ginkgo.When("ContainerList API fails", func() {
			ginkgo.It("should return an error and not proceed with removal", func() {
				imageA := util.GenerateRandomSHA256()

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp("/containers/json")),
						ghttp.RespondWithJSONEncoded(
							http.StatusInternalServerError,
							map[string]string{"message": "Internal server error"},
						),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				log, _ := captureLog(zerolog.InfoLevel)
				c.log = log

				err := c.RemoveImageByID(context.Background(), types.ImageID(imageA), "test-image")
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("cannot verify image usage"))
			})
		})
	})
	ginkgo.When("the context is canceled", func() {
		ginkgo.Describe("IsContainerStale", func() {
			ginkgo.It("should return context.Canceled error", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				// Create a canceled context
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately

				c := &client{log: testLog(), api: mockClient}

				_, _, _, err := c.IsContainerStale(
					ctx,
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.MatchError(context.Canceled))
			})
		})
		ginkgo.Describe("RemoveImageByID", func() {
			ginkgo.It("should return context.Canceled error without warning", func() {
				imageID := util.GenerateRandomSHA256()

				// Create a canceled context
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately

				log, logbuf := captureLog(zerolog.WarnLevel)
				c := &client{log: log, api: mockClient}

				err := c.RemoveImageByID(ctx, types.ImageID(imageID), "test-image")
				gomega.Expect(err).To(gomega.MatchError(context.Canceled))
				gomega.Expect(string(logbuf.Contents())).
					NotTo(gomega.ContainSubstring("Failed to list containers for image usage check"))
			})
		})
	})
	ginkgo.When("checking container staleness with no-pull", func() {
		ginkgo.When("no newer local image exists", func() {
			ginkgo.It("should return false and current image ID", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID: currentImageID,
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				stale, latestID, _, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeFalse())
				gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))
				gomega.Eventually(logbuf).
					Should(gbytes.Say(`msg="Skipping image pull due to no-pull setting - checking local image only"`))
				gomega.Eventually(logbuf).Should(gbytes.Say(`msg="No new image found"`))
			})
		})
		ginkgo.When("a newer local image exists", func() {
			ginkgo.It("should return true and new image ID", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID: newImageID,
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				stale, latestID, _, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(latestID).To(gomega.Equal(types.ImageID(newImageID)))
				gomega.Eventually(logbuf).
					Should(gbytes.Say(`msg="Skipping image pull due to no-pull setting - checking local image only"`))
				gomega.Eventually(logbuf).Should(gbytes.Say(`msg="Found new image"`))
			})
		})
		ginkgo.When("a newer local image exists with a repo digest", func() {
			ginkgo.It("should return the latest digest from RepoDigests", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newImageID := "sha256:" + util.GenerateRandomSHA256()
				newDigest := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID:          newImageID,
							RepoDigests: []string{"test-image:latest@" + newDigest},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(string(latestID)).To(gomega.Equal(newImageID))
				gomega.Expect(latestDigest).To(gomega.Equal(newDigest))
			})
		})
		ginkgo.When("a newer local image exists with empty RepoDigests", func() {
			ginkgo.It("should return an empty latest digest", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID:          newImageID,
							RepoDigests: []string{},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(string(latestID)).To(gomega.Equal(newImageID))
				gomega.Expect(latestDigest).To(gomega.BeEmpty())
			})
		})
		ginkgo.When("a newer local image exists with malformed RepoDigests", func() {
			ginkgo.It("should return an empty latest digest when no @ separator", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID:          newImageID,
							RepoDigests: []string{"malformed-digest-no-at-separator"},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(string(latestID)).To(gomega.Equal(newImageID))
				gomega.Expect(latestDigest).To(gomega.BeEmpty())
			})
		})
		ginkgo.When("image inspection fails", func() {
			ginkgo.It("should return an error", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(
							http.StatusNotFound,
							map[string]string{"message": "No such image"},
						),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				stale, latestID, _, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{NoPull: true},
				)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(stale).To(gomega.BeFalse())
				gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))
				gomega.Eventually(logbuf).
					Should(gbytes.Say(`msg="Skipping image pull due to no-pull setting - checking local image only"`))
				gomega.Eventually(logbuf).Should(gbytes.Say(`msg="Failed to inspect latest image"`))
			})
		})
	})
	ginkgo.When("pulling and checking for updates", func() {
		ginkgo.When("a newer image is available with a repo digest", func() {
			ginkgo.It("should return the latest digest after pulling", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newImageID := "sha256:" + util.GenerateRandomSHA256()
				newDigest := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("test-image:latest"),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AllowUnhandledRequests = true
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/test-image:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID:          newImageID,
							RepoDigests: []string{"test-image:latest@" + newDigest},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(string(latestID)).To(gomega.Equal(newImageID))
				gomega.Expect(latestDigest).To(gomega.Equal(newDigest))
			})
		})
		ginkgo.When("the image is not found in any registry", func() {
			ginkgo.It("should treat the container as up-to-date without error", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("atlas-badgerdb:latest"),
					WithRepoDigests([]string{"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35"}),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				// Domain-less Config.Image + registry 404 is handled inside
				// CompareDigest as match=true.
				// PullImage skips without ImagePull.
				// HasNewImage still runs and inspects the local image by name.
				mockServer.AllowUnhandledRequests = true
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", gomega.MatchRegexp(`/images/.+/json`)),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID: currentImageID,
							RepoDigests: []string{
								"atlas-badgerdb@sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
							},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient, RateLimiter: ratelimit.New()}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeFalse())
				gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))
				gomega.Expect(latestDigest).To(gomega.Equal(
					"sha256:80f07677bee57274a48929d0688bf0cfabe5e83f06f2f152dab4076445d6ab35",
				))
				gomega.Eventually(logbuf).Should(gbytes.Say(`Digest match, skipping pull`))

				for _, req := range mockServer.ReceivedRequests() {
					gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
				}
			})
		})

		// Empty RepoDigests skips the registry pull, but HasNewImage still
		// compares the running image ID to the local tag.
		ginkgo.When("container image has empty RepoDigests but the local tag points at a different ID", func() {
			ginkgo.It("skips pull and still reports the container as stale", func() {
				currentImageID := "sha256:" + util.GenerateRandomSHA256()
				newerLocalImageID := "sha256:" + util.GenerateRandomSHA256()
				container := MockContainer(
					WithImageName("registry.example.com/app:latest"),
					WithRepoDigests([]string{}),
					func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
						container.Image = currentImageID
						image.ID = currentImageID
					},
				)

				mockServer.AllowUnhandledRequests = true
				mockServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest(
							"GET",
							gomega.HaveSuffix("/images/registry.example.com/app:latest/json"),
						),
						ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
							ID:          newerLocalImageID,
							RepoDigests: []string{},
						}),
					),
				)

				c := &client{log: testLog(), api: mockClient}

				log, logbuf := captureLog(zerolog.DebugLevel)
				c.log = log

				stale, latestID, latestDigest, err := c.IsContainerStale(
					context.Background(),
					container,
					types.UpdateParams{},
				)
				gomega.Expect(err).To(gomega.Succeed())
				gomega.Expect(stale).To(gomega.BeTrue())
				gomega.Expect(string(latestID)).To(gomega.Equal(newerLocalImageID))
				gomega.Expect(latestDigest).To(gomega.BeEmpty())
				gomega.Eventually(logbuf).Should(gbytes.Say(`empty RepoDigests`))
				gomega.Eventually(logbuf).Should(gbytes.Say(`Digest match, skipping pull`))
				gomega.Eventually(logbuf).Should(gbytes.Say(`Found new image`))

				for _, req := range mockServer.ReceivedRequests() {
					gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
				}
			})
		})
	})
})

var _ = ginkgo.Describe("IsImagePinnedByDigest", func() {
	// Valid 64-hex digest accepted by distribution/reference + go-digest.
	const fullDigest = "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	ginkgo.DescribeTable("pin detection",
		func(imageName string, wantPinned bool) {
			gomega.Expect(IsImagePinnedByDigest(imageName)).To(gomega.Equal(wantPinned))
		},
		ginkgo.Entry("empty", "", false),
		ginkgo.Entry("tag only", "nginx:latest", false),
		ginkgo.Entry("untagged name (not a digest)", "nginx", false),
		ginkgo.Entry("substring sha256 in tag name", "myapp-sha256-build:1.0", false),
		ginkgo.Entry("bare content digest", fullDigest, true),
		ginkgo.Entry("repo@digest", "nginx@"+fullDigest, true),
		ginkgo.Entry("org/repo@digest", "library/nginx@"+fullDigest, true),
		ginkgo.Entry("registry/repo@digest", "registry.example.com/org/app@"+fullDigest, true),
		ginkgo.Entry("registry with port@digest", "registry.example.com:5000/app@"+fullDigest, true),
		ginkgo.Entry("tag and digest", "nginx:1.27@"+fullDigest, true),
		ginkgo.Entry("fully qualified tag and digest", "docker.io/library/nginx:latest@"+fullDigest, true),
		// Parse fails. String fallback treats explicit @sha256: as pinned.
		ginkgo.Entry("malformed empty digest still pinned", "nginx@sha256:", true),
		ginkgo.Entry("at-sha256 without algorithm still not bare pin", "nginx@deadbeef", false),
	)
})

// withContainerImageName creates a Gomega matcher for container image name.
//
// Parameters:
//   - matcher: Matcher for the image name.
//
// Returns:
//   - gomegaTypes.GomegaMatcher: Matcher for verifying image name.
func withContainerImageName(matcher gomegaTypes.GomegaMatcher) gomegaTypes.GomegaMatcher {
	return gomega.WithTransform(func(container types.Container) string {
		return container.ImageName()
	}, matcher)
}

var _ = ginkgo.Describe("CheckContainerUpdate", func() {
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
		mockServer.RouteToHandler(
			"GET",
			regexp.MustCompile(`/info$`),
			ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]any{}),
		)
	})
	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.When("image is pinned by digest", func() {
		ginkgo.It("reports no update for bare sha256 without contacting the registry", func() {
			pinnedID := "sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b"
			container := MockContainer(WithImageName(pinnedID))
			c := &client{log: testLog(), api: mockClient}

			available, latestID, latestDigest, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeFalse())
			gomega.Expect(string(latestID)).To(gomega.Equal(string(container.ImageID())))
			gomega.Expect(latestDigest).To(gomega.BeEmpty())

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}
		})
		ginkgo.It("reports no update for repo@sha256 without contacting the registry", func() {
			pinnedRef := "nginx@sha256:fa5269854a5e615e51a72b17ad3fd1e01268f278a6684c8ed3c5f0cdce3f230b"
			container := MockContainer(WithImageName(pinnedRef))
			c := &client{log: testLog(), api: mockClient}

			available, latestID, latestDigest, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeFalse())
			gomega.Expect(string(latestID)).To(gomega.Equal(string(container.ImageID())))
			gomega.Expect(latestDigest).To(gomega.BeEmpty())

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}
		})
	})

	ginkgo.When("no-pull is enabled", func() {
		ginkgo.It("uses local image inspection only", func() {
			currentImageID := "sha256:" + util.GenerateRandomSHA256()
			container := MockContainer(
				WithImageName("test-image:latest"),
				func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
					container.Image = currentImageID
					image.ID = currentImageID
				},
			)

			mockServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(
						"GET",
						gomega.HaveSuffix("/images/test-image:latest/json"),
					),
					ghttp.RespondWithJSONEncoded(http.StatusOK, dockerImage.InspectResponse{
						ID: currentImageID,
					}),
				),
			)

			c := &client{log: testLog(), api: mockClient}

			available, latestID, _, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{NoPull: true},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeFalse())
			gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}
		})
	})

	ginkgo.When("local image has empty RepoDigests", func() {
		ginkgo.It("treats the image as up-to-date without pulling", func() {
			currentImageID := "sha256:" + util.GenerateRandomSHA256()
			container := MockContainer(
				WithImageName("local-build:latest"),
				WithRepoDigests([]string{}),
				func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
					container.Image = currentImageID
					image.ID = currentImageID
				},
			)

			c := &client{log: testLog(), api: mockClient}

			available, latestID, latestDigest, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeFalse())
			gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))
			gomega.Expect(latestDigest).To(gomega.BeEmpty())

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}
		})
	})

	ginkgo.When("registry digest comparison is performed", func() {
		var registryServer *ghttp.Server

		ginkgo.BeforeEach(func() {
			registryServer = ghttp.NewServer()
		})
		ginkgo.AfterEach(func() {
			registryServer.Close()
		})

		// appendUnauthenticatedRegistryHandlers stubs GET /v2/ (no auth) and HEAD manifest.
		appendUnauthenticatedRegistryHandlers := func(remoteDigest string) {
			registryServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("HEAD", "/v2/test/image/manifests/latest"),
					ghttp.RespondWith(http.StatusOK, nil, http.Header{
						digest.ContentDigestHeader: []string{remoteDigest},
					}),
				),
			)
		}

		ginkgo.It("reports no update when remote digest matches local RepoDigests", func() {
			serverURL, err := url.Parse(registryServer.URL())
			gomega.Expect(err).To(gomega.Succeed())

			localDigest := "sha256:" + util.GenerateRandomSHA256()
			currentImageID := "sha256:" + util.GenerateRandomSHA256()
			imageRef := serverURL.Host + "/test/image:latest"

			appendUnauthenticatedRegistryHandlers(localDigest)

			container := MockContainer(
				WithImageName(imageRef),
				WithRepoDigests([]string{
					fmt.Sprintf("%s/test/image@%s", serverURL.Host, localDigest),
				}),
				func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
					container.Image = currentImageID
					image.ID = currentImageID
				},
			)

			// The test registry serves plain HTTP.
			c := &client{
				log:         testLog(),
				api:         mockClient,
				Registry:    registryConfig.Registry{TLSSkip: true},
				RateLimiter: ratelimit.New(),
			}

			available, latestID, latestDigest, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeFalse())
			gomega.Expect(latestID).To(gomega.Equal(types.ImageID(currentImageID)))
			gomega.Expect(latestDigest).To(gomega.Equal(localDigest))

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}

			gomega.Expect(registryServer.ReceivedRequests()).To(gomega.HaveLen(2))
		})

		ginkgo.It("reports update available when remote digest differs from local RepoDigests", func() {
			serverURL, err := url.Parse(registryServer.URL())
			gomega.Expect(err).To(gomega.Succeed())

			localDigest := "sha256:" + util.GenerateRandomSHA256()
			remoteDigest := "sha256:" + util.GenerateRandomSHA256()
			currentImageID := "sha256:" + util.GenerateRandomSHA256()
			imageRef := serverURL.Host + "/test/image:latest"

			appendUnauthenticatedRegistryHandlers(remoteDigest)

			container := MockContainer(
				WithImageName(imageRef),
				WithRepoDigests([]string{
					fmt.Sprintf("%s/test/image@%s", serverURL.Host, localDigest),
				}),
				func(container *dockerContainer.InspectResponse, image *dockerImage.InspectResponse) {
					container.Image = currentImageID
					image.ID = currentImageID
				},
			)

			// The test registry serves plain HTTP.
			c := &client{
				log:         testLog(),
				api:         mockClient,
				Registry:    registryConfig.Registry{TLSSkip: true},
				RateLimiter: ratelimit.New(),
			}

			available, latestID, latestDigest, err := c.CheckContainerUpdate(
				context.Background(),
				container,
				types.UpdateParams{},
			)
			gomega.Expect(err).To(gomega.Succeed())
			gomega.Expect(available).To(gomega.BeTrue())
			gomega.Expect(latestID).To(gomega.Equal(types.ImageID("")))
			gomega.Expect(latestDigest).To(gomega.Equal(remoteDigest))

			for _, req := range mockServer.ReceivedRequests() {
				gomega.Expect(req.URL.Path).ToNot(gomega.ContainSubstring("/images/create"))
			}

			gomega.Expect(registryServer.ReceivedRequests()).To(gomega.HaveLen(2))
		})
	})
})

// IsOutsideCooldown tests (white-box, early-out paths require no registry).
var _ = ginkgo.Describe("ExtractImageDigest", func() {
	ginkgo.When("RepoDigests is empty", func() {
		ginkgo.It("returns an empty string", func() {
			gomega.Expect(ExtractImageDigest(nil, "")).To(gomega.Equal(""))
			gomega.Expect(ExtractImageDigest([]string{}, "")).To(gomega.Equal(""))
		})
	})

	ginkgo.When("RepoDigests contain no @ separator", func() {
		ginkgo.It("returns an empty string", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"malformed-without-at",
				"sha256:abcdef",
			}, "")).To(gomega.Equal(""))
		})
	})

	ginkgo.When("RepoDigests contain valid entries", func() {
		ginkgo.It("returns the digest from the first matching entry", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"nginx:latest@sha256:0123456789abcdef",
			}, "")).To(gomega.Equal("sha256:0123456789abcdef"))
		})

		ginkgo.It("skips malformed entries before the first valid one", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"malformed-no-at",
				"nginx:latest@sha256:0123456789abcdef",
			}, "")).To(gomega.Equal("sha256:0123456789abcdef"))
		})

		ginkgo.It("prefers the RepoDigest matching the image name", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"other.io/foo@sha256:aaa",
				"docker.io/library/nginx@sha256:bbb",
			}, "nginx:latest")).To(gomega.Equal("sha256:bbb"))
		})

		ginkgo.It("preserves the registry port when matching a tagged image name", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"registry.example.com:5000/nginx@sha256:def456",
				"docker.io/library/nginx@sha256:bbb",
			}, "registry.example.com:5000/nginx:latest")).To(gomega.Equal("sha256:def456"))
		})

		ginkgo.It("matches the preferred RepoDigest when imageName is in digest form", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"docker.io/library/nginx@sha256:bbb",
				"other.io/foo@sha256:aaa",
			}, "nginx@sha256:abc123")).To(gomega.Equal("sha256:bbb"))
		})

		ginkgo.It("falls back to the first valid digest when no name matches", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"other.io/foo@sha256:aaa",
				"other.io/bar@sha256:bbb",
			}, "nginx:latest")).To(gomega.Equal("sha256:aaa"))
		})

		ginkgo.It("matches fully-qualified image names to short RepoDigest paths", func() {
			gomega.Expect(ExtractImageDigest([]string{
				"library/nginx@sha256:ccc",
			}, "docker.io/library/nginx:latest")).To(gomega.Equal("sha256:ccc"))
		})
	})
})

var _ = ginkgo.Describe("IsOutsideCooldown (cooldown gating before pull)", func() {
	ginkgo.When("no cooldown delay is configured", func() {
		ginkgo.It("returns true (safe to pull) with no registry calls", func() {
			i := newImageClient(nil, testLog(), registryConfig.Registry{}, ratelimit.New())
			c := MockContainer(WithImageName("test:latest"))

			outside, err := i.isOutsideCooldown(
				context.Background(), c, types.UpdateParams{},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(outside).To(gomega.BeTrue())
		})
	})

	ginkgo.When("container is monitor-only or no-pull", func() {
		ginkgo.It("returns true (bypasses cooldown check)", func() {
			i := newImageClient(nil, testLog(), registryConfig.Registry{}, ratelimit.New())
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
})
