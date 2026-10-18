package registry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dockerCliConfig "github.com/docker/cli/cli/config"

	"github.com/nicholas-fedor/watchtower/internal/logging"
	"github.com/nicholas-fedor/watchtower/pkg/registry/auth"
)

// TestEncodedEnvAuth_ReturnsCredentialsWhenSet verifies that encodedEnvAuth
// returns base64-encoded credentials when REPO_USER and REPO_PASS are set.
func TestEncodedEnvAuth_ReturnsCredentialsWhenSet(t *testing.T) {
	expected := "eyJ1c2VybmFtZSI6IndhdGNodG93ZXItdXNlciIsInBhc3N3b3JkIjoid2F0Y2h0b3dlci1wYXNzIn0="

	t.Setenv("REPO_USER", "watchtower-user")
	t.Setenv("REPO_PASS", "watchtower-pass")

	config, err := encodedEnvAuth(newTestLogger(t))
	require.NoError(t, err)
	assert.Equal(t, expected, config)
}

// TestEncodedEnvAuth_ReturnsErrorWhenUnset verifies that encodedEnvAuth
// returns an error when REPO_USER and REPO_PASS are not set.
func TestEncodedEnvAuth_ReturnsErrorWhenUnset(t *testing.T) {
	t.Setenv("REPO_USER", "")
	t.Setenv("REPO_PASS", "")

	_, err := encodedEnvAuth(newTestLogger(t))
	require.Error(t, err)
}

// TestEncodedEnvAuth_PartialCredentials verifies that encodedEnvAuth returns
// an error when only one of REPO_USER or REPO_PASS is set.
func TestEncodedEnvAuth_PartialCredentials(t *testing.T) {
	tests := []struct {
		name     string
		repoUser string
		repoPass string
	}{
		{
			name:     "username set but password missing",
			repoUser: "partial-user",
			repoPass: "",
		},
		{
			name:     "password set but username missing",
			repoUser: "",
			repoPass: "partial-pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REPO_USER", tt.repoUser)
			t.Setenv("REPO_PASS", tt.repoPass)

			credentials, err := encodedEnvAuth(newTestLogger(t))
			require.Error(t, err)
			assert.Empty(t, credentials)
		})
	}
}

// TestEncodedConfigAuth_ReturnsEmptyCredentialsWhenFileNotPresent verifies that
// encodedConfigCredentials returns empty credentials when the Docker config directory
// does not contain a valid config file.
func TestEncodedConfigAuth_ReturnsEmptyCredentialsWhenFileNotPresent(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", "/nonexistent/watchtower-test-path")

	credentials, err := encodedConfigCredentials(newTestLogger(t), "docker.io/library/nginx:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_FileStoreNoUsername tests that encodedConfigCredentials
// returns empty string and nil error when the Docker config file's auth entry has
// no username (covers the empty-username guard added for native store compatibility).
func TestEncodedConfigCredentials_FileStoreNoUsername(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"serveraddress": "ghcr.io",
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	writeErr := os.WriteFile(configPath, configContent, 0o600)
	require.NoError(t, writeErr)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	normalizedRef, parseErr := reference.ParseNormalizedNamed("ghcr.io/test/image:latest")
	require.NoError(t, parseErr)

	credentials, err := encodedConfigCredentials(newTestLogger(t), normalizedRef.String())
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_FileStoreUsernameOnly tests that encodedConfigCredentials
// returns empty string when the Docker config auth entry has a username but no
// password or identity token (not usable for registry auth).
func TestEncodedConfigCredentials_FileStoreUsernameOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"serveraddress": "ghcr.io",
				"username":      "testuser",
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	writeErr := os.WriteFile(configPath, configContent, 0o600)
	require.NoError(t, writeErr)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	normalizedRef, parseErr := reference.ParseNormalizedNamed("ghcr.io/test/image:latest")
	require.NoError(t, parseErr)

	credentials, err := encodedConfigCredentials(newTestLogger(t), normalizedRef.String())
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_IdentityToken accepts ECR-style identity tokens
// without username/password and transforms them to HTTP Basic for bearer exchange.
func TestEncodedConfigCredentials_IdentityToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	const identityToken = "eyJlY3ItdG9rZW4iOiJ0ZXN0In0="

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"123456789012.dkr.ecr.us-east-1.amazonaws.com": map[string]string{
				"identitytoken": identityToken,
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, configContent, 0o600))

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	credentials, err := encodedConfigCredentials(newTestLogger(t), "123456789012.dkr.ecr.us-east-1.amazonaws.com/app:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, identityToken, authConfig["identitytoken"])

	// End-to-end: TransformAuth must produce Basic credentials for the
	// Authorization header used when exchanging for a registry bearer token.
	basicEncoded := auth.TransformAuth(newTestLogger(t), credentials)
	basicDecoded, decodeErr := base64.StdEncoding.DecodeString(basicEncoded)
	require.NoError(t, decodeErr)
	assert.Equal(t, ":"+identityToken, string(basicDecoded))

	authHeader := "Basic " + basicEncoded
	assert.True(t, strings.HasPrefix(authHeader, "Basic "))
	assert.NotContains(t, authHeader, "Bearer ")
}

// TestEncodedConfigCredentials_RegistryTokenOnlyRejected ensures registrytoken-only
// entries are not treated as usable until a Bearer-auth path exists for them.
func TestEncodedConfigCredentials_RegistryTokenOnlyRejected(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"registrytoken": "bearer-only-token",
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, configContent, 0o600))

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_PasswordOnly accepts password-as-token entries
// with an empty username (common for GHCR PATs stored in config).
func TestEncodedConfigCredentials_PasswordOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"password": "ghp_only_token",
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, configContent, 0o600))

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "ghp_only_token", authConfig["password"])
}

// TestEncodedConfigCredentials_FileStoreValidCredentials tests the happy path
// where the Docker config file contains valid username and password.
func TestEncodedConfigCredentials_FileStoreValidCredentials(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configContent, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"username": "testuser",
				"password": "testpass",
			},
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	writeErr := os.WriteFile(configPath, configContent, 0o600)
	require.NoError(t, writeErr)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	normalizedRef, parseErr := reference.ParseNormalizedNamed("ghcr.io/test/image:latest")
	require.NoError(t, parseErr)

	credentials, err := encodedConfigCredentials(newTestLogger(t), normalizedRef.String())
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "testuser", authConfig["username"])
	assert.Equal(t, "testpass", authConfig["password"])
}

// TestEncodedConfigCredentials_NoConfigFile tests that encodedConfigCredentials
// returns empty credentials and nil error when the Docker config directory
// does not exist, since the file store gracefully handles missing config files.
func TestEncodedConfigCredentials_NoConfigFile(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", "/nonexistent/watchtower-test-path")
	dockerCliConfig.SetDir("/nonexistent/watchtower-test-path")

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedAuth_UsesConfigWhenEnvUnset verifies that encodedAuth falls
// through to encodedConfigCredentials when REPO_USER/REPO_PASS are unset
// and a config file is present.
func TestEncodedAuth_UsesConfigWhenEnvUnset(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "cfguser",
			"password": "cfgpass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	t.Setenv("REPO_USER", "")
	t.Setenv("REPO_PASS", "")

	credentials, err := encodedAuth(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "cfguser", authConfig["username"])
	assert.Equal(t, "cfgpass", authConfig["password"])
}

// TestEncodedConfigCredentials_MultipleRegistries verifies that a Docker config
// file with credentials for multiple registries returns the correct distinct
// credentials for each registry.
func TestEncodedConfigCredentials_MultipleRegistries(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "ghcr-user",
			"password": "ghcr-pass",
		},
		"registry.example.com": {
			"username": "example-user",
			"password": "example-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	tests := []struct {
		name      string
		imageRef  string
		wantUser  string
		wantPass  string
		wantEmpty bool
	}{
		{
			name:     "ghcr.io returns ghcr credentials",
			imageRef: "ghcr.io/test/image:latest",
			wantUser: "ghcr-user",
			wantPass: "ghcr-pass",
		},
		{
			name:     "registry.example.com returns example credentials",
			imageRef: "registry.example.com/org/image:latest",
			wantUser: "example-user",
			wantPass: "example-pass",
		},
		{
			name:      "unconfigured registry returns empty",
			imageRef:  "docker.io/library/alpine:latest",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentials, err := encodedConfigCredentials(newTestLogger(t), tt.imageRef)
			require.NoError(t, err)

			if tt.wantEmpty {
				assert.Empty(t, credentials)

				return
			}

			assert.NotEmpty(t, credentials)

			authConfig := decodeEncodedAuth(t, credentials)
			assert.Equal(t, tt.wantUser, authConfig["username"])
			assert.Equal(t, tt.wantPass, authConfig["password"])
		})
	}
}

// TestEncodedAuth_ConfigPreferredOverEnv verifies that encodedAuth returns
// Docker config credentials for a registry when both config and REPO_USER/REPO_PASS
// are set, so env credentials are not sent to every registry.
func TestEncodedAuth_ConfigPreferredOverEnv(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "cfg-user",
			"password": "cfg-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	t.Setenv("REPO_USER", "env-user")
	t.Setenv("REPO_PASS", "env-pass")

	credentials, err := encodedAuth(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "cfg-user", authConfig["username"])
	assert.Equal(t, "cfg-pass", authConfig["password"])
}

// TestEncodedAuth_EnvUsedWhenConfigHasNoRegistryEntry verifies REPO_USER/REPO_PASS
// still apply when the Docker config has no credentials for the image's registry.
func TestEncodedAuth_EnvUsedWhenConfigHasNoRegistryEntry(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "cfg-user",
			"password": "cfg-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	t.Setenv("REPO_USER", "env-user")
	t.Setenv("REPO_PASS", "env-pass")

	credentials, err := encodedAuth(newTestLogger(t), "quay.io/org/app:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "env-user", authConfig["username"])
	assert.Equal(t, "env-pass", authConfig["password"])
}

// TestEncodedConfigCredentials_MultipleImagesSameRegistry verifies that
// different images from the same registry receive identical credentials.
func TestEncodedConfigCredentials_MultipleImagesSameRegistry(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "shared-user",
			"password": "shared-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	imageRefs := []string{
		"ghcr.io/org/image-a:latest",
		"ghcr.io/org/image-b:v1.2.3",
		"ghcr.io/another/repo:tag",
	}

	var firstCredentials string

	for _, ref := range imageRefs {
		credentials, err := encodedConfigCredentials(newTestLogger(t), ref)
		require.NoError(t, err)
		assert.NotEmpty(t, credentials)

		if firstCredentials == "" {
			firstCredentials = credentials
		} else {
			assert.Equal(t, firstCredentials, credentials,
				"expected identical credentials for all images from same registry")
		}
	}
}

// TestEncodedConfigCredentials_RegistryMissingFromConfig verifies that
// encodedConfigCredentials returns empty credentials when the config file
// contains entries for other registries but not the requested one.
func TestEncodedConfigCredentials_RegistryMissingFromConfig(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"docker.io": {
			"username": "docker-user",
			"password": "docker-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_EmptyAuthsMap verifies that a config file with
// an empty `auths` map returns empty credentials without error.
func TestEncodedConfigCredentials_EmptyAuthsMap(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{})

	defer os.Unsetenv("DOCKER_CONFIG")

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedConfigCredentials_MalformedConfigJSON verifies that encodedConfigCredentials
// returns an error when the config file contains malformed JSON.
func TestEncodedConfigCredentials_MalformedConfigJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	writeErr := os.WriteFile(configPath, []byte("not valid json {{{"), 0o600)
	require.NoError(t, writeErr)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	credentials, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/test/image:latest")
	require.Error(t, err)
	assert.Empty(t, credentials)
}

// TestEncodedAuth_FallsThroughToConfigWhenEnvPartiallySet verifies that
// encodedAuth falls through to encodedConfigCredentials when env vars are
// partially set (one present, one missing) and a valid config file exists.
func TestEncodedAuth_FallsThroughToConfigWhenEnvPartiallySet(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "cfg-user",
			"password": "cfg-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	t.Setenv("REPO_USER", "env-only-user")
	t.Setenv("REPO_PASS", "")

	credentials, err := encodedAuth(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.NotEmpty(t, credentials)

	authConfig := decodeEncodedAuth(t, credentials)
	assert.Equal(t, "cfg-user", authConfig["username"])
	assert.Equal(t, "cfg-pass", authConfig["password"])
}

// TestEncodedAuth_EnvPartiallySetWithUnconfiguredRegistry verifies that
// encodedAuth returns empty credentials without error when env vars are
// partially set and the config file has no entry for the requested registry.
func TestEncodedAuth_EnvPartiallySetWithUnconfiguredRegistry(t *testing.T) {
	writeTestDockerConfig(t, map[string]map[string]string{
		"docker.io": {
			"username": "docker-user",
			"password": "docker-pass",
		},
	})

	defer os.Unsetenv("DOCKER_CONFIG")

	t.Setenv("REPO_USER", "env-only-user")
	t.Setenv("REPO_PASS", "")

	credentials, err := encodedAuth(newTestLogger(t), "ghcr.io/test/image:latest")
	require.NoError(t, err)
	assert.Empty(t, credentials)
}

// TestGetPullOptions_TraceLogsOmitCredentials verifies that GetPullOptions trace
// logging never includes the encoded registry credential payload.
func TestGetPullOptions_TraceLogsOmitCredentials(t *testing.T) {
	const secretPass = "super-secret-pull-pass"

	logBuf := &bytes.Buffer{}
	log := logging.New(logging.LogfmtWriter(logBuf), logging.TraceLevel)

	// Empty Docker config so encodedAuth falls through to environment credentials.
	tempDir := t.TempDir()
	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	t.Setenv("REPO_USER", "pull-user")
	t.Setenv("REPO_PASS", secretPass)

	opts, err := GetPullOptions(log, "docker.io/library/alpine:latest")
	require.NoError(t, err)
	require.NotEmpty(t, opts.RegistryAuth)

	out := logBuf.String()
	assert.Contains(t, out, "Retrieved authentication credentials")
	assert.Contains(t, out, "has_credentials")
	assert.NotContains(t, out, secretPass)
	assert.NotContains(t, out, opts.RegistryAuth)
}

// TestEncodedAuth_TraceLogsOmitSecrets verifies that trace-level credential logs
// never include password or token material from config or environment sources.
// The capture logger is Trace-level and is passed into production so Trace branches run.
func TestEncodedAuth_TraceLogsOmitSecrets(t *testing.T) {
	const (
		secretPass  = "super-secret-repo-pass"
		secretToken = "super-secret-identity-token"
	)

	logBuf := &bytes.Buffer{}
	log := logging.New(logging.LogfmtWriter(logBuf), logging.TraceLevel)

	t.Run("environment credentials", func(t *testing.T) {
		logBuf.Reset()

		t.Setenv("REPO_USER", "env-user")
		t.Setenv("REPO_PASS", secretPass)

		_, err := encodedEnvAuth(log)
		require.NoError(t, err)

		out := logBuf.String()
		// Positive: Trace path ran and logged non-sensitive indicators.
		assert.Contains(t, out, "Using environment credentials")
		assert.Contains(t, out, "has_username=true")
		assert.Contains(t, out, "has_password=true")
		assert.NotContains(t, out, secretPass)
		assert.NotContains(t, out, "env-user")
	})

	t.Run("config credentials", func(t *testing.T) {
		logBuf.Reset()

		writeTestDockerConfig(t, map[string]map[string]string{
			"ghcr.io": {
				"username": "cfg-user",
				"password": secretPass,
			},
		})

		t.Setenv("REPO_USER", "")
		t.Setenv("REPO_PASS", "")

		_, err := encodedConfigCredentials(log, "ghcr.io/org/app:latest")
		require.NoError(t, err)

		out := logBuf.String()
		assert.Contains(t, out, "Using config credentials")
		assert.Contains(t, out, "has_username=true")
		assert.Contains(t, out, "has_password=true")
		assert.NotContains(t, out, secretPass)
		assert.NotContains(t, out, "cfg-user")
	})

	t.Run("identity token credentials", func(t *testing.T) {
		logBuf.Reset()

		tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
		require.NoError(t, err)

		defer os.RemoveAll(tempDir)

		configContent, err := json.Marshal(map[string]any{
			"auths": map[string]any{
				"ghcr.io": map[string]string{
					"identitytoken": secretToken,
				},
			},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "config.json"), configContent, 0o600))

		t.Setenv("DOCKER_CONFIG", tempDir)
		t.Setenv("HOME", tempDir)
		dockerCliConfig.SetDir(tempDir)

		_, err = encodedConfigCredentials(log, "ghcr.io/org/app:latest")
		require.NoError(t, err)

		out := logBuf.String()
		assert.Contains(t, out, "Using config credentials")
		assert.Contains(t, out, "has_identity_tok")
		assert.NotContains(t, out, secretToken)
	})
}

// TestEncodedConfigCredentials_InvalidImageRef verifies that encodedConfigCredentials
// returns an error when the image reference is invalid and cannot be parsed.
func TestEncodedConfigCredentials_InvalidImageRef(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	defer os.RemoveAll(tempDir)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	credentials, err := encodedConfigCredentials(newTestLogger(t), "")
	require.Error(t, err)
	assert.Empty(t, credentials)
}

// resetEncodedAuthCache clears cached encoded auth entries, so Docker config
// changes in one test do not leak into another.
func resetEncodedAuthCache() {
	initEncodedAuthCache()
	encodedAuthCache.InvalidateAll()
}

func TestEncodedConfigCredentials_CacheHitAndMtimeInvalidation(t *testing.T) {
	resetEncodedAuthCache()
	t.Cleanup(resetEncodedAuthCache)

	tempDir := writeTestDockerConfig(t, map[string]map[string]string{
		"ghcr.io": {
			"username": "cache-user",
			"password": "cache-pass",
		},
	})
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	first, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	require.NotEmpty(t, first)

	second, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	assert.Equal(t, first, second)

	configPath := filepath.Join(tempDir, "config.json")
	updated, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"username": "cache-user",
				"password": "rotated-pass",
			},
		},
	})
	require.NoError(t, err)

	// Ensure mtime changes even on coarse filesystems.
	past := configFileModTime(tempDir) - int64(time.Second)
	require.NoError(t, os.Chtimes(configPath, time.Unix(0, past), time.Unix(0, past)))
	require.NoError(t, os.WriteFile(configPath, updated, 0o600))

	future := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(configPath, future, future))

	third, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	require.NotEmpty(t, third)
	assert.NotEqual(t, first, third)
}

func TestEncodedConfigCredentials_FailedLookupNotCached(t *testing.T) {
	resetEncodedAuthCache()
	t.Cleanup(resetEncodedAuthCache)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{"), 0o600))

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	mtime := configFileModTime(tempDir)

	empty, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.Error(t, err)
	require.ErrorIs(t, err, errFailedLoadDockerConfig)
	assert.Empty(t, empty)

	valid, err := json.Marshal(map[string]any{
		"auths": map[string]any{
			"ghcr.io": map[string]string{
				"username": "later-user",
				"password": "later-pass",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, valid, 0o600))
	require.NoError(t, os.Chtimes(configPath, time.Unix(0, mtime), time.Unix(0, mtime)))

	got, err := encodedConfigCredentials(newTestLogger(t), "ghcr.io/org/app:latest")
	require.NoError(t, err)
	require.NotEmpty(t, got)
}

// writeTestDockerConfig writes a Docker config.json with the given auths map
// and returns the temp directory path. The caller is responsible for cleanup.
func writeTestDockerConfig(t *testing.T, auths map[string]map[string]string) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "watchtower-test-docker-config")
	require.NoError(t, err)

	configContent, err := json.Marshal(map[string]any{
		"auths": auths,
	})
	require.NoError(t, err)

	configPath := filepath.Join(tempDir, "config.json")
	writeErr := os.WriteFile(configPath, configContent, 0o600)
	require.NoError(t, writeErr)

	t.Setenv("DOCKER_CONFIG", tempDir)
	t.Setenv("HOME", tempDir)
	dockerCliConfig.SetDir(tempDir)

	return tempDir
}

// decodeEncodedAuth decodes a base64-encoded auth string and unmarshals it
// into a username/password map. It is a test helper that fatals on failure.
func decodeEncodedAuth(t *testing.T, encoded string) map[string]string {
	t.Helper()

	decoded, decodeErr := base64.URLEncoding.DecodeString(encoded)
	require.NoError(t, decodeErr)

	var authConfig map[string]string

	unmarshalErr := json.Unmarshal(decoded, &authConfig)
	require.NoError(t, unmarshalErr)

	return authConfig
}
