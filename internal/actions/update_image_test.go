package actions_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

var _ = ginkgo.Describe("the update action", func() {
	// Tests for image reference handling to cover isPinned functionality
	ginkgo.When("handling different image reference formats", func() {
		var (
			client *mockActions.MockClient
			config types.UpdateParams
		)

		ginkgo.BeforeEach(func() {
			config = types.UpdateParams{
				Cleanup: true,
				Filter:  filters.NoFilter,
			}
		})

		ginkgo.It("should process tagged images and update if stale", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"tagged-container",
							"/tagged-container",
							"image:1.0.0",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"tagged-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Scanned()).
				To(gomega.HaveLen(1), "Tagged container should be scanned")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Tagged container should be updated if stale")
			gomega.Expect(cleanupImageInfos).
				To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("image:1.0.0"))))
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(1)), "IsContainerStale should be called")
		})

		ginkgo.It("should process untagged images and update if stale", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"untagged-container",
							"/untagged-container",
							"image",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"untagged-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Scanned()).
				To(gomega.HaveLen(1), "Untagged container should be scanned")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Untagged container should be updated if stale")
			gomega.Expect(cleanupImageInfos).
				To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID("image"))))
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(1)), "IsContainerStale should be called")
		})

		ginkgo.It("should skip pinned containers and not collect image IDs", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"pinned-container",
							"/pinned-container",
							"image@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"pinned-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Scanned()).
				To(gomega.HaveLen(1), "Pinned container should be scanned")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "Pinned container should not be updated")
			gomega.Expect(cleanupImageInfos).
				To(gomega.BeEmpty(), "No image IDs should be collected for pinned container")
			gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
				To(gomega.Equal(int32(0)), "RemoveImageByID should not be called")
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(0)), "IsContainerStale should not be called")
		})

		ginkgo.It("should skip bare sha256-pinned containers and not call IsContainerStale", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"bare-digest-container",
							"/bare-digest-container",
							"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"bare-digest-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Scanned()).
				To(gomega.HaveLen(1), "Bare digest container should be scanned")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "Bare digest container should not be updated")
			gomega.Expect(cleanupImageInfos).To(gomega.BeEmpty())
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(0)), "IsContainerStale should not be called for bare digest pin")
		})

		ginkgo.It(
			"should skip pinned containers with tag and digest and not collect image IDs",
			func() {
				client = &mockActions.MockClient{
					TestData: &mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainer(
								"pinned-tagged-container",
								"/pinned-tagged-container",
								"image:latest@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								time.Now(),
							),
						},
						Staleness: map[string]bool{
							"pinned-tagged-container": true,
						},
					},
					Stopped: make(map[string]bool),
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					config,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Scanned()).
					To(gomega.HaveLen(1), "Pinned container should be scanned")
				gomega.Expect(report.Updated()).
					To(gomega.BeEmpty(), "Pinned container should not be updated")
				gomega.Expect(cleanupImageInfos).
					To(gomega.BeEmpty(), "No image IDs should be collected for pinned container")
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called")
				gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
					To(gomega.Equal(int32(0)), "IsContainerStale should not be called")
			},
		)

		ginkgo.It("should process invalid image references with fallback", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"invalid-container",
							"/invalid-container",
							":latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"invalid-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Scanned()).
				To(gomega.HaveLen(1), "Invalid container should be scanned with fallback")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Invalid container should be updated with valid fallback")
			gomega.Expect(cleanupImageInfos).
				To(gomega.ContainElement(gomega.HaveField("ImageID", types.ImageID(":latest"))))
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(1)), "IsContainerStale should be called")
		})

		ginkgo.It(
			"should skip containers with missing Config.Image and imageInfo.ID with error",
			func() {
				client = &mockActions.MockClient{
					TestData: &mockActions.TestData{
						Containers: []types.Container{
							mockActions.CreateMockContainerWithImageInfoP(
								"edge-container",
								"/edge-container",
								"",
								time.Now(),
								nil,
							),
						},
						Staleness: map[string]bool{
							"edge-container": true,
						},
					},
					Stopped: make(map[string]bool),
				}
				report, cleanupImageInfos, err := actions.Update(newNopLogger(),
					context.Background(),
					client,
					config,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(report.Skipped()).
					To(gomega.HaveLen(1), "Container with missing image info should be skipped")
				gomega.Expect(report.Scanned()).
					To(gomega.BeEmpty(), "Container should not be scanned")
				gomega.Expect(report.Updated()).
					To(gomega.BeEmpty(), "Container should not be updated")
				gomega.Expect(cleanupImageInfos).
					To(gomega.BeEmpty(), "No image IDs should be collected")
				gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
					To(gomega.Equal(int32(0)), "RemoveImageByID should not be called")
				gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
					To(gomega.Equal(int32(1)), "IsContainerStale should be called")
			},
		)

		// This test differs from the previous one ("should process invalid image references with fallback")
		// in that the container name "InvalidContainer" contains uppercase characters, making the fallback
		// image "InvalidContainer:latest" invalid since Docker image names must be lowercase.
		// In contrast, "invalid-container:latest" is valid.
		ginkgo.It("should skip containers with invalid fallback image reference", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"InvalidContainer",
							"/InvalidContainer",
							":latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"InvalidContainer": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.HaveLen(1), "Container with invalid fallback image should be skipped")
			gomega.Expect(report.Scanned()).To(gomega.BeEmpty(), "Container should not be scanned")
			gomega.Expect(report.Updated()).To(gomega.BeEmpty(), "Container should not be updated")
			gomega.Expect(cleanupImageInfos).
				To(gomega.BeEmpty(), "No image IDs should be collected")
			gomega.Expect(client.TestData.TriedToRemoveImageCount.Load()).
				To(gomega.Equal(int32(0)), "RemoveImageByID should not be called")
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(0)), "IsContainerStale should not be called")
		})

		// Verify that failures across parallel staleness checks are all counted
		// in the final report when IsContainerStale returns an error for every container.
		ginkgo.It("should count all parallel staleness failures in the final report", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainerWithImageInfoP(
							"invalid-ref-container",
							"/invalid-ref-container",
							"",
							time.Now(),
							nil,
						),
						mockActions.CreateMockContainer(
							"stale-fail-container",
							"/stale-fail-container",
							"image:latest",
							time.Now(),
						),
						mockActions.CreateMockContainer(
							"normal-container",
							"/normal-container",
							"image:latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"invalid-ref-container": true,
						"stale-fail-container":  true,
						"normal-container":      true,
					},
				},
				Stopped: make(map[string]bool),
			}
			client.TestData.IsContainerStaleError = errors.New("stale check failed")

			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.HaveLen(3), "All three containers should be skipped when IsContainerStale fails")
			gomega.Expect(report.Scanned()).
				To(gomega.BeEmpty(), "No containers should be scanned when all fail")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "No containers should be updated when IsContainerStale fails")
			gomega.Expect(cleanupImageInfos).
				To(gomega.BeEmpty(), "No image IDs should be collected")
			gomega.Expect(client.TestData.IsContainerStaleCount.Load()).
				To(gomega.Equal(int32(3)), "IsContainerStale should be called for all three containers")
		})

		ginkgo.It("should count exhausted registry rate limits as failed", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"rate-limited-container",
							"/rate-limited-container",
							"lscr.io/linuxserver/sonarr:latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"rate-limited-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			client.TestData.IsContainerStaleError = &ratelimit.Error{
				RetryAfter:    23722 * time.Nanosecond,
				Allowed:       44000,
				AllowedWindow: time.Minute,
				Host:          "ghcr.io",
			}

			report, cleanupImageInfos, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Failed()).
				To(gomega.HaveLen(1), "Rate-limited staleness checks should be failed")
			gomega.Expect(report.Skipped()).
				To(gomega.BeEmpty(), "Rate-limited staleness checks should not be skipped")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty())
			gomega.Expect(cleanupImageInfos).
				To(gomega.BeEmpty())
		})

		ginkgo.It("logs one notifiable warning per rate-limited container", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"rate-limited-container",
							"/rate-limited-container",
							"lscr.io/linuxserver/sonarr:latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"rate-limited-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			client.TestData.IsContainerStaleError = &ratelimit.Error{
				RetryAfter:    23722 * time.Nanosecond,
				Allowed:       44000,
				AllowedWindow: time.Minute,
				Host:          "ghcr.io",
			}

			var logs bytes.Buffer

			log := zerolog.New(&logs)

			report, _, err := actions.Update(&log,
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Failed()).To(gomega.HaveLen(1))

			gomega.Expect(strings.Count(logs.String(), "Registry rate limit retries exhausted")).
				To(gomega.Equal(1), "One warning per failed container")

			var warning string

			for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
				if strings.Contains(line, "Registry rate limit retries exhausted") {
					warning = line
				}
			}

			gomega.Expect(warning).To(gomega.ContainSubstring(`"level":"warn"`))
			gomega.Expect(warning).NotTo(gomega.ContainSubstring(`"notify":"no"`),
				"The warning must reach log-based notifications")
			gomega.Expect(warning).To(gomega.ContainSubstring("rate-limited-container"))
		})

		ginkgo.It("leaves a long Retry-After to the retry loop's warning", func() {
			client = &mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer(
							"rate-limited-container",
							"/rate-limited-container",
							"lscr.io/linuxserver/sonarr:latest",
							time.Now(),
						),
					},
					Staleness: map[string]bool{
						"rate-limited-container": true,
					},
				},
				Stopped: make(map[string]bool),
			}
			client.TestData.IsContainerStaleError = &ratelimit.Error{
				RetryAfter: 2 * time.Hour,
				Host:       "ghcr.io",
			}

			var logs bytes.Buffer

			log := zerolog.New(&logs)

			report, _, err := actions.Update(&log,
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Failed()).To(gomega.HaveLen(1))
			gomega.Expect(logs.String()).NotTo(gomega.ContainSubstring("Registry rate limit retries exhausted"),
				"The retry loop already warned about a Retry-After beyond the honor window")
		})
	})
})
