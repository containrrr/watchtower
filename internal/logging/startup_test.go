package logging

import (
	"bytes"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	mockLogging "github.com/nicholas-fedor/watchtower/internal/logging/mocks"
)

// newBufferLogger returns a logger that writes JSON events at level and above
// to a new buffer, so a test can check what was logged.
//
// Parameters:
//   - level: Lowest level the logger writes.
//
// Returns:
//   - *zerolog.Logger: Logger writing to the buffer.
//   - *bytes.Buffer: Buffer holding the logged events.
func newBufferLogger(level zerolog.Level) (*zerolog.Logger, *bytes.Buffer) {
	buffer := &bytes.Buffer{}

	return new(zerolog.New(buffer).Level(level).With().Timestamp().Logger()), buffer
}

// newVersionProvider returns a mock Docker client that reports API version 1.50.
//
// Parameters:
//   - t: Test that owns the mock.
//
// Returns:
//   - *mockLogging.MockAPIVersionProvider: Mock reporting the API version.
func newVersionProvider(t *testing.T) *mockLogging.MockAPIVersionProvider {
	t.Helper()

	client := mockLogging.NewMockAPIVersionProvider(t)
	client.EXPECT().GetVersion().Return("1.50").Maybe()

	return client
}

// TestWriteStartupMessage verifies the startup message: the Watchtower and
// Docker API versions, the notifiers, the image usage budget, the scope, and a
// trace-level warning, that nothing is written when startup messages are
// suppressed, and that a missing logger panics unless they are.
func TestWriteStartupMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// level is the lowest level the logger writes.
		level  zerolog.Level
		params StartupParams
		// want holds the text the output must contain, or nothing when the
		// output must be empty.
		want []string
	}{
		{
			name:   "should log startup information with no notifier",
			level:  zerolog.InfoLevel,
			params: StartupParams{HTTPAPIUpdate: true},
			want:   []string{"Watchtower v1.0.0 using Docker API v1.50", "Using no notifications"},
		},
		{
			name:   "should suppress startup messages when flag is set",
			level:  zerolog.InfoLevel,
			params: StartupParams{NoStartupMessage: true},
		},
		{
			name:   "should suppress startup messages including HTTP API when no-startup-message is set",
			level:  zerolog.InfoLevel,
			params: StartupParams{NoStartupMessage: true, HTTPAPIUpdate: true},
		},
		{
			name:   "should log Docker image usage budget when configured",
			level:  zerolog.InfoLevel,
			params: StartupParams{DiskSpaceMaxBytes: 40_000_000_000, DiskSpaceWarnBytes: 32_000_000_000},
			want: []string{
				"Docker image usage budget enabled",
				`"disk_space_max":40000000000`,
				`"disk_space_warn":32000000000`,
			},
		},
		{
			name:   "should log scope information when provided",
			level:  zerolog.InfoLevel,
			params: StartupParams{Scope: "prod"},
			want:   []string{"Only checking containers in scope"},
		},
		{
			name:  "should warn about trace logging",
			level: zerolog.TraceLevel,
			want:  []string{"Trace-level logging enabled"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log, buffer := newBufferLogger(tt.level)

			params := tt.params
			params.Logger = log
			params.Filtering = "Watching all containers"
			params.Client = newVersionProvider(t)
			params.Version = "v1.0.0"

			WriteStartupMessage(params)

			if len(tt.want) == 0 {
				assert.Empty(t, buffer.String())

				return
			}

			for _, want := range tt.want {
				assert.Contains(t, buffer.String(), want)
			}
		})
	}

	t.Run("should panic clearly when Logger is nil and not suppressed", func(t *testing.T) {
		t.Parallel()

		assert.PanicsWithValue(t, "logging.WriteStartupMessage: Logger is required when NoStartupMessage is false", func() {
			WriteStartupMessage(StartupParams{
				Filtering: "Watching all containers",
				Client:    newVersionProvider(t),
				Version:   "v1.0.0",
			})
		})
	})

	t.Run("should not panic when Logger is nil but NoStartupMessage is set", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() {
			WriteStartupMessage(StartupParams{
				NoStartupMessage: true,
				Filtering:        "Watching all containers",
				Client:           newVersionProvider(t),
				Version:          "v1.0.0",
			})
		})
	})
}

// Test_setupStartupLogger verifies that the startup logger is the given logger
// when there is no notifier to batch startup lines through.
func Test_setupStartupLogger(t *testing.T) {
	t.Parallel()

	t.Run("should return the provided logger when notifier is nil", func(t *testing.T) {
		t.Parallel()

		log, _ := newBufferLogger(zerolog.InfoLevel)

		assert.Same(t, log, setupStartupLogger(log, nil))
	})
}

// Test_logNotifierInfo verifies that the startup message names the configured
// notifiers, or says there are none.
func Test_logNotifierInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		notifiers []string
		want      string
	}{
		{name: "should log configured notifiers", notifiers: []string{"email", "slack"}, want: "Using notifications: email, slack"},
		{name: "should log when no notifiers are configured", notifiers: []string{}, want: "Using no notifications"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log, buffer := newBufferLogger(zerolog.InfoLevel)

			logNotifierInfo(log, tt.notifiers)
			assert.Contains(t, buffer.String(), tt.want)
		})
	}
}

// Test_logScheduleInfo verifies the startup line that describes when updates
// run: on a schedule, once, on startup, through the HTTP API with or without
// periodic polls, or on the default schedule.
func Test_logScheduleInfo(t *testing.T) {
	t.Parallel()

	updateOnStart := true

	tests := []struct {
		name string
		info ScheduleInfo
		want string
	}{
		{
			name: "should log scheduled run information",
			info: ScheduleInfo{Sched: time.Now().Add(time.Hour)},
			want: "Next scheduled run",
		},
		{
			name: "should log one-time update",
			info: ScheduleInfo{RunOnce: true},
			want: "Running a one time update",
		},
		{
			name: "should log flag conflict when both run-once and update-on-start are set",
			info: ScheduleInfo{RunOnce: true, UpdateOnStart: &updateOnStart},
			want: "Run once mode: Disregarding update on start",
		},
		{
			name: "should log update on start",
			info: ScheduleInfo{UpdateOnStart: &updateOnStart},
			want: "Update on startup enabled",
		},
		{
			name: "should log HTTP API without periodic polls",
			info: ScheduleInfo{HTTPAPIUpdate: true},
			want: "HTTP API enabled and periodic updates disabled",
		},
		{
			name: "should log HTTP API with periodic polls",
			info: ScheduleInfo{HTTPAPIUpdate: true, HTTPAPIPeriodicPolls: true},
			want: "HTTP API and periodic updates enabled",
		},
		{
			name: "should log default periodic updates",
			info: ScheduleInfo{},
			want: "Periodic updates are enabled with default schedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log, buffer := newBufferLogger(zerolog.InfoLevel)

			logScheduleInfo(log, tt.info)
			assert.Contains(t, buffer.String(), tt.want)
		})
	}
}
