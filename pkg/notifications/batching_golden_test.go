package notifications

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/testutil/golden"
	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

// batchEntries returns the entries one update session queues when two
// containers, web1 and web2, share an image and a third, db, uses its own.
// Each entry carries the fields its emitting code attaches.
func batchEntries() []*notificationEntry {
	entry := func(message string, data map[string]any) *notificationEntry {
		return &notificationEntry{Message: message, Data: data, Time: goldenEntryTime, Level: "info"}
	}

	sharedImage := map[string]any{"image": "org/web:1.0", "new_id": "aaa111111111"}

	return []*notificationEntry{
		entry("Only checking containers in scope", map[string]any{"scope": "production"}),
		entry("Image is within cooldown period - not eligible for update",
			map[string]any{"image": "org/web:1.0", "cooldown": "1d", "eligible_in": "2h", "eligible_at": "2026-10-07T14:00:00Z"}),
		entry("Image is within cooldown period - not eligible for update",
			map[string]any{"image": "org/web:1.0", "cooldown": "1d", "eligible_in": "2h", "eligible_at": "2026-10-07T14:00:00Z"}),
		entry("Found new image", withContainer(sharedImage, "web1")),
		entry("Found new image", withContainer(sharedImage, "web2")),
		entry("Found new image", map[string]any{"container": "db", "image": "org/db:2", "new_id": "bbb222222222"}),
		entry("Found new Git revision",
			map[string]any{"container": "web1", "revision": "github.com/org/web@v1.1", "short_commit": "0123456"}),
		entry("Found new Git revision",
			map[string]any{"container": "web2", "revision": "github.com/org/web@v1.1", "short_commit": "0123456"}),
		entry("Stopping container", map[string]any{"container": "web1", "id": "111111111111"}),
		entry("Stopping container", map[string]any{"container": "web2", "id": "222222222222"}),
		entry("Started new container", map[string]any{"container": "web1", "new_id": "aaa111111111"}),
		entry("Started new container", map[string]any{"container": "web2", "new_id": "aaa111111111"}),
		entry("Removing image",
			map[string]any{"container_name": "web1", "image_name": "org/web:1.0", "image_id": "01d111111111"}),
		entry("Removing image",
			map[string]any{"container_name": "web2", "image_name": "org/web:1.0", "image_id": "01d111111111"}),
	}
}

// withContainer returns a copy of data with the container field set.
func withContainer(data map[string]any, container string) map[string]any {
	out := map[string]any{"container": container}
	maps.Copy(out, data)

	return out
}

// newBatchingNotifier returns a legacy-template notifier whose worker never
// starts, so every message it renders stays in its message channel, with
// entries already queued as during a batch.
func newBatchingNotifier(t *testing.T, entries []*notificationEntry) *shoutrrrTypeNotifier {
	t.Helper()

	notifier := createTestNotifier(newTestLogger(t), []string{}, zerolog.TraceLevel, true, StaticData{}, false, 0)
	t.Cleanup(notifier.Close)

	notifier.entries = entries

	return notifier
}

// sentMessages drains the messages the notifier rendered.
func sentMessages(notifier *shoutrrrTypeNotifier) []string {
	var messages []string

	for {
		select {
		case msg := <-notifier.messages:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// writeMessages appends each message to out under a numbered heading.
func writeMessages(out *strings.Builder, messages []string) {
	for i, msg := range messages {
		fmt.Fprintf(out, "--- message %d\n%s\n", i+1, strings.TrimRight(msg, "\n"))
	}
}

// TestNotificationBatching snapshots the messages sent for one batch of queued
// entries in each sending mode: grouped, where entries repeated across
// containers sharing an image are removed; focused on a single container's
// report, where only that container's entries are sent; and split by
// container, where each container gets its own message without removal.
func TestNotificationBatching(t *testing.T) {
	var out strings.Builder

	out.WriteString("=== grouped\n")

	grouped := newBatchingNotifier(t, batchEntries())
	grouped.SendNotification(nil)
	writeMessages(&out, sentMessages(grouped))

	out.WriteString("\n=== focused on web1\n")

	focus := mockTypes.NewMockContainerReport(t)
	focus.EXPECT().Name().Return("web1").Maybe()
	focus.EXPECT().ImageName().Return("org/web:1.0").Maybe()

	focused := newBatchingNotifier(t, batchEntries())
	focused.SendNotification(&session.SingleContainerReport{UpdatedReports: []types.ContainerReport{focus}})
	writeMessages(&out, sentMessages(focused))

	out.WriteString("--- still queued\n")

	for _, entry := range focused.entries {
		fmt.Fprintf(&out, "%s %v\n", entry.Message, entry.Data)
	}

	// The messages are sent in map order, so they are sorted for the snapshot.
	out.WriteString("\n=== split by container (sorted)\n")

	split := newBatchingNotifier(t, batchEntries())
	FlushSplitByContainer(split)

	messages := sentMessages(split)
	slices.Sort(messages)
	writeMessages(&out, messages)

	require.Empty(t, split.entries, "splitting by container empties the queue")

	golden.Assert(t, "notification-batching", []byte(out.String()))
}
