package logging_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	mockLogging "github.com/nicholas-fedor/watchtower/internal/logging/mocks"
)

// TestStartupLogging runs the Ginkgo test suite for the internal logging startup package.
func TestStartupLogging(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Internal Logging Startup Suite")
}

// testLogger builds a *zerolog.Logger that writes to buf at the given level.
func testLogger(buf *bytes.Buffer, level zerolog.Level) *zerolog.Logger {
	l := zerolog.New(buf).Level(level).With().Timestamp().Logger()

	return &l
}

var _ = ginkgo.Describe("WriteStartupMessage", func() {
	var (
		client *mockLogging.MockAPIVersionProvider
		buffer *bytes.Buffer
		log    *zerolog.Logger
	)

	ginkgo.BeforeEach(func() {
		client = mockLogging.NewMockAPIVersionProvider(ginkgo.GinkgoT())
		client.EXPECT().GetVersion().Return("1.50").Maybe()

		buffer = &bytes.Buffer{}
		log = testLogger(buffer, zerolog.InfoLevel)
	})

	ginkgo.It("should log startup information with no notifier", func() {
		logging.WriteStartupMessage(logging.StartupParams{
			HTTPAPIUpdate: true,
			Logger:        log,
			Filtering:     "Watching all containers",
			Client:        client,
			Version:       "v1.0.0",
		})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Watchtower v1.0.0 using Docker API v1.50"))
		gomega.Expect(output).To(gomega.ContainSubstring("Using no notifications"))
	})

	ginkgo.It("should suppress startup messages when flag is set", func() {
		logging.WriteStartupMessage(logging.StartupParams{
			Logger:           log,
			NoStartupMessage: true,
			Filtering:        "Watching all containers",
			Client:           client,
			Version:          "v1.0.0",
		})

		gomega.Expect(buffer.String()).To(gomega.BeEmpty())
	})

	ginkgo.It(
		"should suppress startup messages including HTTP API when no-startup-message is set",
		func() {
			logging.WriteStartupMessage(logging.StartupParams{
				Logger:           log,
				NoStartupMessage: true,
				HTTPAPIUpdate:    true,
				Filtering:        "Watching all containers",
				Client:           client,
				Version:          "v1.0.0",
			})

			gomega.Expect(buffer.String()).To(gomega.BeEmpty())
		},
	)

	ginkgo.It("should log Docker image usage budget when configured", func() {
		logging.WriteStartupMessage(logging.StartupParams{
			Logger:             log,
			Filtering:          "Watching all containers",
			Client:             client,
			Version:            "v1.0.0",
			DiskSpaceMaxBytes:  40_000_000_000,
			DiskSpaceWarnBytes: 32_000_000_000,
		})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Docker image usage budget enabled"))
		gomega.Expect(output).To(gomega.ContainSubstring(`"disk_space_max":40000000000`))
		gomega.Expect(output).To(gomega.ContainSubstring(`"disk_space_warn":32000000000`))
	})

	ginkgo.It("should log scope information when provided", func() {
		logging.WriteStartupMessage(logging.StartupParams{
			Logger:    log,
			Filtering: "Watching all containers",
			Scope:     "prod",
			Client:    client,
			Version:   "v1.0.0",
		})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Only checking containers in scope"))
	})

	ginkgo.It("should warn about trace logging", func() {
		traceLog := testLogger(buffer, zerolog.TraceLevel)

		logging.WriteStartupMessage(logging.StartupParams{
			Logger:    traceLog,
			Filtering: "Watching all containers",
			Client:    client,
			Version:   "v1.0.0",
		})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Trace-level logging enabled"))
	})

	ginkgo.It("should panic clearly when Logger is nil and not suppressed", func() {
		gomega.Expect(func() {
			logging.WriteStartupMessage(logging.StartupParams{
				Filtering: "Watching all containers",
				Client:    client,
				Version:   "v1.0.0",
			})
		}).To(gomega.PanicWith(gomega.MatchRegexp("Logger is required")))
	})

	ginkgo.It("should not panic when Logger is nil but NoStartupMessage is set", func() {
		gomega.Expect(func() {
			logging.WriteStartupMessage(logging.StartupParams{
				NoStartupMessage: true,
				Filtering:        "Watching all containers",
				Client:           client,
				Version:          "v1.0.0",
			})
		}).NotTo(gomega.Panic())
		gomega.Expect(buffer.String()).To(gomega.BeEmpty())
	})
})

var _ = ginkgo.Describe("SetupStartupLogger", func() {
	ginkgo.It("should return the provided logger when notifier is nil", func() {
		buf := &bytes.Buffer{}
		log := testLogger(buf, zerolog.InfoLevel)

		// Suppression is handled by WriteStartupMessage's early return. This helper
		// returns the logger instance passed in (no global logger).
		got := logging.SetupStartupLogger(log, nil)
		gomega.Expect(got).To(gomega.Equal(log))
	})
})

var _ = ginkgo.Describe("LogNotifierInfo", func() {
	var (
		buffer *bytes.Buffer
		log    *zerolog.Logger
	)

	ginkgo.BeforeEach(func() {
		buffer = &bytes.Buffer{}
		log = testLogger(buffer, zerolog.InfoLevel)
	})

	ginkgo.It("should log configured notifiers", func() {
		logging.LogNotifierInfo(log, []string{"email", "slack"})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Using notifications: email, slack"))
	})

	ginkgo.It("should log when no notifiers are configured", func() {
		logging.LogNotifierInfo(log, []string{})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Using no notifications"))
	})
})

var _ = ginkgo.Describe("LogScheduleInfo", func() {
	var (
		buffer *bytes.Buffer
		log    *zerolog.Logger
	)

	ginkgo.BeforeEach(func() {
		buffer = &bytes.Buffer{}
		log = testLogger(buffer, zerolog.InfoLevel)
	})

	ginkgo.It("should log scheduled run information", func() {
		sched := time.Now().Add(time.Hour)

		logging.LogScheduleInfo(log, logging.ScheduleInfo{Sched: sched})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Next scheduled run"))
	})

	ginkgo.It("should log one-time update", func() {
		logging.LogScheduleInfo(log, logging.ScheduleInfo{RunOnce: true})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Running a one time update"))
	})

	ginkgo.It("should log flag conflict when both run-once and update-on-start are set", func() {
		updateOnStart := true

		logging.LogScheduleInfo(log, logging.ScheduleInfo{
			RunOnce:       true,
			UpdateOnStart: &updateOnStart,
		})

		output := buffer.String()
		gomega.Expect(output).
			To(gomega.ContainSubstring("Run once mode: Disregarding update on start"))
	})

	ginkgo.It("should log update on start", func() {
		updateOnStart := true

		logging.LogScheduleInfo(log, logging.ScheduleInfo{UpdateOnStart: &updateOnStart})

		output := buffer.String()
		gomega.Expect(output).To(gomega.ContainSubstring("Update on startup enabled"))
	})

	ginkgo.It("should log HTTP API without periodic polls", func() {
		logging.LogScheduleInfo(log, logging.ScheduleInfo{HTTPAPIUpdate: true})

		output := buffer.String()
		gomega.Expect(output).
			To(gomega.ContainSubstring("HTTP API enabled and periodic updates disabled"))
	})

	ginkgo.It("should log HTTP API with periodic polls", func() {
		logging.LogScheduleInfo(log, logging.ScheduleInfo{
			HTTPAPIUpdate:        true,
			HTTPAPIPeriodicPolls: true,
		})

		output := buffer.String()
		gomega.Expect(output).
			To(gomega.ContainSubstring("HTTP API and periodic updates enabled"))
	})

	ginkgo.It("should log default periodic updates", func() {
		logging.LogScheduleInfo(log, logging.ScheduleInfo{})

		output := buffer.String()
		gomega.Expect(output).
			To(gomega.ContainSubstring("Periodic updates are enabled with default schedule"))
	})
})
