package events

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBroadcaster(t *testing.T) {
	b := NewBroadcaster()
	require.NotNil(t, b)
	assert.Equal(t, 0, b.SubscriberCount())
}

func TestUnsubscribe(t *testing.T) {
	b := NewBroadcaster()

	ch, _ := b.SubscribeWithDone()
	assert.Equal(t, 1, b.SubscriberCount())

	b.Unsubscribe(ch)
	assert.Equal(t, 0, b.SubscriberCount())
}

func TestPublish(t *testing.T) {
	b := NewBroadcaster()

	ch, _ := b.SubscribeWithDone()

	event := Event{
		Type:      "scan_completed",
		Timestamp: time.Now().UTC(),
		Data:      map[string]int{"scanned": 5},
	}

	b.Publish(event)

	select {
	case received := <-ch:
		assert.Equal(t, event.Type, received.Type)
		assert.Equal(t, event.Data, received.Data)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestPublishMultipleSubscribers(t *testing.T) {
	b := NewBroadcaster()

	ch1, _ := b.SubscribeWithDone()
	ch2, _ := b.SubscribeWithDone()

	event := Event{Type: "scan_started", Timestamp: time.Now().UTC()}
	b.Publish(event)

	select {
	case received := <-ch1:
		assert.Equal(t, event.Type, received.Type)
	case <-time.After(time.Second):
		t.Fatal("subscriber 1 timed out")
	}

	select {
	case received := <-ch2:
		assert.Equal(t, event.Type, received.Type)
	case <-time.After(time.Second):
		t.Fatal("subscriber 2 timed out")
	}
}

func TestPublishBackpressure(t *testing.T) {
	b := NewBroadcaster()

	ch, _ := b.SubscribeWithDone()

	for range subscriberChannelSize + 5 {
		b.Publish(Event{Type: "test", Timestamp: time.Now().UTC()})
	}

	count := 0
	drainDone := time.After(100 * time.Millisecond)

	for {
		select {
		case <-ch:
			count++
		case <-drainDone:
			goto done
		}
	}

done:
	assert.LessOrEqual(t, count, subscriberChannelSize)
}

func TestUnsubscribeNonExistent(t *testing.T) {
	b := NewBroadcaster()

	ch := make(chan Event)
	found := b.Unsubscribe(ch)
	assert.False(t, found, "unsubscribing non-existent channel should return false")
	assert.Equal(t, 0, b.SubscriberCount())
}

func TestUnsubscribeIdempotent(t *testing.T) {
	b := NewBroadcaster()

	ch, _ := b.SubscribeWithDone()
	assert.Equal(t, 1, b.SubscriberCount())

	found1 := b.Unsubscribe(ch)
	assert.True(t, found1, "first unsubscribe should return true")

	found2 := b.Unsubscribe(ch)
	assert.False(t, found2, "second unsubscribe should return false")
	assert.Equal(t, 0, b.SubscriberCount())
}

func TestSubscribeWithDone(t *testing.T) {
	b := NewBroadcaster()

	ch, done := b.SubscribeWithDone()
	require.NotNil(t, ch)
	require.NotNil(t, done)
	assert.Equal(t, 1, b.SubscriberCount())

	b.Unsubscribe(ch)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("done channel should be closed after unsubscribe")
	}
}

func TestPublishIgnoresUnsubscribed(t *testing.T) {
	b := NewBroadcaster()

	ch1, _ := b.SubscribeWithDone()
	ch2, _ := b.SubscribeWithDone()

	b.Unsubscribe(ch1)

	for range subscriberChannelSize + 5 {
		b.Publish(Event{Type: "test", Timestamp: time.Now().UTC()})
	}

	select {
	case <-ch2:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ch2 should have received events")
	}
}
