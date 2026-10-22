package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/distribution/reference"
	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/internal/registry/hosts"
)

// challengeHeaderName is the HTTP Header containing challenge instructions.
const challengeHeaderName = "WWW-Authenticate"

// Errors for registry operations.
var (
	errFailedParseImageReference = errors.New("failed to parse image reference")
	errInvalidChallengeHeader    = errors.New("challenge header did not include all values needed to construct an auth url")
)

// challengeValues holds the parsed components of a WWW-Authenticate Bearer challenge.
type challengeValues struct {
	realm   string
	service string
	scope   string
}

// parseChallenge parses a Bearer challenge header into its components.
//
// It splits the header into key-value pairs and removes quotes from values.
// The header is expected to start with "bearer " (case-insensitive), where the
// scheme must be followed by whitespace or end of string. Parameter keys are
// matched case-insensitively, but value casing is preserved. Commas inside
// quoted values are not treated as delimiters.
//
// Parameters:
//   - header: The raw WWW-Authenticate header value.
//
// Returns:
//   - challengeValues: Parsed realm, service, and scope values.
func parseChallenge(header string) challengeValues {
	trimmed := strings.TrimSpace(header)
	lowered := strings.ToLower(trimmed)

	var raw string

	switch {
	case lowered == "bearer":
		raw = ""
	case strings.HasPrefix(lowered, "bearer "), strings.HasPrefix(lowered, "bearer\t"):
		raw = trimmed[6:]
	}

	parts := splitQuoted(raw, ',')

	var values challengeValues

	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)

		key, val, ok := strings.Cut(trimmedPart, "=")
		if ok {
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)

			switch strings.ToLower(key) {
			case "realm":
				values.realm = strings.Trim(val, `"`)
			case "service":
				values.service = strings.Trim(val, `"`)
			case "scope":
				values.scope = strings.Trim(val, `"`)
			}
		}
	}

	return values
}

// splitQuoted splits a string by a delimiter rune without splitting on
// delimiters that appear inside double-quoted sections.
//
// Parameters:
//   - input: The string to split.
//   - delim: The delimiter rune to split on.
//
// Returns:
//   - []string: The split segments, with quoted commas preserved.
func splitQuoted(input string, delim rune) []string {
	var (
		parts   []string
		current []rune
	)

	inQuotes := false

	for _, char := range input {
		switch {
		case char == '"':
			inQuotes = !inQuotes

			current = append(current, char)
		case char == delim && !inQuotes:
			parts = append(parts, string(current))

			current = current[:0]
		default:
			current = append(current, char)
		}
	}

	// Preserve trailing empty segment when the string ends with a delimiter.
	if len(current) > 0 || (len(input) > 0 && rune(input[len(input)-1]) == delim) {
		parts = append(parts, string(current))
	}

	return parts
}

// extractChallengeHost extracts the host from a realm URL.
//
// For example, "https://ghcr.io/token" yields "ghcr.io". It parses the trimmed
// realm with the URL parser and returns the parsed URL's Host for valid http or
// https URLs that name a host. Invalid or unsupported realms, and realms with a
// port but no host name, return an empty string.
//
// Parameters:
//   - log: Contextual logger.
//   - realm: The realm URL from the WWW-Authenticate header.
//
// Returns:
//   - string: The extracted host, or empty if extraction fails.
func extractChallengeHost(log *zerolog.Logger, realm string) string {
	realm = strings.TrimSpace(realm)
	log.Debug().
		Str("trimmed_realm", realm).
		Msg("Trimmed realm for host extraction")

	parsed, err := url.Parse(realm)
	if err != nil || parsed.Hostname() == "" {
		log.Debug().
			Str("realm", realm).
			Msg("Failed to extract challenge host from realm")

		return ""
	}

	switch parsed.Scheme {
	case "http", "https":
		return parsed.Host
	}

	log.Debug().
		Str("realm", realm).
		Str("scheme", parsed.Scheme).
		Msg("Unsupported realm URL scheme for challenge host extraction")

	return ""
}

// GetRegistryAddress extracts the registry address from an image reference.
//
// It returns the domain part of the reference, mapping Docker Hub's default
// domain to its canonical host if needed. [hosts.LSCRRegistryDomain] is mapped
// to [hosts.GitHubRegistryDomain] since those images are hosted on GHCR.
//
// Parameters:
//   - log: Logger for diagnostics.
//   - imageRef: Image reference string (for example "docker.io/library/alpine").
//
// Returns:
//   - string: Registry address (for example "index.docker.io") if successful.
//   - error: Non-nil if parsing fails, nil on success.
func GetRegistryAddress(log *zerolog.Logger, imageRef string) (string, error) {
	// Parse the image reference into a normalized form for consistent domain extraction.
	normalizedRef, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image_ref", imageRef).
			Msg("Failed to parse image reference")

		return "", fmt.Errorf("%w: %w", errFailedParseImageReference, err)
	}

	// Extract the domain from the normalized reference.
	domain := reference.Domain(normalizedRef)

	// Map Docker Hub's default domain to its canonical host for registry requests.
	if domain == hosts.DockerRegistryDomain {
		domain = hosts.DockerRegistryHost

		log.Debug().
			Str("image_ref", imageRef).
			Str("address", domain).
			Msg("Mapped Docker Hub domain to canonical host")
	}

	// lscr.io images are hosted on GitHub Container Registry (ghcr.io).
	// Map here so all callers benefit, including getChallengeURL and getAuthURL.
	if domain == hosts.LSCRRegistryDomain {
		domain = hosts.GitHubRegistryDomain

		log.Debug().
			Str("image_ref", imageRef).
			Str("address", domain).
			Msg("Mapped lscr.io to ghcr.io")
	}

	log.Debug().
		Str("image_ref", imageRef).
		Str("address", domain).
		Msg("Extracted registry address")

	return domain, nil
}
