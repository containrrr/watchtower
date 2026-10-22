package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := Event{
		Type:      "scan_started",
		Timestamp: timeFromMillis(1_234_567_890),
		Data:      ScanStartedData{Cleanup: true},
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Event
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, original.Type, decoded.Type)
	assert.True(t, original.Timestamp.Equal(decoded.Timestamp))

	data, ok := decoded.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, data["cleanup"])
}

func TestScanCompletedData_JSON(t *testing.T) {
	t.Parallel()

	data := ScanCompletedData{Scanned: 23, Updated: 0, Failed: 0, Skipped: 5}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	assert.JSONEq(t, `{"scanned":23,"updated":0,"failed":0,"skipped":5}`, string(raw))
}

func TestScanFailedData_JSON(t *testing.T) {
	t.Parallel()

	data := ScanFailedData{Error: "container not found"}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	assert.JSONEq(t, `{"error":"container not found"}`, string(raw))
}

func TestImageCleanupData_JSON(t *testing.T) {
	t.Parallel()

	data := ImageCleanupData{
		Images: []ImageCleanupEntry{
			{ImageID: "img1", ImageName: "app:1", ContainerID: "c1", ContainerName: "c1"},
		},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)

	var decoded ImageCleanupData
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Images, 1)
	assert.Equal(t, "img1", decoded.Images[0].ImageID)
}

func TestBroadcaster_PublishDeliversToAllSubscribers(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()
	ch1, _ := broadcaster.SubscribeWithDone()
	ch2, _ := broadcaster.SubscribeWithDone()

	require.NotNil(t, ch1)
	require.NotNil(t, ch2)

	broadcaster.Publish(Event{Type: "scan_completed", Timestamp: timeFromMillis(0), Data: ScanCompletedData{Scanned: 3, Updated: 1, Failed: 0}})

	msg1 := <-ch1
	msg2 := <-ch2

	assert.Equal(t, "scan_completed", msg1.Type)
	assert.Equal(t, "scan_completed", msg2.Type)
}

func TestBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()
	ch, _ := broadcaster.SubscribeWithDone()
	require.NotNil(t, ch)

	require.True(t, broadcaster.Unsubscribe(ch))

	broadcaster.Publish(Event{Type: "lost", Timestamp: timeFromMillis(0), Data: ScanCompletedData{Scanned: 1}})

	select {
	case <-ch:
		t.Fatal("subscriber received event after unsubscribe")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestBroadcaster_PublishToEmptyBroadcaster(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()

	assert.NotPanics(t, func() {
		broadcaster.Publish(Event{Type: "noop", Timestamp: timeFromMillis(0), Data: ScanCompletedData{Scanned: 0}})
	})
}

func TestBroadcaster_SubscriberCount(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()

	require.Equal(t, 0, broadcaster.SubscriberCount())

	ch1, _ := broadcaster.SubscribeWithDone()
	require.NotNil(t, ch1)
	assert.Equal(t, 1, broadcaster.SubscriberCount())

	ch2, _ := broadcaster.SubscribeWithDone()
	require.NotNil(t, ch2)
	assert.Equal(t, 2, broadcaster.SubscriberCount())

	broadcaster.Unsubscribe(ch1)
	assert.Equal(t, 1, broadcaster.SubscriberCount())

	broadcaster.Unsubscribe(ch2)
	assert.Equal(t, 0, broadcaster.SubscriberCount())
}

func TestBroadcaster_MaxSubscribers(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()

	for i := range maxSubscribers {
		ch, _ := broadcaster.SubscribeWithDone()
		require.NotNil(t, ch, "subscriber %d", i)
	}

	overflow, _ := broadcaster.SubscribeWithDone()
	assert.Nil(t, overflow, "subscriber over max")
}

func TestBroadcaster_SubscribeAndUnsubscribeRoundTrips(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()

	ch, _ := broadcaster.SubscribeWithDone()
	require.NotNil(t, ch)
	assert.Equal(t, 1, broadcaster.SubscriberCount())

	broadcaster.Unsubscribe(ch)
	assert.Equal(t, 0, broadcaster.SubscriberCount())

	// Unsubscribing again should be a no-op (idempotent).
	assert.False(t, broadcaster.Unsubscribe(ch))
}

func TestBroadcaster_SubscribeWithDoneAndUnsubscribe(t *testing.T) {
	t.Parallel()

	broadcaster := NewBroadcaster()
	ch, done := broadcaster.SubscribeWithDone()
	require.NotNil(t, ch)
	require.NotNil(t, done)
	assert.Equal(t, 1, broadcaster.SubscriberCount())

	broadcaster.Publish(Event{Type: "hello", Timestamp: timeFromMillis(0), Data: ScanCompletedData{Scanned: 1}})

	msg := <-ch
	assert.Equal(t, "hello", msg.Type)

	broadcaster.Unsubscribe(ch)
	assert.Equal(t, 0, broadcaster.SubscriberCount())

	// Unsubscribe closes sub.done; the done channel should reflect that.
	select {
	case <-done:
	case <-time.After(30 * time.Millisecond):
		t.Fatal("done channel not closed after unsubscribe")
	}
}

// timeFromMillis returns a UTC time.Time from a unix-millisecond value, keeping
// these tests timezone-independent.
func timeFromMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}
