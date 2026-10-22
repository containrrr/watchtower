/*
Package synology-stop is a Watchtower "stop" lifecycle hook that performs graceful
Docker container shutdown on Synology DSM using the official Web API.

This prevents abrupt SIGKILL by calling SYNO.Docker.Container stop instead of
relying solely on Docker signals.

Environment variables:

	SYNO_URL         – DSM base URL (e.g. http://nas.local:5000)
	SYNO_USER        – DSM username with Docker permissions
	SYNO_PASS        – DSM password
	CLIENT_TIMEOUT     – HTTP timeout in seconds (default 30)
	CLIENT_SSL_VERIFY  – Enable TLS verification (default 1/true)

Build:

	go build -o synology-stop synology-stop.go

Deploy:

	WATCHTOWER_LIFECYCLE_HOOKS=stop:/path/to/synology-stop

References are to "DSM Login Web API Guide" (Last updated Apr 19, 2023):

	[DSM API Guide §2.1] → API Workflow
	[DSM API Guide §2.3] → Making API Requests
	[DSM API Guide §2.4] → Parsing API Response & Common Error Codes
	[DSM API Guide §3.2] → SYNO.API.Auth (login, logout, SynoToken)
*/
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Constants used for constructing DSM Web API requests.
const (
	// apiAuthPath is the fixed authentication endpoint – never changes [DSM API Guide §3.2].
	apiAuthPath = "/webapi/auth.cgi"

	// authAPI is the name of the authentication API.
	authAPI = "SYNO.API.Auth"

	// authVersion is the current stable version of the auth API (DSM 7+) [DSM API Guide §3.2].
	authVersion = 6

	// dockerStopVersion is the version for SYNO.Docker.Container stop.
	dockerStopVersion = 1

	// logoutVersion is the version for SYNO.API.Auth logout [DSM API Guide §3.2].
	logoutVersion = 6

	// authMethodLogin is the method name for login request.
	authMethodLogin = "login"

	// enableSynoToken must be "yes" to receive SynoToken in login response [DSM API Guide §3.2].
	enableSynoToken = "yes"

	// formatSID requests that sid be returned directly in JSON [DSM API Guide §3.2].
	formatSID = "sid"

	// defaultTimeoutStr is the fallback HTTP timeout if CLIENT_TIMEOUT not set.
	defaultTimeoutStr = "30"

	// defaultSSLVerifyStr is the fallback for CLIENT_SSL_VERIFY (1 = verify certificates).
	defaultSSLVerifyStr = "1"
)

var (
	// ErrNoSynoURL → common error 101: missing required parameter [DSM API Guide §2.4 p8].
	ErrNoSynoURL = errors.New("SYNO_URL environment variable is required")

	// ErrNoSynoUser → common error 101: missing required parameter [DSM API Guide §2.4 p8].
	ErrNoSynoUser = errors.New("SYNO_USER environment variable is required")

	// ErrNoSynoPass → common error 101: missing required parameter [DSM API Guide §2.4 p8].
	ErrNoSynoPass = errors.New("SYNO_PASS environment variable is required")

	// ErrNoWTContainer is returned when Watchtower does not provide container info.
	ErrNoWTContainer = errors.New("WT_CONTAINER environment variable is missing")

	// ErrContainerNameEmpty protects against empty or whitespace-only container names.
	ErrContainerNameEmpty = errors.New("container name is empty")

	// ErrAuthenticationFailed covers all login failures: bad credentials, 2FA, account locked, etc.
	// Corresponds to auth error codes 400–410 [DSM API Guide §3.2 p18].
	ErrAuthenticationFailed = errors.New(
		"authentication failed: invalid credentials or server error",
	)

	// ErrMissingSIDOrSynoToken is returned when the login JSON response is malformed
	// or the server did not return expected fields (should never happen on a healthy DSM).
	ErrMissingSIDOrSynoToken = errors.New("failed to extract SID or SynoToken from response")

	// ErrInvalidSynoURL is returned when SYNO_URL cannot be parsed or is missing a scheme.
	ErrInvalidSynoURL = errors.New("SYNO_URL must be a valid URL with scheme (http or https)")
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	SynoURL         string        // DSM base URL (required) [SYNO_URL]
	SynoUser        string        // DSM account (required) [SYNO_USER]
	SynoPass        string        // DSM password (required) [SYNO_PASS]
	ClientTimeout   time.Duration // HTTP client timeout [CLIENT_TIMEOUT]
	ClientSSLVerify bool          // TLS cert verification [CLIENT_SSL_VERIFY]
	IsHTTPS         bool          // true when SynoURL scheme is "https"
}

type WTContainer struct {
	Name string `json:"name"`
}

type SynoAuthResponse struct {
	Success bool `json:"success"`
	Data    struct {
		SID       string `json:"sid"`
		SynoToken string `json:"synotoken"`
	} `json:"data"`
}

type SynoAPIResponse struct {
	Success bool          `json:"success"`
	Error   SynologyError `json:"error,omitzero"`
}

type SynologyError struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

func (e SynologyError) Error() string {
	return fmt.Sprintf("Synology API error %d: %s", e.Code, e.Text)
}

// main is the entry point for the Watchtower stop hook.
// It executes the full graceful-shutdown workflow against Synology DSM [DSM API Guide §2.1].
func main() {
	// Load and validate configuration from environment variables
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Extract the container name that Watchtower is about to stop
	containerName, err := parseContainerName()
	if err != nil {
		log.Fatalf("Failed to parse container name: %v", err)
	}

	log.Printf("Stopping container: %s", containerName)

	// Create HTTP client with configured timeout and optional insecure TLS
	client := newClient(config)

	// Perform login to obtain session ID and CSRF token [DSM API Guide §3.2]
	sid, synoToken, err := authenticate(client, config)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Issue the actual container stop command via entry.cgi [DSM API Guide §2.3]
	err = stopContainer(client, config, containerName, sid, synoToken)
	if err != nil {
		// Best-effort logout even if stop failed
		_ = logout(client, config, sid)

		log.Fatalf("Failed to stop container: %v", err)
	}

	// Clean up the session (non-critical) [DSM API Guide §3.2]
	err = logout(client, config, sid)
	if err != nil {
		log.Printf("Warning: logout failed: %v", err)
	}

	log.Printf("Container %q stopped successfully", containerName)
	os.Exit(0)
}

// loadConfig reads and validates all required/optional environment variables.
// Returns a populated Config or an error (common error 101 if required vars missing [DSM API Guide §2.4 p8]).
func loadConfig() (*Config, error) {
	config := &Config{}

	config.SynoURL = os.Getenv("SYNO_URL")
	// Required by all API calls – abort if missing (common error 101 [DSM API Guide §2.4 p8])
	if config.SynoURL == "" {
		return nil, ErrNoSynoURL
	}

	parsedURL, err := url.Parse(config.SynoURL)
	if err != nil {
		return nil, ErrInvalidSynoURL
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrInvalidSynoURL
	}

	config.IsHTTPS = scheme == "https"

	config.SynoUser = os.Getenv("SYNO_USER")
	// Required for login – abort if missing (common error 101 [DSM API Guide §2.4 p8])
	if config.SynoUser == "" {
		return nil, ErrNoSynoUser
	}

	config.SynoPass = os.Getenv("SYNO_PASS")
	// Required for login – abort if missing (common error 101 [DSM API Guide §2.4 p8])
	if config.SynoPass == "" {
		return nil, ErrNoSynoPass
	}

	// Optional timeout with sensible default
	timeoutStr := os.Getenv("CLIENT_TIMEOUT")
	if timeoutStr == "" {
		timeoutStr = defaultTimeoutStr
	}

	timeout, err := strconv.Atoi(timeoutStr)
	// Invalid numeric value provided
	if err != nil {
		return nil, fmt.Errorf("invalid CLIENT_TIMEOUT: %w", err)
	}

	config.ClientTimeout = time.Duration(timeout) * time.Second

	// Optional SSL verification toggle
	verifyStr := os.Getenv("CLIENT_SSL_VERIFY")
	if verifyStr == "" {
		verifyStr = defaultSSLVerifyStr
	}

	verify, err := strconv.ParseBool(verifyStr)
	// Invalid boolean value
	if err != nil {
		return nil, fmt.Errorf("invalid CLIENT_SSL_VERIFY %q: %w", verifyStr, err)
	}

	config.ClientSSLVerify = verify

	return config, nil
}

// parseContainerName extracts the Docker container name from Watchtower's WT_CONTAINER JSON.
// Watchtower sets WT_CONTAINER={"name":"my-app"} during the stop hook.
func parseContainerName() (string, error) {
	wtEnvVar := os.Getenv("WT_CONTAINER")
	// Watchtower always sets this during stop hooks
	if wtEnvVar == "" {
		return "", ErrNoWTContainer
	}

	var c WTContainer
	// Malformed JSON from Watchtower
	err := json.Unmarshal([]byte(wtEnvVar), &c)
	if err != nil {
		return "", fmt.Errorf("failed to parse WT_CONTAINER JSON: %w", err)
	}

	name := strings.TrimSpace(c.Name)
	// Safety check against empty or whitespace-only names
	if name == "" {
		return "", ErrContainerNameEmpty
	}

	return name, nil
}

// newClient creates an *http.Client with the configured timeout and
// optional insecure TLS transport.
func newClient(config *Config) *http.Client {
	transport := &http.Transport{}
	// User explicitly disabled certificate verification
	if !config.ClientSSLVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,             //nolint:gosec // Controlled by env var
			MinVersion:         tls.VersionTLS12, // Minimum required by DSM 7 [DSM API Guide §2.1 p4]
		}
	}

	return &http.Client{
		Timeout:   config.ClientTimeout,
		Transport: transport,
	}
}

// authenticate performs DSM login via SYNO.API.Auth v6 [DSM API Guide §3.2].
// Returns session ID (sid) and SynoToken required for authenticated requests.
func authenticate(client *http.Client, config *Config) (string, string, error) {
	params := url.Values{}
	params.Set("api", authAPI)
	params.Set("version", strconv.Itoa(authVersion))
	params.Set("method", authMethodLogin)
	params.Set("account", config.SynoUser)
	params.Set("passwd", config.SynoPass)
	params.Set(
		"enable_syno_token",
		enableSynoToken,
	) // Required for CSRF protection [DSM API Guide §3.2]
	params.Set("format", formatSID)

	authURL := config.SynoURL + apiAuthPath + "?" + params.Encode()

	body, err := doHTTPRequest(
		client,
		config,
		authURL,
		http.MethodGet,
		nil,
		"",
		"",
	)
	if err != nil {
		return "", "", err
	}

	var resp SynoAuthResponse
	// Unexpected response format
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse auth response: %w", err)
	}

	// Login rejected by DSM (wrong credentials, 2FA, etc.) – codes 400-410 [DSM API Guide §3.2 p18]
	if !resp.Success {
		return "", "", ErrAuthenticationFailed
	}

	// Defensive check – should never happen on a healthy DSM
	if resp.Data.SID == "" || resp.Data.SynoToken == "" {
		return "", "", ErrMissingSIDOrSynoToken
	}

	return resp.Data.SID, resp.Data.SynoToken, nil
}

// stopContainer sends the POST request to SYNO.Docker.Container stop method [DSM API Guide §2.3].
// Uses form-encoded body, X-Syno-Token header, and sid cookie.
func stopContainer(
	client *http.Client,
	config *Config,
	containerName, sid, synoToken string,
) error {
	stopURL := config.SynoURL + "/webapi/entry.cgi"

	stopParams := url.Values{}
	stopParams.Set("name", containerName)
	stopParams.Set("api", "SYNO.Docker.Container")
	stopParams.Set("method", "stop")
	stopParams.Set("version", strconv.Itoa(dockerStopVersion))
	data := stopParams.Encode()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		stopURL,
		bytes.NewBufferString(data),
	)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %w", err)
	}

	// Headers required by DSM API [DSM API Guide §2.3]
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Syno-Token", synoToken) // CSRF protection [DSM API Guide §3.2]
	req.AddCookie(buildCookie("id", sid, config.IsHTTPS))

	resp, err := client.Do(req)
	// Network or timeout error
	if err != nil {
		return fmt.Errorf("stop container request failed: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	// Failed to read response body
	if err != nil {
		return fmt.Errorf("failed to read stop response: %w", err)
	}

	var apiResp SynoAPIResponse
	// Non-JSON response (e.g., HTML error page)
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return fmt.Errorf("failed to parse stop response: %w", err)
	}

	// Docker API returned an error (container not found, already stopped, etc.)
	if !apiResp.Success {
		return fmt.Errorf("failed to stop container: %w", apiResp.Error)
	}

	return nil
}

// buildCookie creates an http.Cookie with Secure set based on whether the endpoint uses HTTPS.
//
//nolint:gosec // G124: Secure flag set based on parsed SYNO_URL scheme
func buildCookie(name, value string, isHTTPS bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Secure:   isHTTPS,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

// logout terminates the DSM session using SYNO.API.Auth logout (best-effort, non-critical) [DSM API Guide §3.2].
func logout(client *http.Client, config *Config, sid string) error {
	params := url.Values{}
	params.Set("api", authAPI)
	params.Set("version", strconv.Itoa(logoutVersion))
	params.Set("method", "logout")
	params.Set("session", "Docker")
	logoutURL := config.SynoURL + apiAuthPath + "?" + params.Encode()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		logoutURL,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create logout request: %w", err)
	}

	req.AddCookie(buildCookie("id", sid, config.IsHTTPS))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// doHTTPRequest centralises common DSM API request logic
// (sid cookie, SynoToken, Content-Type) [DSM API Guide §2.3].
func doHTTPRequest(
	client *http.Client,
	config *Config,
	urlStr, method string,
	body io.Reader,
	sid, synoToken string,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		method,
		urlStr,
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Attach session cookie if provided
	if sid != "" {
		req.AddCookie(buildCookie("id", sid, config.IsHTTPS))
	}
	// Attach CSRF token if provided
	if synoToken != "" {
		req.Header.Set("X-Syno-Token", synoToken)
	}
	// Form-encoded body is the standard for entry.cgi requests
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, nil
}
