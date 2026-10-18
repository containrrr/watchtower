package actions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/nicholas-fedor/watchtower/internal/actions"
	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// registryManifest represents a single-platform OCI/Docker manifest used by the mock server.
type registryManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int64  `json:"size"`
	} `json:"config"`
}

// registryConfig represents an image config blob used by the mock server.
type registryConfig struct {
	Created *time.Time `json:"created"`
}

// mockRegistryHandlers returns HTTP handlers for a minimal OCI registry that
// serves a manifest and config blob with the given creation time.
// The handlers cover:
//   - GET /v2/             → 200 (no-auth ping)
//   - GET /v2/{name}/manifests/{tag} → manifest with config digest
//   - GET /v2/{name}/blobs/{digest}  → config blob with creation timestamp
func mockRegistryHandlers(creationTime time.Time) []http.HandlerFunc {
	const configDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	return []http.HandlerFunc{
		// GET /v2/ — OCI registry version check (returns 200 to skip auth).
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/"),
			ghttp.RespondWith(http.StatusOK, ""),
		),
		// GET /v2/{name}/manifests/{tag} — return a manifest with a config digest.
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", gomega.MatchRegexp(`/v2/.+/manifests/.+`)),
			ghttp.VerifyHeader(http.Header{
				"Accept": []string{
					"application/vnd.oci.image.index.v1+json, " +
						"application/vnd.docker.distribution.manifest.list.v2+json, " +
						"application/vnd.oci.image.manifest.v1+json, " +
						"application/vnd.docker.distribution.manifest.v2+json",
				},
			}),
			func(w http.ResponseWriter, r *http.Request) {
				manifest := registryManifest{
					SchemaVersion: 2,
					MediaType:     "application/vnd.oci.image.manifest.v1+json",
				}
				manifest.Config.MediaType = "application/vnd.oci.image.config.v1+json"
				manifest.Config.Digest = configDigest
				manifest.Config.Size = 1234

				var buf bytes.Buffer

				err := json.NewEncoder(&buf).Encode(manifest)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)

					return
				}

				w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
				w.WriteHeader(http.StatusOK)

				_, _ = w.Write(buf.Bytes())
			},
		),
		// GET /v2/{name}/blobs/{digest} — return a config blob with creation time.
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", gomega.MatchRegexp(`/v2/.+/blobs/`+configDigest)),
			func(w http.ResponseWriter, r *http.Request) {
				cfg := registryConfig{Created: &creationTime}

				var buf bytes.Buffer

				err := json.NewEncoder(&buf).Encode(cfg)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)

					return
				}

				w.Header().Set("Content-Type", "application/vnd.oci.image.config.v1+json")
				w.WriteHeader(http.StatusOK)

				_, _ = w.Write(buf.Bytes())
			},
		),
	}
}

// extractHost strips the scheme ("http://") from a ghttp.Server URL to produce
// a bare host:port suitable for constructing image references.
func extractHost(serverURL string) string {
	return strings.TrimPrefix(serverURL, "http://")
}

// cooldownConfig returns a base UpdateParams with CooldownDelay and standard settings.
func cooldownConfig(cooldownDelay time.Duration) types.UpdateParams {
	return types.UpdateParams{
		Cleanup:       true,
		Filter:        filters.NoFilter,
		CooldownDelay: cooldownDelay,
	}
}

var _ = ginkgo.Describe("the update action cooldown", func() {
	var registryServer *ghttp.Server

	ginkgo.AfterEach(func() {
		if registryServer != nil {
			registryServer.Close()
		}
	})

	ginkgo.When("CooldownDelay is set and image is newer than cooldown", func() {
		ginkgo.It("should defer the update and skip the container", func() {
			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "myimage:latest", time.Now()),
					},
					IsContainerStaleError: container.ErrImageCooldown,
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				cooldownConfig(1*time.Hour),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.HaveLen(1), "Container should be skipped due to cooldown")
			gomega.Expect(report.Scanned()).
				To(gomega.BeEmpty(), "Container should not be scanned when deferred by cooldown")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "Container should not be updated when deferred by cooldown")
		})
	})

	ginkgo.When("CooldownDelay is set and image is older than cooldown", func() {
		ginkgo.It("should proceed with the update normally", func() {
			// Image was created 2 days ago. Cooldown is 1 hour.
			registryServer = ghttp.NewServer()
			registryServer.AppendHandlers(mockRegistryHandlers(time.Now().Add(-48 * time.Hour))...)

			host := extractHost(registryServer.URL())
			imageName := host + "/myimage:latest"
			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", imageName, time.Now()),
					},
					Staleness: map[string]bool{
						"c1": true,
					},
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				cooldownConfig(1*time.Hour),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.BeEmpty(), "Container should not be skipped when cooldown has elapsed")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Container should be updated after cooldown passes")
		})
	})

	ginkgo.When("CooldownDelay is zero", func() {
		ginkgo.It("should skip the cooldown check entirely", func() {
			// No mock registry server is needed because the cooldown check
			// is skipped when CooldownDelay is zero.
			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "fake-image:latest", time.Now()),
					},
					Staleness: map[string]bool{
						"c1": true,
					},
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				cooldownConfig(0),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.BeEmpty(), "Container should not be skipped when cooldown is disabled")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Container should be updated when cooldown is disabled")
		})
	})

	ginkgo.When("NoPull is enabled", func() {
		ginkgo.It("should skip the cooldown check", func() {
			// No mock registry server is needed because NoPull bypasses the cooldown check.
			// Use a standard mock container via CreateMockContainer.
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "fake-image:latest", time.Now()),
					},
					Staleness: map[string]bool{
						"c1": true,
					},
				},
				false,
				false,
			)

			config := cooldownConfig(1 * time.Hour)
			config.NoPull = true

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.BeEmpty(), "Container should not be skipped when NoPull is set")
			gomega.Expect(report.Updated()).
				To(gomega.HaveLen(1), "Container should be updated when NoPull bypasses cooldown")
		})
	})

	ginkgo.When("image age fetch fails", func() {
		ginkgo.It("should skip the container with a conservative posture", func() {
			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "myimage:latest", time.Now()),
					},
					IsContainerStaleError: container.ErrImageCooldown,
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				cooldownConfig(1*time.Hour),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.HaveLen(1), "Container should be skipped when image age fetch fails")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "Container should not be updated when image age fetch fails")
		})
	})

	ginkgo.When("MonitorOnly is enabled", func() {
		ginkgo.It("should skip the cooldown check entirely", func() {
			// No mock registry server is needed because monitor-only bypasses the cooldown check.
			// Use a standard mock container via CreateMockContainer.
			client := mockActions.CreateMockClient(
				&mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "fake-image:latest", time.Now()),
					},
					Staleness: map[string]bool{
						"c1": true,
					},
				},
				false,
				false,
			)

			config := cooldownConfig(1 * time.Hour)
			config.MonitorOnly = true

			report, _, err := actions.Update(newNopLogger(),
				context.Background(),
				client,
				config,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(report.Skipped()).
				To(gomega.BeEmpty(), "Container should not be skipped when MonitorOnly is set")
			gomega.Expect(report.Updated()).
				To(gomega.BeEmpty(), "Monitor-only container should not be updated")
		})
	})
})
