package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"
	"unsafe"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/rs/zerolog"

	shoutrrrTypes "github.com/nicholas-fedor/shoutrrr/pkg/types"

	"github.com/nicholas-fedor/watchtower/pkg/session"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// shoutrrrType is the identifier for Shoutrrr notifications.
const shoutrrrType = "shoutrrr"

// initialEntriesCapacity defines the initial capacity for the entries slice in the Shoutrrr notifier.
//
// It sets a reasonable default for expected log entry batch sizes.
const initialEntriesCapacity = 10

// maxURLLengthForLogging defines the maximum length of URLs displayed in logs to avoid exposing sensitive information.
const maxURLLengthForLogging = 50

// messageChannelBufferSize defines the buffer size for the notification message channel.
const messageChannelBufferSize = 1000

// shutdownGracePeriod defines the time to wait for in-flight messages to complete during shutdown.
// This allows error logging to complete before canceling the context.
const shutdownGracePeriod = 50 * time.Millisecond

// eventFieldRawCapacity is the initial close-brace buffer size for eventFieldMap.
const eventFieldRawCapacity = 256

// eventFieldRawMaxCapacity is the largest pooled close-brace buffer that is reused.
const eventFieldRawMaxCapacity = 8 << 10

// eventFieldRawPool reuses close-brace JSON buffers across hooked log events.
var eventFieldRawPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, eventFieldRawCapacity)

		return &buf
	},
}

// router defines the interface for sending Shoutrrr notifications.
//
// It abstracts the underlying service implementation.
type router interface {
	Send(message string, params *shoutrrrTypes.Params) []error
}

// shoutrrrTypeNotifier manages Shoutrrr notifications.
//
// It handles queuing, templating, and sending with delay.
// Uses mutex for thread-safe access to entries and sync.Once for idempotent operations.
// Implements zerolog.Hook so log events are pushed via Run.
type shoutrrrTypeNotifier struct {
	Urls           []string              // Notification service URLs.
	Router         router                // Router for sending messages.
	entries        []*notificationEntry  // Queued log entries.
	entriesMutex   sync.RWMutex          // Mutex for thread-safe access to entries.
	logLevel       zerolog.Level         // Minimum log level for notifications.
	template       *template.Template    // Template for message formatting.
	messages       chan string           // Channel for message queuing.
	done           chan struct{}         // Signal for send completion.
	stop           chan struct{}         // Channel for stopping the notifier.
	legacyTemplate bool                  // Use legacy log-only template if true.
	params         *shoutrrrTypes.Params // Notification parameters.
	data           StaticData            // Static notification data.
	//nolint:containedctx
	ctx    context.Context    // Context for cancellation.
	cancel context.CancelFunc // Cancel function for the context.
	// These fields must only be accessed via sync/atomic (e.g., atomic.Load/atomic.Store) to avoid data races.
	receiving atomic.Bool   // Tracks if receiving logs.
	delay     time.Duration // Delay between sends.
	hookOnce  sync.Once     // Ensures RegisterHook executes only once.
	closeOnce sync.Once     // Ensures Close executes only once.
	// extractWarnOnce ensures the eventFieldMap failure warning is emitted only once.
	extractWarnOnce sync.Once // Guards the first extraction failure warning.
	// These fields must only be accessed via sync/atomic (e.g., atomic.Load/atomic.Store) to avoid data races.
	closed atomic.Bool // Tracks if the notifier is closed.

	// localLog is a child logger with notify=no for internal logging (loop prevention).
	// Initialized at create time from the process logger and rebound in RegisterHook
	// to a child of the hooked logger. Never a package-level variable.
	// Still inherits hooks. Run short-circuits on notify=no (and fail-closed parse)
	// before taking entriesMutex, so internal logs never re-enter the queue or deadlock.
	// Concurrent application Logs are serialized only by entriesMutex on queue access.
	localLog *zerolog.Logger
}

// getScheme extracts the scheme from a Shoutrrr URL.
//
// Parameters:
//   - url: URL to parse.
//
// Returns:
//   - string: Scheme or "invalid" if none found.
func getScheme(url string) string {
	schemeEnd := strings.Index(url, ":")
	if schemeEnd <= 0 {
		return "invalid"
	}

	return url[:schemeEnd]
}

// GetNames returns service names from URLs.
//
// Returns:
//   - []string: List of schemes from URLs.
func (n *shoutrrrTypeNotifier) GetNames() []string {
	names := make([]string, len(n.Urls))
	for i, u := range n.Urls {
		names[i] = getScheme(u)
	}

	return names
}

// GetURLs returns the configured service URLs.
//
// Returns:
//   - []string: List of URLs.
func (n *shoutrrrTypeNotifier) GetURLs() []string {
	return n.Urls
}

// RegisterHook attaches this notifier as a zerolog.Hook on the given logger instance.
//
// It updates *log in place to the hooked logger so the composition root continues
// using the same pointer for subsequent application logging. A notify=no child of
// the hooked logger is stored for internal loop-safe logging. Registration is
// idempotent via hookOnce.
//
// Parameters:
//   - log: Process logger pointer. Replaced with the hooked instance.
func (n *shoutrrrTypeNotifier) RegisterHook(log *zerolog.Logger) {
	if log == nil {
		return
	}

	n.hookOnce.Do(func() {
		n.receiving.Store(true)

		// Hook returns Logger by value. Write back so the composition root's pointer is hooked.
		hooked := log.Hook(n)
		*log = hooked

		local := log.With().Str("notify", "no").Logger()
		n.localLog = &local

		n.localLog.Debug().
			Strs("urls", redactServiceURLs(n.Urls)).
			Msg("Added Shoutrrr notifier as zerolog hook, starting notification goroutine")

		// Send using a separate goroutine to avoid blocking the main process.
		go sendNotifications(n)
	})
}

// createNotifier initializes a Shoutrrr notifier.
//
// Parameters:
//   - log: Process logger for config-time logs and the Shoutrrr stdlog bridge.
//   - urls: Service URLs.
//   - level: Minimum log level for notifications.
//   - tplString: Template string.
//   - legacy: Use legacy template if true.
//   - data: Static notification data.
//   - stdout: Log to stdout if true.
//   - delay: Delay between sends.
//
// Returns:
//   - *shoutrrrTypeNotifier: Initialized notifier.
func createNotifier(
	log *zerolog.Logger,
	urls []string,
	level zerolog.Level,
	tplString string,
	legacy bool,
	data StaticData,
	stdout bool,
	delay time.Duration,
) *shoutrrrTypeNotifier {
	// Child logger that must never re-enter the notification hook.
	local := log.With().Str("notify", "no").Logger()
	localLog := &local

	// Parse or fallback to default template.
	tpl, err := getShoutrrrTemplate(localLog, tplString, legacy)
	if err != nil {
		localLog.Error().Err(err).
			Msg("Could not use configured notification template, falling back to default")

		// An empty template string selects the built-in default, which always parses.
		tpl, _ = getShoutrrrTemplate(localLog, "", legacy)
	}

	// Set logger based on stdout flag.
	var logger shoutrrrTypes.StdLogger
	if stdout {
		logger = newStdLogger(os.Stdout, "")
	} else {
		// Bridge shoutrrr's stdlib logger into the process zerolog (notify=no).
		logger = newStdLogger(localLog, "Shoutrrr: ")
	}

	// Initialize sender with default options.
	router, err := shoutrrr.NewSenderWithOptions(logger, shoutrrrTypes.SenderOptions{}, urls...)
	if err != nil {
		localLog.Fatal().Err(err).Msg("Failed to initialize Shoutrrr notifications")
	}

	// Set params with title if provided.
	params := &shoutrrrTypes.Params{}
	if data.Title != "" {
		params.SetTitle(data.Title)
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &shoutrrrTypeNotifier{
		Urls:   urls,   // Notification service URLs.
		Router: router, // Router for sending messages.
		messages: make(
			chan string,
			messageChannelBufferSize,
		), // Channel buffer size for notification messages
		done: make(
			chan struct{},
			1,
		), // Signal for send completion.
		stop: make(
			chan struct{},
		), // Channel for stopping the notifier.
		logLevel:       level,                                                 // Minimum log level for notifications.
		template:       tpl,                                                   // Template for message formatting.
		legacyTemplate: legacy,                                                // Use legacy log-only template if true.
		data:           data,                                                  // Static notification data.
		params:         params,                                                // Notification parameters.
		ctx:            ctx,                                                   // Context for cancellation.
		cancel:         cancel,                                                // Cancel function for the context.
		delay:          delay,                                                 // Delay between sends.
		entries:        make([]*notificationEntry, 0, initialEntriesCapacity), // Queued log entries.
		localLog:       localLog,                                              // Loop-safe internal logger.
	}
}

// processSendErrors processes errors from router send operations.
//
// Parameters:
//   - notifier: Notifier instance.
//   - errs: Errors returned from router send.
func processSendErrors(notifier *shoutrrrTypeNotifier, errs []error) {
	log := notifier.ll()
	failureCount := 0

	var authFailures, networkFailures, rateLimitFailures int

	for i, err := range errs {
		// Index guard against potential errs/Urls length mismatch
		if i >= len(notifier.Urls) {
			log.Error().
				Err(err).
				Int("index", i).
				Int("urls_length", len(notifier.Urls)).
				Int("errs_length", len(errs)).
				Str("failure_type", "index_mismatch").
				Msg("Error index out of bounds for URLs slice")

			continue
		}

		// Increment failure count and prepare URL details for logging on notification failure.
		if err != nil {
			failureCount++
			scheme := getScheme(notifier.Urls[i])
			sanitizedURL := sanitizeURLForLogging(notifier.Urls[i])

			// Diagnostic logging: Categorize failure types
			errStr := err.Error()

			errStrLower := strings.ToLower(errStr) // Compute lowercase once for efficiency
			switch {
			case strings.Contains(errStrLower, "unauthorized") ||
				strings.Contains(errStrLower, "authentication") ||
				strings.Contains(errStrLower, "invalid token") ||
				strings.Contains(errStrLower, "invalid api") ||
				strings.Contains(errStrLower, "invalid key") ||
				strings.Contains(errStrLower, "invalid credentials"):
				authFailures++

				log.Warn().
					Err(err).
					Str("service", scheme).
					Int("index", i).
					Str("url", sanitizedURL).
					Str("failure_type", "authentication").
					Msg("Authentication failure detected - check API keys/tokens")
			case strings.Contains(errStrLower, "timeout") ||
				strings.Contains(errStrLower, "connection") ||
				strings.Contains(errStrLower, "network"):
				networkFailures++

				log.Warn().
					Err(err).
					Str("service", scheme).
					Int("index", i).
					Str("url", sanitizedURL).
					Str("failure_type", "network").
					Msg("Network connectivity failure detected - check internet connection")
			case strings.Contains(errStrLower, "rate limit") ||
				strings.Contains(errStrLower, "too many requests"):
				rateLimitFailures++

				log.Warn().
					Err(err).
					Str("service", scheme).
					Int("index", i).
					Str("url", sanitizedURL).
					Str("failure_type", "rate_limit").
					Msg("Rate limiting detected - consider increasing delays or reducing frequency")
			default:
				log.Error().
					Err(err).
					Str("service", scheme).
					Int("index", i).
					Str("url", sanitizedURL).
					Str("failure_type", "unknown").
					Msg("Failed to send shoutrrr notification")
			}
		}
	}

	// Diagnostic logging: Summary with categorized failures
	if failureCount > 0 {
		log.Warn().
			Int("total_urls", len(notifier.Urls)).
			Int("failed_count", failureCount).
			Int("success_count", len(notifier.Urls)-failureCount).
			Int("auth_failures", authFailures).
			Int("network_failures", networkFailures).
			Int("rate_limit_failures", rateLimitFailures).
			Msg("Notification send completed with failures")
	} else if len(notifier.Urls) > 0 {
		log.Debug().
			Int("total_urls", len(notifier.Urls)).
			Msg("Notification send completed successfully")
	}
}

// sendNotifications sends queued messages via the router.
//
// Parameters:
//   - notifier: Notifier instance.
func sendNotifications(notifier *shoutrrrTypeNotifier) {
	log := notifier.ll()
	defer func() { notifier.done <- struct{}{} }()

	for {
		select {
		case msg := <-notifier.messages:
			// Log goroutine receipt of message
			log.Trace().
				Int("msg_length", len(msg)).
				Str("notification_type", shoutrrrType).
				Int("total_urls", len(notifier.Urls)).
				Msg("Notification goroutine received message from channel")

			log.Debug().Str("message", msg).Msg("Sending notification")

			// Only delay if a positive delay is configured.
			// Use a context-aware select to allow interruption when the context is canceled.
			if notifier.delay > 0 {
				timer := time.NewTimer(notifier.delay)

				select {
				case <-timer.C:
					// Delay completed normally - stop the timer
					timer.Stop()
				case <-notifier.ctx.Done():
					// Context canceled - stop the timer and return early
					timer.Stop()
					log.Debug().Msg("Context canceled during delay, skipping send")

					return
				}
			}

			// Diagnostic logging: Log attempt details before sending
			log.Trace().
				Int("total_urls", len(notifier.Urls)).
				Str("delay", notifier.delay.String()).
				Int("msg_length", len(msg)).
				Msg("Attempting to send notification to configured services")

			// Log before calling Router.Send
			log.Trace().
				Int("msg_length", len(msg)).
				Int("total_urls", len(notifier.Urls)).
				Str("notification_type", shoutrrrType).
				Msg("Calling Router.Send with message")

			notifier.send(msg)
		case <-notifier.stop:
			// Shutdown mode: drain all remaining messages from the channel
			log.Debug().Msg("Shutdown signal received, draining remaining messages without delay")

			for {
				select {
				case msg := <-notifier.messages:
					// Log goroutine receipt of message during shutdown
					log.Trace().
						Int("msg_length", len(msg)).
						Str("notification_type", shoutrrrType).
						Int("total_urls", len(notifier.Urls)).
						Bool("shutdown_mode", true).
						Msg("Processing remaining notification message during shutdown")

					log.Debug().Str("message", msg).Msg("Sending notification during shutdown")

					// Skip delay during shutdown to expedite processing

					// Diagnostic logging: Log attempt details before sending
					log.Trace().
						Int("total_urls", len(notifier.Urls)).
						Str("delay", notifier.delay.String()).
						Int("msg_length", len(msg)).
						Msg("Attempting to send notification to configured services during shutdown")

					// Log before calling Router.Send
					log.Trace().
						Int("msg_length", len(msg)).
						Int("total_urls", len(notifier.Urls)).
						Str("notification_type", shoutrrrType).
						Msg("Calling Router.Send with message during shutdown")

					notifier.send(msg)
				default:
					// Channel is empty, all messages drained
					log.Debug().Msg("All remaining messages drained during shutdown")

					return
				}
			}
		case <-notifier.ctx.Done():
			// Context canceled
			log.Debug().Msg("Context canceled, stopping notification goroutine")

			return
		}
	}
}

// StartNotification begins queuing messages for batching.
//
// It sends any existing queued entries before resetting the entries slice to ensure a fresh queue for each run.
// When suppressSummary is true, skip sending queued entries.
// sendEntries is invoked only after releasing entriesMutex so template work and
// internal logging never hold the queue lock (avoids re-entrant hook deadlocks).
func (n *shoutrrrTypeNotifier) StartNotification(suppressSummary bool) {
	n.entriesMutex.Lock()

	var toSend []*notificationEntry
	// Capture residual entries to send after unlock when not suppressed.
	if len(n.entries) > 0 && !suppressSummary {
		toSend = n.entries
	}

	// Capture the count before resetting the slice.
	preResetCount := len(n.entries)

	// Reset the entries slice to an empty slice with initial capacity for new batching.
	n.entries = make([]*notificationEntry, 0, initialEntriesCapacity)

	n.entriesMutex.Unlock()

	if len(toSend) > 0 {
		n.sendEntries(toSend, nil)
	}

	n.ll().Debug().
		Bool("legacy_template", n.legacyTemplate).
		Bool("receiving", n.receiving.Load()).
		Int("entries_count", preResetCount).
		Bool("suppress_summary", suppressSummary).
		Msg("StartNotification called - batching mode enabled")
}

// SendNotification sends queued messages with a report.
//
// When the report focuses on a single container (exactly one container across
// Updated/Restarted, or a single Skipped/Stale focus with no updates), matching
// queued entries are filtered out of the queue and sent with that report. This
// supports --notification-split-by-container without exposing entry state.
// Otherwise the entire queue is drained and sent.
//
// Parameters:
//   - report: Scan report to include.
func (n *shoutrrrTypeNotifier) SendNotification(report types.Report) {
	n.entriesMutex.Lock()

	var entries []*notificationEntry

	focus, ok := singleFocusContainer(report)
	if ok {
		// Keep non-nil empty slices so Run continues treating batching as active
		// when every queued entry matched the focus.
		matched := make([]*notificationEntry, 0)
		rest := make([]*notificationEntry, 0)

		for _, e := range n.entries {
			if entryMatchesFocus(e, focus) {
				matched = append(matched, e)
			} else {
				rest = append(rest, e)
			}
		}

		n.entries = rest
		entries = matched
	} else {
		entries = n.entries
		n.entries = nil // Clear the queue after copying to prevent re-sending
	}

	n.entriesMutex.Unlock()

	n.ll().Debug().
		Int("entries_count", len(entries)).
		Bool("legacy_template", n.legacyTemplate).
		Bool("report_available", report != nil).
		Msg("SendNotification called - sending queued entries and report")

	// Deduplicate entries to prevent repeated messages when multiple containers
	// share the same image (e.g., two containers using nginx:latest will both
	// log "Found new image" but only one notification should be sent).
	entries = deduplicateEntries(entries)

	n.sendEntries(entries, report)
}

// FlushSplitByContainer sends one notification per distinct container value in the
// queued entries, then clears the queue.
//
// Used by the check API split path. This is intentionally a package-level helper
// (not on types.Notifier): the only production Notifier is *shoutrrrTypeNotifier.
// Adding a split method to the interface would force every mock or stub to implement
// it. Non-shoutrrr values fall back to a single SendNotification(nil).
//
// Parameters:
//   - notifier: Active notifier instance (typically *shoutrrrTypeNotifier).
func FlushSplitByContainer(notifier types.Notifier) {
	if notifier == nil {
		return
	}

	shoutrrr, ok := notifier.(*shoutrrrTypeNotifier)
	if !ok {
		notifier.SendNotification(nil)

		return
	}

	shoutrrr.entriesMutex.Lock()
	entries := shoutrrr.entries
	shoutrrr.entries = nil
	shoutrrr.entriesMutex.Unlock()

	if len(entries) == 0 {
		return
	}

	byContainer := make(map[string][]*notificationEntry)

	for _, entry := range entries {
		container, _ := entry.Data["container"].(string)
		if container == "" {
			name, ok := entry.Data["container_name"].(string)
			if ok && name != "" {
				container = name
			} else {
				container = "unknown"
			}
		}

		byContainer[container] = append(byContainer[container], entry)
	}

	for _, containerEntries := range byContainer {
		if len(containerEntries) > 0 {
			shoutrrr.sendEntries(containerEntries, nil)
		}
	}
}

// Close gracefully shuts down the Shoutrrr notifier.
//
// It signals the notification goroutine to stop, waits for in-flight messages
// to complete within the grace period, cancels the context to unblock pending sends,
// and ensures the goroutine has finished before returning.
func (n *shoutrrrTypeNotifier) Close() {
	log := n.ll()

	n.closeOnce.Do(func() {
		n.closed.Store(true)

		// If no worker goroutine exists, skip waiting and cancel immediately.
		if !n.receiving.Load() {
			log.Debug().Msg("No notification worker running, canceling context immediately")
			n.cancel()

			return
		}

		// Signal goroutine to stop processing new messages.
		if n.stop != nil {
			log.Debug().Msg("Closing stop channel to signal shutdown")
			close(n.stop)
		}

		// Wait for the worker goroutine to exit by waiting on n.done channel.
		log.Debug().Msg("Waiting for the notification goroutine to finish")
		<-n.done

		// Only AFTER the worker has exited, close n.messages channel.
		log.Debug().Msg("Closing messages channel after worker exit")
		close(n.messages)

		// Cancel context to unblock any pending operations.
		log.Debug().Msg("Canceling notification context to unblock pending sends")
		n.cancel()
	})
}

// singleFocusContainer returns the sole focus container when the report is a
// session.SingleContainerReport (split path), or false for a full session report.
// Type assertion is required because a full report with one update is otherwise
// indistinguishable from a split single-container report.
func singleFocusContainer(report types.Report) (types.ContainerReport, bool) {
	if report == nil {
		return nil, false
	}

	scr, ok := report.(*session.SingleContainerReport)
	if !ok {
		return nil, false
	}

	if len(scr.UpdatedReports) == 1 {
		return scr.UpdatedReports[0], true
	}

	if len(scr.RestartedReports) == 1 {
		return scr.RestartedReports[0], true
	}

	if len(scr.SkippedReports) == 1 {
		return scr.SkippedReports[0], true
	}

	if len(scr.StaleReports) == 1 {
		return scr.StaleReports[0], true
	}

	return nil, false
}

// entryMatchesFocus reports whether a queued entry belongs to the focus container.
func entryMatchesFocus(entry *notificationEntry, focus types.ContainerReport) bool {
	if entry == nil || focus == nil {
		return false
	}

	name := focus.Name()
	if c, ok := entry.Data["container"].(string); ok && c == name {
		return true
	}

	if c, ok := entry.Data["container_name"].(string); ok && c == name {
		return true
	}

	// Image-level messages (e.g. cooldown) without a container field match by image name.
	_, hasContainer := entry.Data["container"]
	if !hasContainer {
		_, hasCN := entry.Data["container_name"]
		if !hasCN {
			img, ok := entry.Data["image"].(string)
			if ok && img != "" && img == focus.ImageName() {
				return true
			}
		}
	}

	return false
}

// deduplicateEntries removes duplicate log entries that occur when multiple containers
// share the same image. This applies to grouped (non-split) notifications only.
//
// For "Found new image" entries, deduplication is based on (message, image name, new image ID).
// When new_id is a short registry digest (check path) rather than a local image ID,
// entries still dedupe by image + new_id so multiple containers on the same image
// produce one notification in grouped mode.
// For "Removing image" entries, deduplication is based on (message, image ID).
// All other entries are kept as-is.
//
// Parameters:
//   - entries: Log entries that may contain duplicates.
//
// Returns:
//   - []*notificationEntry: Deduplicated entries preserving original order.
func deduplicateEntries(entries []*notificationEntry) []*notificationEntry {
	if len(entries) <= 1 {
		return entries
	}

	type dedupKey struct {
		message string
		data    string
	}

	seen := make(map[dedupKey]bool)
	result := make([]*notificationEntry, 0, len(entries))

	for _, entry := range entries {
		var key dedupKey

		switch entry.Message {
		case "Found new image":
			// Deduplicate by image name and new image ID.
			image, _ := entry.Data["image"].(string)
			newID, _ := entry.Data["new_id"].(string)
			key = dedupKey{message: entry.Message, data: image + "\x00" + newID}
		case "Found new Git revision":
			revision, _ := entry.Data["revision"].(string)
			shortCommit, _ := entry.Data["short_commit"].(string)
			key = dedupKey{message: entry.Message, data: revision + "\x00" + shortCommit}
		case "Removing image":
			// Deduplicate by image ID.
			imageID, _ := entry.Data["image_id"].(string)
			key = dedupKey{message: entry.Message, data: imageID}
		case "Image is within cooldown period - not eligible for update",
			"Image age exceeds cooldown - eligible for update",
			"Image creation time unavailable - update check unavailable":
			// Deduplicate by image name — multiple containers may share the same image.
			image, _ := entry.Data["image"].(string)
			key = dedupKey{message: entry.Message, data: image}
		default:
			// Non-deduplicatable entries are always kept.
			result = append(result, entry)

			continue
		}

		if !seen[key] {
			seen[key] = true

			result = append(result, entry)
		}
	}

	return result
}

// ShouldSendNotification checks if a notification should be sent for the given report based on the notifier's log level.
//
// Parameters:
//   - report: The report to check.
//
// Returns:
//   - bool: True if notification should be sent, false otherwise.
func (n *shoutrrrTypeNotifier) ShouldSendNotification(report types.Report) bool {
	if report != nil && n.logLevel == zerolog.ErrorLevel {
		if len(report.Failed()) == 0 {
			return false
		}
	}

	return true
}

// Run implements the [zerolog.Hook] interface for *shoutrrrTypeNotifier.
//
// It queues or immediately sends log events as notifications.
//
// Loop prevention:
//   - Events carrying notify=no (including all localLog output) are ignored before
//     any lock is taken, so nested localLog calls never re-enter the queue.
//   - If field extraction fails (nil or unparseable event buffer), the event is
//     fail-closed and not queued. This never risks missing notify=no and looping.
//
// Concurrent application log events are allowed. entriesMutex serializes queue
// access only. sendEntries and debug logging run after unlock so the lock is never
// held across template work or re-entrant hook paths that would deadlock.
//
// Events below the configured notification level are ignored.
//
// Only zerolog events on the hooked process logger are captured. Domain producers
// such as pkg/container ("Found new image", "Removing image") log through the process
// logger from NewClient so log-mode notifications receive them. Report-mode
// notifications (--notification-report) render from types.Report.
//
// Parameters:
//   - event: Zerolog event (fields are read from the in-progress JSON buffer).
//   - level: Event level.
//   - message: Log message.
func (n *shoutrrrTypeNotifier) Run(event *zerolog.Event, level zerolog.Level, message string) {
	if n.closed.Load() {
		return
	}

	// Filter by notification level (zerolog: higher ordinal is more severe).
	if level < n.logLevel {
		return
	}

	// Fail-closed: if we cannot parse fields, we cannot prove notify!=no, so skip queueing.
	// Done before entriesMutex so nested localLog hooks never contend with a held lock.
	fields, timestamp, ok := eventFieldMap(event)
	if !ok {
		// Emit a warning only on the first extraction failure to aid debugging
		// without flooding logs on every subsequent malformed event.
		n.extractWarnOnce.Do(func() {
			n.ll().Warn().
				Msg("Failed to extract fields from log event; notification queueing skipped for this event")
		})

		return
	}

	notify, has := fields["notify"].(string)
	if has && notify == "no" {
		return // Skip non-notify entries (loop prevention).
	}
	// notify is an internal control field, not template data.
	delete(fields, "notify")

	entry := &notificationEntry{
		Message: message,
		Data:    fields,
		Time:    timestamp,
		Level:   levelToString(level),
	}

	// Hold the mutex only for queue mutation. Log and send outside the lock.
	// Concurrent Runs from different goroutines are fine — they serialize here only.
	n.entriesMutex.Lock()
	batching := n.entries != nil

	var entriesCount int

	if batching {
		n.entries = append(n.entries, entry)
		entriesCount = len(n.entries)
	}
	n.entriesMutex.Unlock()

	if batching {
		// Name the field entry_level. Zerolog reserves level for event severity.
		// Using Str("level", ...) would overwrite Debug or Info in logfmt output.
		n.ll().Debug().
			Str("message", entry.Message).
			Str("entry_level", entry.Level).
			Int("entries_count", entriesCount).
			Bool("legacy_template", n.legacyTemplate).
			Msg("Log entry queued for batching")

		return
	}

	// Same entry_level naming as the batching path above.
	n.ll().Debug().
		Str("message", entry.Message).
		Str("entry_level", entry.Level).
		Bool("legacy_template", n.legacyTemplate).
		Msg("Log entry sent immediately (not batching)")
	n.sendEntries([]*notificationEntry{entry}, nil)
}

// ll returns the notifier's loop-safe logger, falling back to a discarded logger.
//
// Returns:
//   - *zerolog.Logger: Child logger with notify=no, or a nop logger when unset.
func (n *shoutrrrTypeNotifier) ll() *zerolog.Logger {
	if n != nil && n.localLog != nil {
		return n.localLog
	}

	nop := zerolog.Nop()

	return &nop
}

// eventFieldMap extracts application fields and timestamp from a zerolog.Event's
// in-progress buffer.
//
// At hook time the buffer holds a partial JSON object (no message, no closing brace).
// Zerolog does not expose a public GetString API on Event. This helper reconstructs
// the field map from the unexported buffer via reflection. The field layout is
// pinned to zerolog v1.35.x and requires revalidation on upgrades. reflect and
// unsafe are required because the event buffer is unexported.
//
// Parameters:
//   - event: In-progress zerolog event, or nil.
//
// Returns:
//   - map[string]any: Application fields with standard envelope keys removed.
//   - time.Time: Event timestamp, or time.Now when missing or unparsable.
//   - bool: False when extraction fails (caller must fail-closed).
func eventFieldMap(event *zerolog.Event) (map[string]any, time.Time, bool) {
	now := time.Now()
	if event == nil {
		return nil, now, false
	}

	rv := reflect.ValueOf(event).Elem().FieldByName("buf")
	if !rv.IsValid() || rv.Kind() != reflect.Slice || rv.Len() == 0 {
		return nil, now, false
	}

	buf := unsafe.Slice((*byte)(rv.UnsafePointer()), rv.Len())

	rawPtr, ok := eventFieldRawPool.Get().(*[]byte)
	if !ok || rawPtr == nil {
		buf := make([]byte, 0, eventFieldRawCapacity)
		rawPtr = &buf
	}

	// Reset the pooled buffer without dropping its capacity.
	raw := (*rawPtr)[:0]

	if cap(raw) < len(buf)+1 {
		raw = make([]byte, 0, len(buf)+1)
	}

	// Close the partial JSON object produced before the message is appended.
	raw = append(raw, buf...)
	raw = append(raw, '}')

	var fieldMap map[string]any

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	err := dec.Decode(&fieldMap)

	// Return only bounded buffers so oversized events do not stay in the pool.
	if cap(raw) <= eventFieldRawMaxCapacity {
		*rawPtr = raw[:0]
		eventFieldRawPool.Put(rawPtr)
	}

	if err != nil {
		return nil, now, false
	}

	// Normalize json.Number values to int64 or float64 so templates can compare
	// them as numbers without precision loss for large integer values.
	for fieldKey, fieldVal := range fieldMap {
		num, ok := fieldVal.(json.Number)
		if !ok {
			continue
		}

		intVal, intErr := num.Int64()
		if intErr == nil {
			fieldMap[fieldKey] = intVal

			continue
		}

		floatVal, floatErr := num.Float64()
		if floatErr == nil {
			fieldMap[fieldKey] = floatVal
		}
	}

	timestamp := now

	rawTS, ok := fieldMap[zerolog.TimestampFieldName]
	if ok {
		tsStr, ok := rawTS.(string)
		if ok {
			parsed, parseErr := time.Parse(time.RFC3339, tsStr)
			if parseErr != nil {
				parsed, parseErr = time.Parse(time.RFC3339Nano, tsStr)
			}

			if parseErr == nil {
				timestamp = parsed
			}
		}
	}

	// Drop standard zerolog envelope keys so templates see application fields only.
	// Keep ErrorFieldName ("error"): templates and JSON historically surface
	// WithError/Err as Data["error"] (historical notification template field name).
	delete(fieldMap, zerolog.LevelFieldName)
	delete(fieldMap, zerolog.TimestampFieldName)
	delete(fieldMap, zerolog.MessageFieldName)
	delete(fieldMap, zerolog.CallerFieldName)

	return fieldMap, timestamp, true
}

// send sends a message via the notification router.
//
// Cancellation before Send skips the delivery. An in-flight Send is abandoned
// after shutdownGracePeriod so Close cannot block indefinitely.
//
// Parameters:
//   - msg: Message to send.
func (n *shoutrrrTypeNotifier) send(msg string) {
	// Skip delivery when Close already canceled the worker.
	select {
	case <-n.ctx.Done():
		n.ll().Debug().Err(n.ctx.Err()).Msg("Notification send canceled")

		return
	default:
	}

	errsCh := make(chan []error, 1)

	go func() {
		errsCh <- n.Router.Send(msg, n.params)
	}()

	select {
	case errs := <-errsCh:
		processSendErrors(n, errs)
	case <-n.ctx.Done():
		timer := time.NewTimer(shutdownGracePeriod)
		defer timer.Stop()

		select {
		case errs := <-errsCh:
			processSendErrors(n, errs)
		case <-timer.C:
			n.ll().Debug().Err(n.ctx.Err()).Msg("Notification send canceled")
		}
	}
}

// buildMessage constructs a notification message from data.
//
// Parameters:
//   - data: Notification data.
//
// Returns:
//   - string: Rendered message.
//   - error: Non-nil if templating fails, nil on success.
func (n *shoutrrrTypeNotifier) buildMessage(data notificationData) (string, error) {
	log := n.ll()

	var body bytes.Buffer

	dataSource := any(data)
	if n.legacyTemplate {
		dataSource = data.Entries // Use entries only for legacy mode.
	}

	// Log template processing start with nil-safe report access.
	reportAvailable := data.Report != nil

	if log.GetLevel() <= zerolog.DebugLevel {
		log.Debug().
			Bool("legacy_template", n.legacyTemplate).
			Int("entries_count", len(data.Entries)).
			Int("container_count", reportCategoryCount(data.Report)).
			Bool("report_available", reportAvailable).
			Msg("Starting template processing for notification message")
	}

	// Execute template with data.
	err := n.template.Execute(&body, dataSource)
	if err != nil {
		log.Debug().
			Err(err).
			Bool("legacy_template", n.legacyTemplate).
			Str("template_name", n.template.Name()).
			Msg("Template execution failed")

		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	msg := body.String()

	// Log template processing result
	log.Debug().
		Int("msg_length", len(msg)).
		Bool("legacy_template", n.legacyTemplate).
		Str("template_name", n.template.Name()).
		Int("entries_count", len(data.Entries)).
		Msg("Template processing completed successfully")

	log.Debug().Str("message", msg).Msg("Generated notification message")

	return msg, nil
}

// reportCategoryCount returns the sum of report category lengths without calling All().
//
// Categories can overlap, so the sum may exceed the deduplicated All() count.
//
// Parameters:
//   - report: Session report, or nil.
//
// Returns:
//   - int: Combined length of all report categories.
func reportCategoryCount(report types.Report) int {
	if report == nil {
		return 0
	}

	return len(report.Scanned()) +
		len(report.Updated()) +
		len(report.Failed()) +
		len(report.Skipped()) +
		len(report.Stale()) +
		len(report.Fresh()) +
		len(report.Restarted())
}

// sendEntries sends batched entries and optional report.
//
// Parameters:
//   - entries: Log entries to send.
//   - report: Optional scan report.
func (n *shoutrrrTypeNotifier) sendEntries(entries []*notificationEntry, report types.Report) {
	log := n.ll()

	msg, err := n.buildMessage(notificationData{n.data, entries, report})
	if err != nil {
		log.Debug().
			Err(err).
			Str("message", msg).
			Msg("Preparing to send entries")
	} else {
		log.Debug().
			Str("message", msg).
			Msg("Preparing to send entries")
	}

	if msg == "" {
		if err != nil {
			log.Error().Err(err).Msg("Notification template error")
		} else if len(n.Urls) > 1 {
			log.Info().Msg("Skipping notification due to empty message")
		}

		log.Debug().Msg("Message empty, skipping send")

		return
	}

	// Use select with non-blocking send to coordinate with shutdown.
	// This ensures we can't enqueue messages after shutdown has begun.
	select {
	case n.messages <- msg:
		// Message sent successfully to channel
		log.Debug().
			Int("entries_count", len(entries)).
			Int("msg_length", len(msg)).
			Str("channel_status", "sent").
			Msg("Successfully sent message to notification channel")
	default:
		// Non-blocking send failed - check if closed or done before returning.
		// This check is done AFTER the send attempt to catch the race condition
		// where Close() signaled stop but sendEntries was already in progress.
		if n.closed.Load() {
			log.Debug().
				Int("entries_count", len(entries)).
				Int("msg_length", len(msg)).
				Str("channel_status", "closed").
				Msg("Notifier closed, skipping send")

			return
		}

		// Check if worker goroutine has exited
		select {
		case <-n.done:
			log.Debug().
				Int("entries_count", len(entries)).
				Int("msg_length", len(msg)).
				Str("channel_status", "worker_done").
				Msg("Worker goroutine done, skipping send")

			return
		default:
			// Channel is full (not closed, not done), apply backpressure
			log.Debug().
				Int("entries_count", len(entries)).
				Int("msg_length", len(msg)).
				Str("channel_status", "full").
				Msg("Channel full, skipping send (backpressure)")
		}
	}
}

// getShoutrrrTemplate retrieves or generates a template.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - tplString: Template string.
//   - legacy: Use legacy mode if true.
//
// Returns:
//   - *template.Template: Parsed or default template.
//   - error: Non-nil if parsing fails, nil on success.
func getShoutrrrTemplate(log *zerolog.Logger, tplString string, legacy bool) (*template.Template, error) {
	tplBase := template.New("").Funcs(templateFuncs)

	// Use common template if specified.
	builtin, found := commonTemplates[tplString]
	if found {
		log.Debug().Str("template", tplString).Msg("Using common template")
		tplString = builtin
	}

	var tpl *template.Template

	var err error

	// Parse provided template or use default based on presence of tplString.
	switch {
	case tplString != "":
		// Parse provided template if non-empty.
		tpl, err = tplBase.Parse(tplString)
		if err != nil {
			log.Debug().Err(err).Msg("Parse failed")

			return nil, fmt.Errorf("failed to parse template: %w", err)
		}
	default:
		// Fall back to default template.
		key := "default"
		if legacy {
			key = "default-legacy"
		}

		tpl = template.Must(tplBase.Parse(commonTemplates[key]))
	}

	return tpl, nil
}

// safeTruncate truncates a string to a maximum length for logging, appending "..." if truncated.
// Uses rune-aware truncation to avoid breaking UTF-8 sequences.
// If the string is shorter than or equal to maxURLLengthForLogging, returns it unchanged.
//
// Parameters:
//   - s: String to truncate.
//
// Returns:
//   - string: Truncated string or original if no truncation needed.
func safeTruncate(s string) string {
	runes := []rune(s)
	if len(runes) <= maxURLLengthForLogging {
		return s
	}

	return string(runes[:maxURLLengthForLogging]) + "..."
}

// sanitizeURLForLogging removes credentials and query parameters from URLs before truncating.
// Falls back to safeTruncate on parse errors to ensure safe logging.
//
// Parameters:
//   - rawURL: URL string to sanitize.
//
// Returns:
//   - string: Sanitized and truncated URL safe for logging.
func sanitizeURLForLogging(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Fallback to safe truncation if URL parsing fails
		return safeTruncate(rawURL)
	}

	// Remove user info (credentials)
	parsedURL.User = nil

	// Remove query parameters
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	// Remove path and opaque data (tokens/webhook IDs often live here)
	parsedURL.Path = ""
	parsedURL.RawPath = ""
	parsedURL.Opaque = ""

	// Reconstruct the URL without sensitive parts
	sanitized := parsedURL.String()

	return safeTruncate(sanitized)
}
