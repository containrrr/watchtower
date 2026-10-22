package notifications

import (
	"errors"
	"fmt"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// emailType is the identifier for email notifications.
//
// Deprecated: Legacy email notification type is deprecated.
// Use --notification-url with an smtp:// URL instead.
//
// TODO: Remove emailType constant for the v2 release.
//
//nolint:godox
const emailType = "email"

// defaultTimeout is the default duration for SMTP operations.
const defaultTimeout = 10 * time.Second

// Errors for email notification configuration.
var (
	// errInvalidPortRange indicates that the specified SMTP port is outside the valid range (0-65535).
	errInvalidPortRange = errors.New("port out of valid range (0-65535)")
)

// emailTypeNotifier handles email notifications via SMTP.
//
// Deprecated: Legacy email notifier is deprecated.
// Use --notification-url with an smtp:// URL instead.
//
// TODO: Remove emailTypeNotifier for the v2 release.
//
//nolint:godox
type emailTypeNotifier struct {
	From, To               string        // Sender and recipient email addresses.
	Server, User, Password string        // SMTP server details.
	Port                   int           // SMTP server port.
	tlsSkipVerify          bool          // Skip TLS verification if true.
	delay                  time.Duration // Delay for batching notifications.
	log                    *zerolog.Logger
}

// newEmailNotifier creates an email notifier from resolved legacy settings.
//
// Parameters:
//   - log: Logger for configuration diagnostics and fatal validation errors.
//   - legacy: Deprecated email SMTP settings (from process config or flags).
//
// Returns:
//   - types.ConvertibleNotifier: New email notifier instance.
//
// Deprecated: Legacy email notifier is deprecated.
// Use --notification-url with an smtp:// URL instead.
//
// TODO: Remove newEmailNotifier for the v2 release.
//
//nolint:godox
func newEmailNotifier(log *zerolog.Logger, legacy notifyConfig.Legacy) types.ConvertibleNotifier {
	if legacy.EmailFrom == "" {
		log.Fatal().Msg("Email from address is empty.")
	}

	if legacy.EmailTo == "" {
		log.Fatal().Msg("Email to address is empty.")
	}

	if legacy.EmailServer == "" {
		log.Fatal().Msg("Email server is empty.")
	}

	from := legacy.EmailFrom
	to := legacy.EmailTo //nolint:varnamelen
	server := legacy.EmailServer
	user := legacy.EmailUser
	password := legacy.EmailPassword
	port := legacy.EmailPort
	tlsSkipVerify := legacy.EmailTLSSkipVerify
	delay := legacy.EmailDelay

	clog := log.With().
		Str("from", from).
		Str("to", to).
		Str("server", server).
		Int("port", port).
		Bool("tls_skip", tlsSkipVerify).
		Int("delay_seconds", delay).
		Logger()
	clog.Debug().Msg("Initializing email notifier from flags")

	// Log credential presence only at trace level. Never log the password value.
	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("user", user).
			Bool("credentials_configured", user != "" || password != "").
			Msg("Email notifier credentials loaded")
	}

	return &emailTypeNotifier{
		From:          from,
		To:            to,
		Server:        server,
		User:          user,
		Password:      password,
		Port:          port,
		tlsSkipVerify: tlsSkipVerify,
		delay:         time.Duration(delay) * time.Second,
		log:           log,
	}
}

// GetURL generates the SMTP URL from the notifier's configuration.
//
// Parameters:
//   - c: Cobra command (unused here).
//
// Returns:
//   - string: SMTP URL.
//   - error: Non-nil if port is invalid, nil on success.
//
// Deprecated: This method is part of the legacy email notifier and will be removed
// for the v2 release. Use --notification-url with an smtp:// URL instead.
func (e *emailTypeNotifier) GetURL(_ *cobra.Command) (string, error) {
	clog := e.log.With().
		Str("from", e.From).
		Str("to", e.To).
		Str("server", e.Server).
		Int("port", e.Port).
		Logger()
	clog.Debug().Msg("Generating SMTP URL")

	// Validate port range (0-65535).
	if e.Port < 0 || e.Port > 65535 {
		clog.Debug().Int("port", e.Port).Msg("Invalid SMTP port")

		return "", fmt.Errorf("port %d: %w", e.Port, errInvalidPortRange)
	}

	// Configure SMTP settings.
	port := uint16(e.Port)

	conf := &smtp.Config{
		FromAddress:   e.From,
		FromName:      "Watchtower",
		ToAddresses:   []string{e.To},
		Port:          port,
		Host:          e.Server,
		Username:      e.User,
		Password:      e.Password,
		UseStartTLS:   true,
		UseHTML:       false,
		Encryption:    smtp.EncMethods.Auto,
		Auth:          smtp.AuthTypes.None,
		ClientHost:    "localhost",
		SkipTLSVerify: e.tlsSkipVerify,
		Timeout:       defaultTimeout,
	}

	// Enable authentication if credentials provided.
	if len(e.User) > 0 {
		conf.Auth = smtp.AuthTypes.Plain

		clog.Debug().Msg("Using plain authentication")
	}

	url := conf.GetURL().String()

	clog.Debug().
		Bool("tls_skip", e.tlsSkipVerify).
		Bool("auth_enabled", len(e.User) > 0).
		Msg("Generated SMTP URL")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("url", redactServiceURL(url)).
			Msg("Generated SMTP URL")
	}

	return url, nil
}

// GetDelay returns the delay for batching email notifications.
//
// Returns:
//   - time.Duration: Configured delay.
//
// Deprecated: This method is part of the legacy email notifier and will be removed
// for the v2 release. Use --notifications-delay instead.
func (e *emailTypeNotifier) GetDelay() time.Duration {
	e.log.Debug().
		Str("from", e.From).
		Str("to", e.To).
		Str("server", e.Server).
		Dur("delay", e.delay).
		Msg("Retrieved email notification delay")

	return e.delay
}
