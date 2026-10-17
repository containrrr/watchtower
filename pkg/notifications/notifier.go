package notifications

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"

	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// colorHex is the default notification color used for services that support it
// (formatted as a CSS hex string).
const colorHex = "#406170"

// colorInt is the default notification color used for services that support it
// (as an int value).
const colorInt = 0x406170

// errUnknownNotificationType indicates an unsupported legacy notification type name.
var errUnknownNotificationType = errors.New("unknown notification type")

// NewNotifier constructs the notification client from resolved process settings.
//
// It parses the notification log level, loads an optional template file, builds static
// template data, appends legacy Shoutrrr URLs when configured, and creates the client.
//
// Parameters:
//   - log: Process logger for configuration-time diagnostics (required and non-nil).
//   - cfg: Notification settings from config.Load (Config.Notify).
//
// Returns:
//   - types.Notifier: Configured notifier instance.
func NewNotifier(log *zerolog.Logger, cfg notifyConfig.Notify) types.Notifier {
	if log == nil {
		panic("notifications.NewNotifier: log is required")
	}

	// Parse the notification log level from resolved configuration.
	// Avoid the field name level. Zerolog reserves it for event severity.
	clog := log.With().Str("notifications_level", cfg.Level).Logger()
	clog.Debug().Msg("Parsing notifications log level")

	logLevel, err := logging.ParseLevel(cfg.Level)
	if err != nil {
		clog.Fatal().Err(err).Msg("Invalid notifications log level")
	}

	// Prefer template file contents when set. Otherwise use the inline template string.
	tplString := cfg.Template

	if cfg.TemplateFile != "" {
		content, readErr := os.ReadFile(cfg.TemplateFile)
		if readErr != nil {
			clog.Fatal().
				Err(readErr).
				Str("file", cfg.TemplateFile).
				Msg("Failed to read notification template file")
		}

		tplString = string(content)

		clog.Debug().
			Str("file", cfg.TemplateFile).
			Msg("Loaded notification template from file")
	}

	data := templateData(log, cfg.Hostname, cfg.TitleTag, cfg.EmailSubjectTag, cfg.SkipTitle)

	// Start from configured Shoutrrr URLs, then append any legacy type URLs.
	urls := append([]string(nil), cfg.URLs...)
	urls, legacyDelay := appendLegacyURLs(log, urls, cfg.LegacyTypes, cfg.Legacy)
	delay := getDelay(log, cfg.DelaySeconds, legacyDelay)

	// Use report template when enabled, otherwise use legacy template.
	legacyTemplate := !cfg.Report

	templateSource := "inline"
	if cfg.TemplateFile != "" {
		templateSource = cfg.TemplateFile
	}

	clog.Debug().
		Int("url_count", len(urls)).
		Str("template_source", templateSource).
		Int("template_bytes", len(tplString)).
		Bool("skip_report", !cfg.Report).
		Bool("stdout", cfg.LogStdout).
		Dur("delay", delay).
		Str("hostname", data.Host).
		Str("title", data.Title).
		Bool("legacy", legacyTemplate).
		Msg("Creating notifier with configuration")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Strs("urls", redactServiceURLs(urls)).
			Msg("Notifier Shoutrrr URLs loaded")
	}

	return createNotifier(
		log,
		urls,
		logLevel,
		tplString,
		legacyTemplate,
		data,
		cfg.LogStdout,
		delay,
	)
}

// BuildURLs builds Shoutrrr notification URLs from resolved notification settings
// without initializing a notifier.
//
// It returns configured URLs plus any legacy Shoutrrr URLs generated from deprecated
// notification types. Errors are returned instead of causing a fatal exit.
//
// Parameters:
//   - log: Logger for legacy notifier construction diagnostics.
//   - cfg: Notification settings from config.Load (Config.Notify).
//
// Returns:
//   - []string: Shoutrrr URLs ready for output or notification use.
//   - error: Non-nil if an unknown legacy notification type is specified or if
//     a legacy notifier fails to generate its URL.
//
// TODO: Remove BuildURLs after the v2 release.
//
//nolint:godox
func BuildURLs(log *zerolog.Logger, cfg notifyConfig.Notify) ([]string, error) {
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}

	urls := append([]string(nil), cfg.URLs...)

	for _, notificationType := range cfg.LegacyTypes {
		if notificationType == shoutrrrType {
			continue
		}

		ctor, ok := legacyNotifierCtors[notificationType]
		if !ok {
			return nil, fmt.Errorf("%w: %q", errUnknownNotificationType, notificationType)
		}

		legacyNotifier := ctor(log, cfg.Legacy)

		shoutrrrURL, err := legacyNotifier.GetURL(nil)
		if err != nil {
			return nil, fmt.Errorf("create %q notification config: %w", notificationType, err)
		}

		urls = append(urls, shoutrrrURL)
	}

	return urls, nil
}

// templateData builds static notification title/host data from resolved values.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - hostname: Hostname from configuration, or empty to use the system hostname.
//   - titleTag: Optional title prefix tag.
//   - emailSubjectTag: Deprecated fallback tag when titleTag is empty.
//   - skipTitle: When true, title is left empty.
//
// Returns:
//   - StaticData: Host and title for notification templates.
func templateData(log *zerolog.Logger, hostname, titleTag, emailSubjectTag string, skipTitle bool) StaticData {
	clog := log.With().Str("hostname_flag", hostname).Logger()
	clog.Debug().Msg("Retrieving template data")

	// Get hostname from configuration or system.
	if hostname == "" {
		hostname, _ = os.Hostname()
		clog.Debug().Str("hostname", hostname).Msg("Using system hostname")
	}

	title := ""

	if !skipTitle {
		tag := titleTag
		if tag == "" {
			tag = emailSubjectTag
			if tag != "" {
				clog.Warn().
					Str("tag", tag).
					Msg("Using deprecated email subject tag flag. Use the notification-title-tag configuration option instead.")
			}
		}

		title = getTitle(log, hostname, tag)
	}

	clog.Debug().
		Str("hostname", hostname).
		Str("title", title).
		Msg("Populated template data")

	return StaticData{
		Host:  hostname,
		Title: title,
	}
}

// legacyNotifierCtor builds a ConvertibleNotifier from resolved legacy settings.
//
//nolint:staticcheck // SA1019: legacy ConvertibleNotifier until v2.
type legacyNotifierCtor func(log *zerolog.Logger, legacy notifyConfig.Legacy) types.ConvertibleNotifier

// legacyNotifierCtors maps legacy notification type names to constructors.
// shoutrrr is omitted so those entries are skipped during append.
var legacyNotifierCtors = map[string]legacyNotifierCtor{
	emailType:   newEmailNotifier,
	slackType:   newSlackNotifier,
	msTeamsType: newMsTeamsNotifier,
	gotifyType:  newGotifyNotifier,
}

// appendLegacyURLs adds shoutrrr URLs from legacy notification type names.
//
// Parameters:
//   - log: Logger for diagnostics and fatal errors.
//   - urls: Initial URL list.
//   - notificationTypes: Legacy type names (email, slack, msteams, gotify).
//   - legacy: Per-type settings for deprecated notifiers.
//
// Returns:
//   - []string: Updated URL list including generated Shoutrrr URLs.
//   - time.Duration: Delay reported by a legacy DelayNotifier, or zero.
//
// Deprecated: Legacy notification types are deprecated.
// Use --notification-url instead.
//
// TODO: Remove appendLegacyURLs for the v2 release.
//
//nolint:godox
func appendLegacyURLs(
	log *zerolog.Logger,
	urls []string,
	notificationTypes []string,
	legacy notifyConfig.Legacy,
) ([]string, time.Duration) {
	clog := log.With().Str("function", "appendLegacyURLs").Logger()
	clog.Debug().Msg("Appending legacy notification URLs")

	clog.Debug().
		Strs("types", notificationTypes).
		Msg("Processing legacy notification types")

	legacyDelay := time.Duration(0)

	for _, notificationType := range notificationTypes {
		if notificationType == shoutrrrType {
			continue
		}

		ctor, ok := legacyNotifierCtors[notificationType]
		if !ok {
			clog.Fatal().
				Str("type", notificationType).
				Msg("Unknown notification type")

			continue
		}

		legacyNotifier := ctor(log, legacy)

		// Generate shoutrrr URL from legacy notifier.
		shoutrrrURL, err := legacyNotifier.GetURL(nil)
		if err != nil {
			clog.Fatal().
				Err(err).
				Str("type", notificationType).
				Msg("Failed to create notification config")
		}

		urls = append(urls, shoutrrrURL)

		// Check for delay if supported.
		delayNotifier, ok := legacyNotifier.(types.DelayNotifier)
		if ok {
			legacyDelay = delayNotifier.GetDelay()
			clog.Debug().
				Str("type", notificationType).
				Dur("delay", legacyDelay).
				Msg("Retrieved delay from legacy notifier")
		}

		clog.Trace().
			Str("type", notificationType).
			Str("url", redactServiceURL(shoutrrrURL)).
			Msg("Created Shoutrrr URL from legacy notifier")
	}

	clog.Debug().
		Int("url_count", len(urls)).
		Dur("delay", legacyDelay).
		Msg("Completed legacy URL appending")

	return urls, legacyDelay
}

// getDelay selects the notification delay from a legacy value or configured seconds.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - delaySeconds: Configured delay in seconds (from Config.Notify.DelaySeconds).
//   - legacyDelay: Delay from a legacy notifier type, preferred when non-zero.
//
// Returns:
//   - time.Duration: Selected delay (legacy delay if set, otherwise delaySeconds, otherwise zero).
//
// Deprecated: Prefer NewNotifier with Config.Notify from config.Load.
//
// TODO: Simplify getDelay to only use configured delay seconds when legacy types are removed.
//
//nolint:godox
func getDelay(log *zerolog.Logger, delaySeconds int, legacyDelay time.Duration) time.Duration {
	clog := log.With().Dur("legacy_delay", legacyDelay).Logger()
	clog.Debug().Msg("Determining notification delay")

	// Use legacy delay if set.
	if legacyDelay > 0 {
		clog.Debug().Msg("Using legacy delay")

		return legacyDelay
	}

	// Use configured delay when no legacy delay applies.
	if delaySeconds > 0 {
		delayDuration := time.Duration(delaySeconds) * time.Second
		clog.Debug().
			Dur("delay", delayDuration).
			Msg("Using configured delay")

		return delayDuration
	}

	clog.Debug().Msg("No delay configured, using zero")

	return 0
}

// getTitle formats the notification title with hostname and tag.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - hostname: Hostname to include.
//   - tag: Optional tag prefix.
//
// Returns:
//   - string: Formatted title.
func getTitle(log *zerolog.Logger, hostname, tag string) string {
	clog := log.With().
		Str("hostname", hostname).
		Str("tag", tag).
		Logger()
	clog.Debug().Msg("Generating notification title")

	// Build title with optional tag and hostname.
	b := strings.Builder{}
	if tag != "" {
		b.WriteRune('[')
		b.WriteString(tag)
		b.WriteRune(']')
		b.WriteRune(' ')
	}

	b.WriteString("Watchtower updates")

	if hostname != "" {
		b.WriteString(" on ")
		b.WriteString(hostname)
	}

	title := b.String()
	clog.Debug().
		Str("title", title).
		Msg("Generated notification title")

	return title
}

// LogLegacyDeprecationWarnings logs deprecation warnings for legacy notification types.
//
// It iterates over the provided notification types and logs a warning for each
// legacy type, advising users to migrate to the notification-url configuration option.
//
// Parameters:
//   - log: Logger for warnings.
//   - notificationTypes: List of notification type strings to check.
func LogLegacyDeprecationWarnings(log *zerolog.Logger, notificationTypes []string) {
	for _, notificationType := range notificationTypes {
		switch notificationType {
		case emailType, slackType, msTeamsType, gotifyType:
			log.Warn().
				Msgf("Using deprecated legacy %s notification configuration. "+
					"Use the notification-url configuration option instead.",
					notificationType)
		}
	}
}
