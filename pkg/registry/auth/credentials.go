package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// TransformAuth converts a base64-encoded JSON object into a base64-encoded
// "username:password" string.
//
// It decodes the input, extracts username and password from a
// RegistryCredentials struct, and re-encodes them for use in HTTP Basic
// Authentication headers, ensuring compatibility with registry requirements.
//
// Parameters:
//   - registryAuth: A base64-encoded string, typically a JSON object with username and password fields.
//
// Returns:
//   - string: A base64-encoded "username:password" string if credentials are present, otherwise the original input.
func TransformAuth(log *zerolog.Logger, registryAuth string) string {
	if registryAuth == "" {
		return ""
	}

	// registry.encodeCredentials produces URLEncoding. StdEncoding is tried
	// first for credentials produced outside Watchtower, then URLEncoding.
	b, err := base64.StdEncoding.DecodeString(registryAuth)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(registryAuth)
	}

	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to decode base64 registry auth - returning original input")

		return registryAuth
	}

	credentials := &types.RegistryCredentials{}

	err = json.Unmarshal(b, credentials)
	if err != nil {
		log.Debug().
			Err(err).
			Msg("Failed to unmarshal registry credentials JSON - returning original input")

		return registryAuth
	}

	username := credentials.Username
	password := credentials.Password

	// Identity tokens from credential helpers are presented as the password in
	// HTTP Basic auth when exchanging for a registry bearer token.
	if password == "" && credentials.IdentityToken != "" {
		password = credentials.IdentityToken
	}

	if password == "" {
		return registryAuth
	}

	basicAuth := fmt.Appendf(nil, "%s:%s", username, password)
	registryAuth = base64.StdEncoding.EncodeToString(basicAuth)

	log.Debug().Msg("Transformed registry credentials to Basic auth format")

	return registryAuth
}
