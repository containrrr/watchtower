package notifications

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	notifyConfig "github.com/nicholas-fedor/watchtower/internal/config/notify"
	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// msTeamsType is the identifier for Microsoft Teams notifications.
//
// Deprecated: Legacy msteams notification type is deprecated.
// Use --notification-url with a teams:// URL instead.
//
// TODO: Remove msTeamsType constant for the v2 release.
//
//nolint:godox
const msTeamsType = "msteams"

// Errors for Microsoft Teams notification configuration.
var (
	// errParseWebhookFailed indicates a failure to parse the Microsoft Teams webhook URL.
	errParseWebhookFailed = errors.New("failed to parse Microsoft Teams webhook URL")
)

// msTeamsTypeNotifier handles Microsoft Teams notifications via webhook.
//
// Deprecated: Legacy msteams notifier is deprecated.
// Use --notification-url with a teams:// URL instead.
//
// TODO: Remove msTeamsTypeNotifier for the v2 release.
//
//nolint:godox
type msTeamsTypeNotifier struct {
	webHookURL string
	log        *zerolog.Logger
}

// newMsTeamsNotifier creates a Teams notifier from resolved legacy settings.
//
// Parameters:
//   - log: Logger for configuration diagnostics and fatal validation errors.
//   - legacy: Deprecated MSTeams webhook settings (from process config or flags).
//
// Returns:
//   - types.ConvertibleNotifier: New Teams notifier instance.
//
// Deprecated: Legacy msteams notifier is deprecated.
// Use --notification-url with a teams:// URL instead.
//
// TODO: Remove newMsTeamsNotifier for the v2 release.
//
//nolint:godox
func newMsTeamsNotifier(log *zerolog.Logger, legacy notifyConfig.Legacy) types.ConvertibleNotifier {
	webHookURL := legacy.MSTeamsHook
	clog := log.With().Str("url", redactServiceURL(webHookURL)).Logger()

	if len(webHookURL) == 0 {
		clog.Fatal().Msg("Microsoft Teams webhook URL is empty")
	}

	return &msTeamsTypeNotifier{webHookURL: webHookURL, log: log}
}

// GetURL generates the Teams service URL from the notifier's webhook.
//
// Parameters:
//   - c: Cobra command (unused here).
//
// Returns:
//   - string: Teams service URL.
//   - error: Non-nil if parsing fails, nil on success.
//
// Deprecated: This method is part of the legacy msteams notifier and will be removed
// for the v2 release. Use --notification-url with a teams:// URL instead.
func (n *msTeamsTypeNotifier) GetURL(_ *cobra.Command) (string, error) {
	clog := n.log
	clog.Debug().Msg("Generating Microsoft Teams service URL")

	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("url", redactServiceURL(n.webHookURL)).
			Msg("Microsoft Teams webhook URL loaded")
	}

	// Validate the webhook URL is parseable and absolute.
	parsed, err := url.Parse(n.webHookURL)
	if err != nil {
		clog.Debug().Err(err).Msg("Failed to parse Microsoft Teams webhook URL")

		return "", fmt.Errorf("%w: %w", errParseWebhookFailed, err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("%w: expected https URL", errParseWebhookFailed)
	}

	// Create Teams config with the full webhook URL as the host.
	config := &teams.Config{
		Host:  n.webHookURL,
		Color: colorHex,
	}

	urlStr := config.GetURL().String()
	if clog.GetLevel() <= zerolog.TraceLevel {
		clog.Trace().
			Str("service_url", redactServiceURL(urlStr)).
			Msg("Generated Microsoft Teams service URL")
	} else {
		clog.Debug().Msg("Generated Microsoft Teams service URL")
	}

	return urlStr, nil
}
