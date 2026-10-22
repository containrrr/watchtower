package notifications

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/gotify"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// gotifyType is the identifier for Gotify notifications.
//
// Deprecated: Legacy gotify notification type is deprecated.
// Use --notification-url with a gotify:// URL instead.
//
// TODO: Remove gotifyType constant for the v2 release.
//
//nolint:godox
const gotifyType = "gotify"

// gotifyTypeNotifier handles Gotify notifications.
//
// It configures URL, token, and TLS settings.
//
// Deprecated: Legacy gotify notifier is deprecated.
// Use --notification-url with a gotify:// URL instead.
//
// TODO: Remove gotifyTypeNotifier for the v2 release.
//
//nolint:godox
type gotifyTypeNotifier struct {
	gotifyURL                string // Gotify server URL.
	gotifyAppToken           string // Gotify application token.
	gotifyInsecureSkipVerify bool   // Skip TLS verification if true.
	log                      *zerolog.Logger
}

// newGotifyNotifier creates a Gotify notifier from resolved legacy settings.
//
// Parameters:
//   - log: Logger for configuration diagnostics and fatal validation errors.
//   - legacy: Deprecated Gotify server settings (from process config or flags).
//
// Returns:
//   - types.ConvertibleNotifier: New Gotify notifier instance.
//
// Deprecated: Legacy gotify notifier is deprecated.
// Use --notification-url with a gotify:// URL instead.
//
// TODO: Remove newGotifyNotifier for the v2 release.
//
//nolint:godox
func newGotifyNotifier(log *zerolog.Logger, legacy notifyConfig.Legacy) types.ConvertibleNotifier {
	apiURL := requireGotifyURL(log, legacy.GotifyURL)
	token := requireGotifyToken(log, legacy.GotifyToken)
	skipVerify := legacy.GotifyTLSSkipVerify

	clog := log.With().
		Str("url", redactServiceURL(apiURL)).
		Bool("skip_verify", skipVerify).
		Logger()
	clog.Debug().Msg("Initializing Gotify notifier")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Int("token_length", len(token)).
			Msg("Gotify notifier token loaded")
	}

	return &gotifyTypeNotifier{
		gotifyURL:                apiURL,
		gotifyAppToken:           token,
		gotifyInsecureSkipVerify: skipVerify,
		log:                      log,
	}
}

// requireGotifyToken validates a Gotify token.
//
// Parameters:
//   - log: Logger for fatal validation errors.
//   - gotifyToken: Token value from resolved configuration or flags.
//
// Returns:
//   - string: Token value (fatal if empty).
//
// Deprecated: This function is part of the legacy gotify notifier and will be removed
// for the v2 release. Use --notification-url with a gotify:// URL instead.
func requireGotifyToken(log *zerolog.Logger, gotifyToken string) string {
	clog := log.With().Str("flag", "notification-gotify-token").Logger()

	// Fatal error if token is missing.
	if len(gotifyToken) < 1 {
		clog.Fatal().Msg("Gotify token is empty.")
	}

	clog.Debug().Int("token_length", len(gotifyToken)).Msg("Retrieved Gotify token")

	return gotifyToken
}

// requireGotifyURL validates a Gotify URL.
//
// Parameters:
//   - log: Logger for fatal validation errors.
//   - gotifyURL: URL value from resolved configuration or flags.
//
// Returns:
//   - string: Validated URL (fatal if empty or malformed).
//
// Deprecated: This function is part of the legacy gotify notifier and will be removed
// for the v2 release. Use --notification-url with a gotify:// URL instead.
func requireGotifyURL(log *zerolog.Logger, gotifyURL string) string {
	clog := log.With().
		Str("flag", "notification-gotify-url").
		Str("url", redactServiceURL(gotifyURL)).
		Logger()

	// Fatal error if URL is missing.
	if len(gotifyURL) < 1 {
		clog.Fatal().Msg("Gotify URL is empty")
	}

	// Validate URL scheme.
	if !strings.HasPrefix(gotifyURL, "http://") && !strings.HasPrefix(gotifyURL, "https://") {
		clog.Fatal().Msg("Gotify URL must start with \"http://\" or \"https://\"")
	}

	// Warn if using insecure HTTP.
	if strings.HasPrefix(gotifyURL, "http://") {
		clog.Warn().Msg("Using an HTTP URL for Gotify is insecure")
	}

	clog.Debug().
		Str("scheme", strings.Split(gotifyURL, ":")[0]).
		Msg("Validated Gotify URL")

	return gotifyURL
}

// GetURL generates the Gotify service URL from the notifier's configuration.
//
// Parameters:
//   - c: Cobra command (unused here).
//
// Returns:
//   - string: Gotify service URL.
//   - error: Non-nil if URL parsing fails, nil on success.
//
// Deprecated: This method is part of the legacy gotify notifier and will be removed
// for the v2 release. Use --notification-url with a gotify:// URL instead.
func (n *gotifyTypeNotifier) GetURL(_ *cobra.Command) (string, error) {
	clog := n.log
	clog.Debug().Msg("Generating Gotify service URL")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("url", redactServiceURL(n.gotifyURL)).
			Msg("Gotify API URL loaded")
	}

	// Parse the API URL.
	apiURL, err := url.Parse(n.gotifyURL)
	if err != nil {
		clog.Debug().Err(err).Msg("Failed to parse Gotify URL")

		return "", fmt.Errorf("failed to generate Gotify URL: %w", err)
	}

	// Configure Gotify settings.
	config := &gotify.Config{
		Host:               apiURL.Host,
		Path:               apiURL.Path,
		DisableTLS:         apiURL.Scheme == "http",
		InsecureSkipVerify: n.gotifyInsecureSkipVerify,
		Token:              n.gotifyAppToken,
	}

	urlStr := config.GetURL().String()

	clog.Debug().
		Bool("disable_tls", apiURL.Scheme == "http").
		Bool("insecure_skip_verify", n.gotifyInsecureSkipVerify).
		Msg("Generated Gotify service URL")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("service_url", redactServiceURL(urlStr)).
			Msg("Generated Gotify service URL")
	}

	return urlStr, nil
}
