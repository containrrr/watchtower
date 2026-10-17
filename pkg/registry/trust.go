package registry

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/maypok86/otter/v2/stats"
	"github.com/rs/zerolog"

	dockerCliConfig "github.com/docker/cli/cli/config"
	dockerConfigConfigfile "github.com/docker/cli/cli/config/configfile"
	dockerConfigCredentials "github.com/docker/cli/cli/config/credentials"
	dockerConfig "github.com/docker/cli/cli/config/types"

	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
)

const (
	// encodedAuthCacheMaximumSize is the maximum number of encoded auth entries.
	encodedAuthCacheMaximumSize = 256
	// encodedAuthCacheInitialCapacity is the initial otter cache capacity.
	encodedAuthCacheInitialCapacity = 16
	// encodedAuthTTL is how long encoded registry auth is reused.
	encodedAuthTTL = 2 * time.Minute
)

// Errors for registry authentication operations.
var (
	// errUnsetRegAuthVars indicates registry auth environment variables (REPO_USER, REPO_PASS) are not set.
	errUnsetRegAuthVars = errors.New(
		"registry auth environment variables (REPO_USER, REPO_PASS) not set",
	)
	// errFailedGetRegistryAddress indicates a failure to extract the registry address from an image reference.
	errFailedGetRegistryAddress = errors.New("failed to get registry address")
	// errFailedLoadDockerConfig indicates a failure to load the Docker configuration file.
	errFailedLoadDockerConfig = errors.New("failed to load Docker config")
	// errFailedMarshalAuthConfig indicates a failure to marshal the auth config to JSON.
	errFailedMarshalAuthConfig = errors.New("failed to marshal auth config to JSON")
	// errFailedGetCredentials indicates a failure to retrieve credentials from the Docker store.
	errFailedGetCredentials = errors.New("failed to get registry credentials")
)

var (
	// encodedAuthCache holds encoded Docker-config credentials by host and config dir.
	encodedAuthCache *otter.Cache[string, encodedAuthEntry]
	// encodedAuthCacheOnce initializes encodedAuthCache exactly once.
	encodedAuthCacheOnce sync.Once
)

// encodedAuthEntry stores encoded credentials and the Docker config mtime.
type encodedAuthEntry struct {
	encoded string
	mtime   int64
}

// encodedAuthExpiryCalculator applies encodedAuthTTL to otter cache entries.
type encodedAuthExpiryCalculator struct{}

// ExpireAfterCreate returns the TTL for a newly created auth cache entry.
func (e *encodedAuthExpiryCalculator) ExpireAfterCreate(_ otter.Entry[string, encodedAuthEntry]) time.Duration {
	return encodedAuthTTL
}

// ExpireAfterUpdate returns the TTL for an updated auth cache entry.
func (e *encodedAuthExpiryCalculator) ExpireAfterUpdate(_ otter.Entry[string, encodedAuthEntry], _ encodedAuthEntry) time.Duration {
	return encodedAuthTTL
}

// ExpireAfterRead preserves the remaining TTL when an auth cache entry is read.
func (e *encodedAuthExpiryCalculator) ExpireAfterRead(entry otter.Entry[string, encodedAuthEntry]) time.Duration {
	return entry.ExpiresAfter()
}

// initEncodedAuthCache initializes the encoded registry auth cache once.
//
// Otter construction failure panics because auth lookup cannot proceed without it.
func initEncodedAuthCache() {
	encodedAuthCacheOnce.Do(func() {
		cache, err := otter.New(
			&otter.Options[string, encodedAuthEntry]{
				MaximumSize:      encodedAuthCacheMaximumSize,
				InitialCapacity:  encodedAuthCacheInitialCapacity,
				ExpiryCalculator: &encodedAuthExpiryCalculator{},
				StatsRecorder:    stats.NewCounter(),
			},
		)
		if err != nil {
			panic("failed to initialize encoded auth cache: " + err.Error())
		}

		encodedAuthCache = cache
	})
}

// resetEncodedAuthCache clears cached encoded auth entries.
//
// Tests use this to isolate Docker config mutations between cases.
func resetEncodedAuthCache() {
	initEncodedAuthCache()
	encodedAuthCache.InvalidateAll()
}

// encodedAuthCacheKey builds a cache key from registry host and config directory.
//
// Parameters:
//   - server: Registry host address.
//   - configDir: Docker config directory.
//
// Returns:
//   - string: Cache key unique to that host and config path.
func encodedAuthCacheKey(server, configDir string) string {
	return server + "\x00" + configDir
}

// configFileModTime returns the config.json mtime in nanoseconds, or 0 if missing.
//
// Parameters:
//   - configDir: Docker config directory.
//
// Returns:
//   - int64: File modification time in nanoseconds, or 0 when the file is absent.
func configFileModTime(configDir string) int64 {
	configPath := filepath.Join(filepath.Clean(configDir), "config.json")

	//nolint:gosec // G703: Path traversal via taint analysis - DOCKER_CONFIG is an operator-provided directory.
	info, err := os.Stat(configPath)
	if err != nil {
		return 0
	}

	return info.ModTime().UnixNano()
}

// encodedAuth attempts to retrieve encoded authentication credentials for a given image name.
//
// Per-image Docker config credentials are preferred when present so REPO_USER/REPO_PASS
// are not sent to every registry. Environment credentials are used when the config has
// no entry for the image's registry (or config lookup fails), preserving the common
// single-registry REPO_USER/REPO_PASS deployment.
//
// Parameters:
//   - imageName: Image reference string (e.g., "docker.io/library/alpine").
//
// Returns:
//   - string: Base64-encoded credentials string if successful, empty if none found.
//   - error: Non-nil if both methods fail, nil on success or if no credentials are available.
func encodedAuth(log *zerolog.Logger, imageName string) (string, error) {
	// Set up logging fields for tracking.
	fields := map[string]any{
		"image_ref": imageName,
	}

	log.Debug().
		Fields(fields).
		Msg("Attempting to retrieve auth credentials")

	configCredentials, configErr := encodedConfigCredentials(log, imageName)
	if configErr == nil && configCredentials != "" {
		log.Debug().
			Fields(fields).
			Msg("Successfully retrieved encoded auth credentials from config")

		return configCredentials, nil
	}

	if configErr != nil {
		log.Debug().
			Err(configErr).
			Fields(fields).
			Msg("Config auth not available, trying environment")
	} else {
		log.Debug().
			Fields(fields).
			Msg("No config credentials for registry, trying environment")
	}

	credentials, err := encodedEnvAuth(log)
	if err != nil {
		// Prefer surfacing a config load/address error when env is also unset.
		if configErr != nil {
			return "", configErr
		}

		// No config entry and no env: empty credentials is success (anonymous pull).
		return "", nil
	}

	if credentials != "" {
		log.Debug().
			Fields(fields).
			Msg("Successfully retrieved encoded auth credentials from environment")
	}

	return credentials, nil
}

// encodedEnvAuth checks for REPO_USER and REPO_PASS environment variables and encodes them.
//
// It returns an error if these variables are not set.
//
// Returns:
//   - string: Base64-encoded auth string if credentials are found.
//   - error: Non-nil if env vars are missing, nil on success.
func encodedEnvAuth(log *zerolog.Logger) (string, error) {
	// Retrieve username and password from environment.
	username := os.Getenv("REPO_USER")
	password := os.Getenv("REPO_PASS")

	// Check if both variables are set.
	if username != "" && password != "" {
		credentials := dockerConfig.AuthConfig{
			Username: username,
			Password: password,
		}

		log.Debug().
			Bool("has_username", true).
			Msg("Loaded auth credentials from environment")

		// Trace only non-sensitive presence indicators. Never log password or tokens.
		if log.GetLevel() == zerolog.TraceLevel {
			log.Trace().
				Bool("has_username", true).
				Bool("has_password", true).
				Msg("Using environment credentials")
		}

		// Encode and return the auth config.
		return encodeCredentials(log, credentials)
	}

	// Return error if variables are missing.
	log.Debug().Msg("Environment auth variables not set")

	return "", errUnsetRegAuthVars
}

// encodedConfigCredentials retrieves authentication credentials from the Docker config file.
//
// Successful and empty lookups are cached by registry host and config directory.
// The cache is invalidated when config.json mtime changes or the TTL expires.
// Failed loads are not cached.
//
// Parameters:
//   - log: Logger for debug output.
//   - imageRef: Image reference string for registry lookup.
//
// Returns:
//   - string: Base64-encoded credentials string if found, empty if none.
//   - error: Non-nil if config loading or address retrieval fails, nil on success or if no auth is found.
func encodedConfigCredentials(log *zerolog.Logger, imageRef string) (string, error) {
	// Get the registry server address from the image reference.
	server, err := auth.GetRegistryAddress(log, imageRef)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image_ref", imageRef).
			Msg("Failed to get registry address")

		return "", fmt.Errorf("%w: %w", errFailedGetRegistryAddress, err)
	}

	// Use DOCKER_CONFIG or default to the root directory.
	configDir := os.Getenv("DOCKER_CONFIG")
	if configDir == "" {
		configDir = "/"

		log.Debug().
			Str("image_ref", imageRef).
			Msg("No DOCKER_CONFIG set, using default directory")
	}

	initEncodedAuthCache()

	mtime := configFileModTime(configDir)
	cacheKey := encodedAuthCacheKey(server, configDir)

	// Reuse encoded auth when the Docker config file has not changed.
	if entry, ok := encodedAuthCache.GetIfPresent(cacheKey); ok && entry.mtime == mtime {
		log.Debug().
			Str("image_ref", imageRef).
			Str("server", server).
			Msg("Using cached registry auth credentials")

		return entry.encoded, nil
	}

	// Load the Docker config file from the specified directory.
	configFile, err := dockerCliConfig.Load(configDir)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image_ref", imageRef).
			Str("config_dir", configDir).
			Msg("Failed to load Docker config")

		return "", fmt.Errorf("%w: %w", errFailedLoadDockerConfig, err)
	}

	// Retrieve credentials from the config store.
	credStore := credentialsStore(*configFile)

	credentials, err := credStore.Get(server)
	if err != nil {
		log.Debug().
			Err(err).
			Str("image_ref", imageRef).
			Str("server", server).
			Msg("Failed to get registry credentials")

		return "", fmt.Errorf("%w: %w", errFailedGetCredentials, err)
	}

	// Empty AuthConfig is a miss. Cache it so later lookups skip Load.
	if !hasUsableRegistryCredentials(credentials) {
		log.Debug().
			Str("image_ref", imageRef).
			Str("server", server).
			Str("config_file", configFile.Filename).
			Msg("No credentials found in config")

		encodedAuthCache.Set(cacheKey, encodedAuthEntry{mtime: mtime})

		return "", nil
	}

	// Log successful retrieval with non-sensitive presence flags only.
	log.Debug().
		Str("image_ref", imageRef).
		Bool("has_username", credentials.Username != "").
		Bool("has_password", credentials.Password != "").
		Bool("has_identity_tok", credentials.IdentityToken != "").
		Str("server", server).
		Str("config_file", configFile.Filename).
		Msg("Loaded auth credentials from config")

	if log.GetLevel() == zerolog.TraceLevel {
		log.Trace().
			Str("image_ref", imageRef).
			Bool("has_username", credentials.Username != "").
			Bool("has_password", credentials.Password != "").
			Bool("has_identity_tok", credentials.IdentityToken != "").
			Str("server", server).
			Msg("Using config credentials")
	}

	// Encode the auth config, including IdentityToken when set.
	encoded, err := encodeCredentials(log, credentials)
	if err != nil {
		return "", err
	}

	encodedAuthCache.Set(cacheKey, encodedAuthEntry{encoded: encoded, mtime: mtime})

	return encoded, nil
}

// hasUsableRegistryCredentials reports whether AuthConfig carries material the
// Docker daemon or registry HTTP clients can authenticate with.
//
// IdentityToken and Password are supported end to end via encodeCredentials and
// TransformAuth (Basic auth). RegistryToken-only entries are not accepted until
// a Bearer-auth path is implemented for them.
func hasUsableRegistryCredentials(credentials dockerConfig.AuthConfig) bool {
	if credentials == (dockerConfig.AuthConfig{}) {
		return false
	}

	if credentials.IdentityToken != "" {
		return true
	}

	// Password-only entries are used for PAT-style tokens (for example GHCR).
	if credentials.Password != "" {
		return true
	}

	return false
}

// credentialsStore returns a new credentials store based on the configuration file settings.
//
// It selects a native or file-based store depending on the config.
//
// Parameters:
//   - configFile: Docker configuration file.
//
// Returns:
//   - dockerConfigCredentials.Store: Configured credentials store.
func credentialsStore(configFile dockerConfigConfigfile.ConfigFile) dockerConfigCredentials.Store {
	// Use native store if a credentials store is specified.
	if configFile.CredentialsStore != "" {
		return dockerConfigCredentials.NewNativeStore(&configFile, configFile.CredentialsStore)
	}

	// Default to file-based store otherwise.
	return dockerConfigCredentials.NewFileStore(&configFile)
}

// encodeCredentials Base64 encodes an AuthConfig struct for HTTP transmission.
//
// It marshals the struct to JSON and applies URL-safe base64 encoding.
//
// Parameters:
//   - authConfig: Authentication configuration to encode.
//
// Returns:
//   - string: Base64-encoded auth string if successful.
//   - error: Non-nil if marshaling fails, nil on success.
func encodeCredentials(log *zerolog.Logger, authConfig dockerConfig.AuthConfig) (string, error) {
	// Set up logging fields with non-sensitive username presence indicator.
	fields := map[string]any{
		"has_username": authConfig.Username != "",
	}

	// Marshal the auth config to JSON.
	//nolint:gosec // G117: This is the expected standard Docker auth format
	buf, err := json.Marshal(authConfig)
	if err != nil {
		log.Debug().
			Err(err).
			Fields(fields).
			Msg("Failed to marshal auth config to JSON")

		return "", fmt.Errorf("%w: %w", errFailedMarshalAuthConfig, err)
	}

	// Encode the JSON to base64 for safe transmission.
	encoded := base64.URLEncoding.EncodeToString(buf)

	log.Debug().
		Fields(fields).
		Msg("Encoded auth config")

	return encoded, nil
}
