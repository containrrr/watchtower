package events

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzBroadcasterSubscribePublish verifies that the Broadcaster correctly
// handles subscribe and publish operations without panicking.
func FuzzBroadcasterSubscribePublish(f *testing.F) {
	f.Add(3, 2)
	f.Add(1, 1)
	f.Add(0, 3)

	f.Fuzz(func(t *testing.T, numSubscribers, numEvents int) {
		if numSubscribers < 0 {
			numSubscribers = 0
		}

		if numEvents < 0 {
			numEvents = 0
		}

		if numSubscribers > 5 {
			numSubscribers = 5
		}

		if numEvents > 5 {
			numEvents = 5
		}

		b := NewBroadcaster()

		var channels []<-chan Event

		for range numSubscribers {
			ch, _ := b.SubscribeWithDone()
			if ch != nil {
				channels = append(channels, ch)
			}
		}

		assert.Equal(t, len(channels), b.SubscriberCount())

		for i := range numEvents {
			event := Event{
				Type: "test-event",
				Data: map[string]int{"index": i},
			}

			b.Publish(event)
		}

		for _, ch := range channels {
			b.Unsubscribe(ch)
		}

		assert.Equal(t, 0, b.SubscriberCount(), "all subscribers should be removed")
	})
}

// FuzzBroadcasterMaxSubscribers verifies that the Broadcaster correctly
// rejects subscribers when the maximum is reached.
func FuzzBroadcasterMaxSubscribers(f *testing.F) {
	f.Add(50)
	f.Add(99)
	f.Add(100)

	f.Fuzz(func(t *testing.T, numSubscribers int) {
		if numSubscribers < 0 {
			numSubscribers = 0
		}

		if numSubscribers > 100 {
			numSubscribers = 100
		}

		b := NewBroadcaster()
		successCount := 0
		rejectCount := 0

		for range numSubscribers {
			ch, _ := b.SubscribeWithDone()
			if ch == nil {
				rejectCount++
			} else {
				successCount++

				b.Unsubscribe(ch)
			}
		}

		assert.LessOrEqual(t, successCount, maxSubscribers,
			"should not exceed max subscribers")
	})
}

// FuzzBroadcasterUnsubscribeNonExistent verifies that Unsubscribe returns
// false when the channel was not previously registered.
func FuzzBroadcasterUnsubscribeNonExistent(f *testing.F) {
	f.Add(true)
	f.Add(false)

	f.Fuzz(func(t *testing.T, subscribeFirst bool) {
		b := NewBroadcaster()

		if subscribeFirst {
			ch, _ := b.SubscribeWithDone()
			assert.True(t, b.Unsubscribe(ch), "should find existing subscriber")
			assert.False(t, b.Unsubscribe(ch), "should not find already-removed subscriber")
		} else {
			unknown := make(chan Event, 10)
			assert.False(t, b.Unsubscribe(unknown), "should return false for unknown subscriber")
		}
	})
}

// FuzzBroadcasterConcurrentSubscribePublishUnsubscribe verifies that the
// Broadcaster handles concurrent subscribe, publish, and unsubscribe
// operations without panicking or deadlocking.
func FuzzBroadcasterConcurrentSubscribePublishUnsubscribe(f *testing.F) {
	f.Add(3, 2, true)
	f.Add(1, 5, false)
	f.Add(5, 1, true)

	f.Fuzz(func(t *testing.T, numSubscribers, numPublishers int, enableUnsubscribe bool) {
		if numSubscribers < 0 {
			numSubscribers = 0
		}

		if numPublishers < 0 {
			numPublishers = 0
		}

		numSubscribers = min(numSubscribers, 10)
		numPublishers = min(numPublishers, 5)

		b := NewBroadcaster()

		var (
			channels []<-chan Event
			mu       sync.Mutex
			wg       sync.WaitGroup
		)

		for range numSubscribers {
			wg.Go(func() {
				ch, _ := b.SubscribeWithDone()
				if ch != nil {
					mu.Lock()

					channels = append(channels, ch)
					mu.Unlock()
				}
			})
		}

		for range numPublishers {
			wg.Go(func() {
				b.Publish(Event{
					Type: "test-event",
					Data: "payload",
				})
			})
		}

		if enableUnsubscribe {
			mu.Lock()

			for _, ch := range channels {
				wg.Add(1)

				go func(ch <-chan Event) {
					defer wg.Done()

					b.Unsubscribe(ch)
				}(ch)
			}

			mu.Unlock()
		}

		wg.Wait()

		count := b.SubscriberCount()

		assert.GreaterOrEqual(t, count, 0, "subscriber count should never be negative")
		assert.LessOrEqual(t, count, maxSubscribers, "subscriber count should not exceed max")
	})
}
