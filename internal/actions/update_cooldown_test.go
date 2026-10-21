package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container"
	"github.com/nicholas-fedor/watchtower/pkg/filters"
	"github.com/nicholas-fedor/watchtower/pkg/registry/ratelimit"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// cooldownConfig returns a base UpdateParams with CooldownDelay and standard settings.
func cooldownConfig(cooldownDelay time.Duration) types.UpdateParams {
	return types.UpdateParams{
		Cleanup:       true,
		Filter:        filters.NoFilter,
		CooldownDelay: cooldownDelay,
	}
}

// TestUpdateAction_Cooldown verifies that an update skips a container whose
// image is in its cooldown or whose image age is unknown, and updates it once
// the cooldown has passed, when the cooldown is zero, or when pulls are off.
// Monitor-only mode updates nothing.
func TestUpdateAction_Cooldown(t *testing.T) {
	t.Parallel()

	t.Run("CooldownDelay is set and image is newer than cooldown", func(t *testing.T) {
		t.Parallel()

		t.Run("should defer the update and skip the container", func(t *testing.T) {
			t.Parallel()

			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "myimage:latest", time.Now()),
					},
					IsContainerStaleError: container.ErrImageCooldown,
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := update(newTestLogger(t), t.Context(), client, cooldownConfig(1*time.Hour), ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Skipped(), 1, "Container should be skipped due to cooldown")
			assert.Empty(t, report.Scanned(), "Container should not be scanned when deferred by cooldown")
			assert.Empty(t, report.Updated(), "Container should not be updated when deferred by cooldown")
		})
	})

	t.Run("CooldownDelay is set and image is older than cooldown", func(t *testing.T) {
		t.Parallel()

		t.Run("should proceed with the update normally", func(t *testing.T) {
			t.Parallel()

			// The client decides cooldown during its staleness check, so an image
			// past its cooldown reports stale without an error.
			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "myimage:latest", time.Now()),
					},
					Staleness: map[string]bool{
						"c1": true,
					},
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := update(newTestLogger(t), t.Context(), client, cooldownConfig(1*time.Hour), ratelimit.New())

			require.NoError(t, err)
			assert.Empty(t, report.Skipped(), "Container should not be skipped when cooldown has elapsed")
			require.Len(t, report.Updated(), 1, "Container should be updated after cooldown passes")
		})
	})

	t.Run("CooldownDelay is zero", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip the cooldown check entirely", func(t *testing.T) {
			t.Parallel()

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

			report, _, err := update(newTestLogger(t), t.Context(), client, cooldownConfig(0), ratelimit.New())

			require.NoError(t, err)
			assert.Empty(t, report.Skipped(), "Container should not be skipped when cooldown is disabled")
			require.Len(t, report.Updated(), 1, "Container should be updated when cooldown is disabled")
		})
	})

	t.Run("NoPull is enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip the cooldown check", func(t *testing.T) {
			t.Parallel()

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

			report, _, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			assert.Empty(t, report.Skipped(), "Container should not be skipped when NoPull is set")
			require.Len(t, report.Updated(), 1, "Container should be updated when NoPull bypasses cooldown")
		})
	})

	t.Run("image age fetch fails", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip the container with a conservative posture", func(t *testing.T) {
			t.Parallel()

			client := mockActions.MockClient{
				TestData: &mockActions.TestData{
					Containers: []types.Container{
						mockActions.CreateMockContainer("c1", "c1", "myimage:latest", time.Now()),
					},
					IsContainerStaleError: container.ErrImageCooldown,
				},
				Stopped: make(map[string]bool),
			}

			report, _, err := update(newTestLogger(t), t.Context(), client, cooldownConfig(1*time.Hour), ratelimit.New())

			require.NoError(t, err)
			require.Len(t, report.Skipped(), 1, "Container should be skipped when image age fetch fails")
			assert.Empty(t, report.Updated(), "Container should not be updated when image age fetch fails")
		})
	})

	t.Run("MonitorOnly is enabled", func(t *testing.T) {
		t.Parallel()

		t.Run("should skip the cooldown check entirely", func(t *testing.T) {
			t.Parallel()

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

			report, _, err := update(newTestLogger(t), t.Context(), client, config, ratelimit.New())

			require.NoError(t, err)
			assert.Empty(t, report.Skipped(), "Container should not be skipped when MonitorOnly is set")
			assert.Empty(t, report.Updated(), "Monitor-only container should not be updated")
		})
	})
}
