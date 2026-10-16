package actions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dockerContainer "github.com/moby/moby/api/types/container"

	mockActions "github.com/nicholas-fedor/watchtower/internal/actions/mocks"
	"github.com/nicholas-fedor/watchtower/internal/api/handlers/events"
	"github.com/nicholas-fedor/watchtower/internal/metrics"
	mockContainer "github.com/nicholas-fedor/watchtower/pkg/container/mocks"
	"github.com/nicholas-fedor/watchtower/pkg/container/oci"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// TestRunUpdatesWithNotifications_UpdateErrorKeepsPartialResults verifies
// that work finished before an update error is still reported and cleaned up.
// A rolling restart is canceled after the first of two containers has been
// replaced: the notification and the metric carry the partial report, the
// replaced container's old image is removed, and the scan still ends with
// scan_failed.
func TestRunUpdatesWithNotifications_UpdateErrorKeepsPartialResults(t *testing.T) {
	t.Parallel()

	const newID = types.ContainerID("e000000000000000000000000000000000000000000000000000000000000000")

	// appContainer returns a running container that uses the given image.
	appContainer := func(id, name, image string) types.Container {
		return mockActions.CreateMockContainerWithConfig(
			id,
			name,
			image,
			true,
			false,
			time.Now(),
			&dockerContainer.Config{Image: image},
		)
	}

	first := appContainer("a000000000000000000000000000000000000000000000000000000000000000", "/app-a", "org/app-a:1")
	second := appContainer("b000000000000000000000000000000000000000000000000000000000000000", "/app-b", "org/app-b:1")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := mockContainer.NewMockClient(t)
	client.EXPECT().ListContainers(mock.Anything, mock.Anything).
		Return([]types.Container{first, second}, nil).Once()
	client.EXPECT().IsContainerStale(mock.Anything, mock.Anything, mock.Anything).
		Return(true, types.ImageID("sha256:new"), "", nil).Times(2)
	client.EXPECT().GetImageAnnotations(mock.Anything, mock.Anything).Return(oci.Annotations{})

	// Only one container is replaced. The session is canceled while waiting
	// for the replacement to become healthy, so the rolling restart stops
	// before the other container.
	client.EXPECT().StopAndRemoveContainer(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	client.EXPECT().CreateContainer(mock.Anything, mock.Anything).Return(newID, nil).Once()
	client.EXPECT().StartContainerByID(mock.Anything, newID).Return(nil).Once()
	client.EXPECT().WaitForContainerHealthy(mock.Anything, newID, mock.Anything).
		RunAndReturn(func(context.Context, types.ContainerID, time.Duration) error {
			cancel()

			return nil
		}).Once()

	var removed []types.ImageID

	client.EXPECT().RemoveImageByID(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, id types.ImageID, _ string) {
			removed = append(removed, id)
		}).
		Return(nil)

	var sent []types.Report

	notifier := mockTypes.NewMockNotifier(t)
	notifier.EXPECT().StartNotification(false).Once()
	notifier.EXPECT().SendNotification(mock.Anything).
		Run(func(report types.Report) { sent = append(sent, report) }).
		Once()

	broadcaster := events.NewBroadcaster()
	published, _ := broadcaster.SubscribeWithDone()

	metric := RunUpdatesWithNotifications(ctx, RunUpdatesWithNotificationsParams{
		Logger:           testLogger(),
		Client:           client,
		Notifier:         notifier,
		EventBroadcaster: broadcaster,
		Update:           types.UpdateParams{RollingRestart: true, Cleanup: true},
	})

	assert.Equal(t, &metrics.Metric{Scanned: 1, Updated: 1, Skipped: 1, ScanFailed: true}, metric)

	require.Len(t, sent, 1, "one notification is sent")
	require.Len(t, sent[0].Updated(), 1, "the notification reports the replaced container")
	assert.Len(t, sent[0].Skipped(), 1, "the notification reports the container left unprocessed as skipped")
	assert.Empty(t, sent[0].Failed())

	updated := sent[0].Updated()[0]
	assert.Equal(t, []types.ImageID{updated.CurrentImageID()}, removed,
		"the replaced container's old image is removed")

	// Events are published synchronously into the subscriber's buffer.
	var eventTypes []string

	for len(published) > 0 {
		eventTypes = append(eventTypes, (<-published).Type)
	}

	assert.Equal(t, []string{"scan_started", "image_cleanup", "scan_failed"}, eventTypes)
}
