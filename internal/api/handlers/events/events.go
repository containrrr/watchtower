package events

import (
	"sync"
	"time"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	// subscriberChannelSize is the buffer size for each subscriber channel.
	subscriberChannelSize = 10
	// maxSubscribers is the maximum number of concurrent SSE subscribers.
	maxSubscribers = 100
)

// Event represents a Watchtower operational event emitted to SSE subscribers.
type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// ScanStartedData carries redacted scan policy flags for SSE subscribers.
// It intentionally omits filter functions, container IDs, and other internal
// fields from types.UpdateParams.
type ScanStartedData struct {
	Cleanup             bool `json:"cleanup"`
	NoRestart           bool `json:"no_restart"`
	MonitorOnly         bool `json:"monitor_only"`
	LifecycleHooks      bool `json:"lifecycle_hooks"`
	RollingRestart      bool `json:"rolling_restart"`
	LabelPrecedence     bool `json:"label_precedence"`
	NoPull              bool `json:"no_pull"`
	RunOnce             bool `json:"run_once"`
	UseComposeDependsOn bool `json:"use_compose_depends_on"`
	SkipSelfUpdate      bool `json:"skip_self_update"`
	EphemeralSelfUpdate bool `json:"ephemeral_self_update"`
	ReviveStopped       bool `json:"revive_stopped"`
}

// NewScanStartedData creates a redacted scan_started event payload from the
// full update configuration. It intentionally omits filter functions, container
// IDs, durations, and other internal fields from types.UpdateParams.
func NewScanStartedData(updateParams types.UpdateParams) ScanStartedData {
	return ScanStartedData{
		Cleanup:             updateParams.Cleanup,
		NoRestart:           updateParams.NoRestart,
		MonitorOnly:         updateParams.MonitorOnly,
		LifecycleHooks:      updateParams.LifecycleHooks,
		RollingRestart:      updateParams.RollingRestart,
		LabelPrecedence:     updateParams.LabelPrecedence,
		NoPull:              updateParams.NoPull,
		RunOnce:             updateParams.RunOnce,
		UseComposeDependsOn: updateParams.UseComposeDependsOn,
		SkipSelfUpdate:      updateParams.SkipSelfUpdate,
		EphemeralSelfUpdate: updateParams.EphemeralSelfUpdate,
		ReviveStopped:       updateParams.ReviveStopped,
	}
}

// ScanCompletedData carries details about a scan that has finished.
type ScanCompletedData struct {
	Scanned int `json:"scanned"`
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// ScanFailedData carries details about a scan that encountered an error.
type ScanFailedData struct {
	Error string `json:"error"`
}

// ImageCleanupData carries details about images cleaned up after a scan.
type ImageCleanupData struct {
	Images []ImageCleanupEntry `json:"images"`
}

// ImageCleanupEntry represents a single cleaned-up image in an event.
type ImageCleanupEntry struct {
	ImageID       string `json:"image_id"`
	ImageName     string `json:"image_name"`
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
}

// subscriber represents a single SSE subscriber with an event channel and a
// done channel that is closed when the subscriber is unsubscribed.
type subscriber struct {
	ch   chan Event
	done chan struct{}
}

// Broadcaster manages SSE subscriber registration and event distribution.
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers []*subscriber
}

// NewBroadcaster creates a new event broadcaster for distributing Watchtower
// operational events to SSE subscribers.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make([]*subscriber, 0, maxSubscribers),
	}
}

// SubscribeWithDone registers a new subscriber and returns both the event channel
// and a done channel. The done channel is closed when the subscriber is unsubscribed,
// allowing the receiver to detect unsubscription even if no more events are sent.
// Returns nil, nil if the maximum number of subscribers has been reached.
func (b *Broadcaster) SubscribeWithDone() (<-chan Event, <-chan struct{}) {
	subCh := make(chan Event, subscriberChannelSize)
	done := make(chan struct{})

	b.mu.Lock()
	if len(b.subscribers) >= maxSubscribers {
		b.mu.Unlock()
		close(subCh)
		close(done)

		return nil, nil
	}

	b.subscribers = append(b.subscribers, &subscriber{ch: subCh, done: done})
	b.mu.Unlock()

	return subCh, done
}

// Unsubscribe removes a subscriber and signals its goroutine to stop by closing
// the done channel. Returns true if the subscriber was found and removed, false
// if it was not found (which may indicate it was already unsubscribed).
func (b *Broadcaster) Unsubscribe(ch <-chan Event) bool {
	b.mu.Lock()

	for i, sub := range b.subscribers {
		if sub.ch == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			b.mu.Unlock()

			close(sub.done)

			return true
		}
	}

	b.mu.Unlock()

	return false
}

// Publish sends an event to all registered subscribers.
// Subscribers that are full (backpressure) or have been unsubscribed have their
// event dropped.
func (b *Broadcaster) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		select {
		case sub.ch <- event:
		case <-sub.done:
		default:
		}
	}
}

// SubscriberCount returns the current number of active subscribers.
func (b *Broadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.subscribers)
}
