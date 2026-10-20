// Package apply implements Git apply backends after monitoring finds a stale ref.
package apply

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	httpPort  = 80
	httpsPort = 443
	sshPort   = 22
)

var (
	// errRemoteEmpty indicates no clone URL was associated.
	errRemoteEmpty = errors.New("git remote is empty")
	// errRemoteInvalid indicates the clone URL cannot be turned into an HTTPS Git context.
	errRemoteInvalid = errors.New("invalid git remote")
	// errCommitEmpty indicates monitoring produced no commit SHA.
	errCommitEmpty = errors.New("git commit is empty")
	// errCommitInvalid indicates the commit cannot be used in a Docker Git fragment.
	errCommitInvalid = errors.New("invalid git commit")
	// errNeedsHTTPS indicates a token was given for a non-HTTP Git context.
	errNeedsHTTPS = errors.New("git build token requires an HTTPS remote")
	// errPathEscape indicates a context subdirectory left the repository.
	errPathEscape = errors.New("git build path escapes checkout")

	errMalformedRemote = errors.New("parse git remote: malformed URL")
)

// RemoteContext builds a Docker Git build-context URL.
//
// Format: {httpsURL}#{commit}[:{subdir}].
//
// Parameters:
//   - repo: Associated clone URL.
//   - commit: Commit SHA from Git monitoring.
//   - contextRel: Optional subdirectory of the repo used as the build context.
//
// Returns:
//   - string: Docker Git URL context.
//   - error: Non-nil when the repo, commit, or context is invalid.
func RemoteContext(repo, commit, contextRel string) (string, error) {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return "", errCommitEmpty
	}

	// Docker's fragment is #ref:dir. Only hex object IDs are accepted so a
	// hostile ref advertisement cannot inject path or tag characters.
	if !validCommit(commit) {
		return "", fmt.Errorf("%w: %s", errCommitInvalid, commit)
	}

	httpsURL, err := httpsRemote(repo)
	if err != nil {
		return "", err
	}

	subdir, err := SafeRelPath(contextRel)
	if err != nil {
		return "", err
	}

	if subdir == "" {
		return httpsURL + "#" + commit, nil
	}

	return httpsURL + "#" + commit + ":" + subdir, nil
}

// WithAuth embeds a token in an HTTPS Git context URL for the builder.
//
// Parameters:
//   - remote: Git context URL from RemoteContext.
//   - username: Basic-auth user (for example x-access-token).
//   - token: Token or password. Empty leaves remote unchanged.
//
// Returns:
//   - string: URL with userinfo, or the original remote.
//   - error: Non-nil when remote is not HTTPS, has no host, or cannot be parsed.
func WithAuth(remote, username, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return remote, nil
	}

	parsed, err := url.Parse(remote)
	if err != nil {
		return "", errMalformedRemote
	}

	if parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: remote scheme must be https", errNeedsHTTPS)
	}

	// An opaque URL such as https:host has no host, and its string form drops
	// userinfo, so the token would be lost. A port alone names no server.
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("%w: missing host", errMalformedRemote)
	}

	if username == "" {
		username = "git"
	}

	parsed.User = url.UserPassword(username, token)

	return parsed.String(), nil
}

// httpsRemote converts a clone URL to an HTTPS (or HTTP) remote without userinfo.
//
// Parameters:
//   - repo: HTTPS, SSH, or scp-like Git URL.
//
// Returns:
//   - string: Scheme, host, and path suitable for Docker's remote context.
//   - error: Non-nil when the URL has no host or path.
func httpsRemote(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", errRemoteEmpty
	}

	endpoint, err := transport.NewEndpoint(repo)
	if err != nil {
		return "", fmt.Errorf("%w: endpoint could not be parsed", errRemoteInvalid)
	}

	host := strings.Trim(endpoint.Host, "[]")
	if host == "" {
		return "", fmt.Errorf("%w: missing host", errRemoteInvalid)
	}

	repoPath := strings.TrimPrefix(endpoint.Path, "/")
	if repoPath == "" {
		return "", fmt.Errorf("%w: missing repository path", errRemoteInvalid)
	}

	if !strings.HasSuffix(repoPath, ".git") {
		repoPath += ".git"
	}

	switch endpoint.Protocol {
	case "http", "https", "ssh", "git":
	default:
		return "", fmt.Errorf("%w: unsupported protocol", errRemoteInvalid)
	}

	// Preserve http only when the operator used it. Everything else becomes https.
	scheme := "https"
	if endpoint.Protocol == "http" {
		scheme = "http"
	}

	parsed := &url.URL{
		Scheme: scheme,
		Host:   httpHost(host),
		Path:   "/" + repoPath,
	}

	if keepPort(endpoint.Protocol, endpoint.Port) {
		parsed.Host = net.JoinHostPort(host, strconv.Itoa(endpoint.Port))
	}

	return parsed.String(), nil
}

// SafeRelPath confines a Dockerfile or context path to a relative repo path.
//
// Parameters:
//   - rel: Requested relative path from a label, flag, or Docker context subdir.
//
// Returns:
//   - string: Clean relative path, or empty for the repository root.
//   - error: Non-nil when the path would escape the repository.
func SafeRelPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return "", nil
	}

	if path.IsAbs(rel) || strings.Contains(rel, ":") {
		return "", fmt.Errorf("%w: %s", errPathEscape, rel)
	}

	cleaned := path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if path.IsAbs(cleaned) || !filepath.IsLocal(filepath.FromSlash(cleaned)) {
		return "", fmt.Errorf("%w: %s", errPathEscape, rel)
	}

	return cleaned, nil
}

// validCommit reports whether commit is a hex Git object ID.
//
// Parameters:
//   - commit: Candidate commit from monitoring.
//
// Returns:
//   - bool: True when commit is 6–64 hex characters.
func validCommit(commit string) bool {
	n := len(commit)
	if n < 6 || n > 64 {
		return false
	}

	for i := range n {
		c := commit[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}

		return false
	}

	return true
}

// keepPort reports whether port should appear on an HTTP or HTTPS Git context host.
//
// SSH and git ports are not copied onto the HTTPS URL. A non-default SSH port
// is not an HTTPS port.
//
// Parameters:
//   - protocol: Original transport protocol from go-git.
//   - port: Endpoint port, or 0 when unset.
//
// Returns:
//   - bool: True when the port is a non-default HTTP or HTTPS port.
func keepPort(protocol string, port int) bool {
	if port == 0 {
		return false
	}

	switch protocol {
	case "http":
		return port != httpPort
	case "https":
		return port != httpsPort
	default:
		return false
	}
}

// httpHost returns hostname, bracketed when it is IPv6.
//
// Parameters:
//   - hostname: Host without port.
//
// Returns:
//   - string: Host suitable for url.URL.Host without a port.
func httpHost(hostname string) string {
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]"
	}

	return hostname
}
