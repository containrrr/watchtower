package actions

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TestUpdateAction_ImageReferences verifies how an update handles tagged,
// untagged, pinned, and invalid image references, and how it reports failed
// and rate-limited staleness checks.
func TestUpdateAction_ImageReferences(t *testing.T) {
	t.Parallel()

	// Tests for image reference handling to cover isPinned functionality
	t.Run("handling different image reference formats", func(t *testing.T) {
		t.Parallel()

		// config is the update configuration every case runs with.
		config := types.UpdateParams{
			Cleanup: true,
			Filter:  filters.NoFilter,
		}

		t.Run("should process tagged images and update if stale", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Tagged container should be scanned")
			require.Len(t, report.Updated(), 1, "Tagged container should be updated if stale")
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("image:1.0.0") }))
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called")
		})

		t.Run("should process untagged images and update if stale", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Untagged container should be scanned")
			require.Len(t, report.Updated(), 1, "Untagged container should be updated if stale")
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID("image") }))
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called")
		})

		t.Run("should skip pinned containers and not collect image IDs", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Pinned container should be scanned")
			assert.Empty(t, report.Updated(), "Pinned container should not be updated")
			assert.Empty(t, cleanupImageInfos, "No image IDs should be collected for pinned container")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called")
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called")
		})

		t.Run("should skip bare sha256-pinned containers and not call IsContainerStale", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Bare digest container should be scanned")
			assert.Empty(t, report.Updated(), "Bare digest container should not be updated")
			assert.Empty(t, cleanupImageInfos)
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called for bare digest pin")
		})

		t.Run("should skip pinned containers with tag and digest and not collect image IDs", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Pinned container should be scanned")
			assert.Empty(t, report.Updated(), "Pinned container should not be updated")
			assert.Empty(t, cleanupImageInfos, "No image IDs should be collected for pinned container")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called")
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called")
		})

		t.Run("should process invalid image references with fallback", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Scanned(), 1, "Invalid container should be scanned with fallback")
			require.Len(t, report.Updated(), 1, "Invalid container should be updated with valid fallback")
			assert.True(t, slices.ContainsFunc(cleanupImageInfos, func(info types.RemovedImageInfo) bool { return info.ImageID == types.ImageID(":latest") }))
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called")
		})

		t.Run("should skip containers with missing Config.Image and imageInfo.ID with error", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Skipped(), 1, "Container with missing image info should be skipped")
			assert.Empty(t, report.Scanned(), "Container should not be scanned")
			assert.Empty(t, report.Updated(), "Container should not be updated")
			assert.Empty(t, cleanupImageInfos, "No image IDs should be collected")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called")
			assert.Equal(t, int32(1), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called")
		})

		// This test differs from the previous one ("should process invalid image references with fallback")
		// in that the container name "InvalidContainer" contains uppercase characters, making the fallback
		// image "InvalidContainer:latest" invalid since Docker image names must be lowercase.
		// In contrast, "invalid-container:latest" is valid.
		t.Run("should skip containers with invalid fallback image reference", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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
			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Skipped(), 1, "Container with invalid fallback image should be skipped")
			assert.Empty(t, report.Scanned(), "Container should not be scanned")
			assert.Empty(t, report.Updated(), "Container should not be updated")
			assert.Empty(t, cleanupImageInfos, "No image IDs should be collected")
			assert.Equal(t, int32(0), client.TestData.TriedToRemoveImageCount.Load(), "RemoveImageByID should not be called")
			assert.Equal(t, int32(0), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should not be called")
		})

		// Verify that failures across parallel staleness checks are all counted
		// in the final report when IsContainerStale returns an error for every container.
		t.Run("should count all parallel staleness failures in the final report", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Skipped(), 3, "All three containers should be skipped when IsContainerStale fails")
			assert.Empty(t, report.Scanned(), "No containers should be scanned when all fail")
			assert.Empty(t, report.Updated(), "No containers should be updated when IsContainerStale fails")
			assert.Empty(t, cleanupImageInfos, "No image IDs should be collected")
			assert.Equal(t, int32(3), client.TestData.IsContainerStaleCount.Load(), "IsContainerStale should be called for all three containers")
		})

		t.Run("should count exhausted registry rate limits as failed", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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

			report, cleanupImageInfos, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Failed(), 1, "Rate-limited staleness checks should be failed")
			assert.Empty(t, report.Skipped(), "Rate-limited staleness checks should not be skipped")
			assert.Empty(t, report.Updated())
			assert.Empty(t, cleanupImageInfos)
		})

		t.Run("logs one notifiable warning per rate-limited container", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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

			report, _, err := update(&log, t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Failed(), 1)

			assert.Equal(t, 1, strings.Count(logs.String(), "Registry rate limit retries exhausted"), "One warning per failed container")

			var warning string

			for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
				if strings.Contains(line, "Registry rate limit retries exhausted") {
					warning = line
				}
			}

			assert.Contains(t, warning, `"level":"warn"`)
			assert.NotContains(t, warning, `"notify":"no"`, "The warning must reach log-based notifications")
			assert.Contains(t, warning, "rate-limited-container")
		})

		t.Run("leaves a long Retry-After to the retry loop's warning", func(t *testing.T) {
			t.Parallel()

			client := &mockActions.MockClient{
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

			report, _, err := update(&log, t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Failed(), 1)
			assert.NotContains(t, logs.String(), "Registry rate limit retries exhausted", "The retry loop already warned about a Retry-After beyond the honor window")
		})
	})
}
