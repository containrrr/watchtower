package git

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

// applyAuth sets Bearer or basic credentials on req.
//
// Credentials are sent only for HTTPS, and only when the request hostname
// is the clone host or GitHub's api.github.com for a github.com clone.
// A container git-host label cannot redirect the process token.
//
// Parameters:
//   - req: Outgoing HTTP request. Ignored when nil.
//   - cloneHost: Hostname from the clone URL. Empty sends no credentials.
//
// Returns:
//   - none.
func (c *Client) applyAuth(req *http.Request, cloneHost string) {
	if c == nil || req == nil || req.URL == nil || req.URL.Scheme != "https" {
		return
	}

	if !credentialHostAllowed(req.URL.Hostname(), cloneHost) {
		return
	}

	if c.opts.Token != "" {
		scheme := "Bearer"

		kind := types.ResolveGitHostKind(cloneHost, c.opts.Hosts)
		if kind == "" && isGiteaAPIPath(req.URL.Path) {
			kind = types.GitHostGitea
		}

		if kind == types.GitHostGitea {
			scheme = "token"
		}

		req.Header.Set("Authorization", scheme+" "+c.opts.Token)

		return
	}

	if c.opts.Username != "" || c.opts.Password != "" {
		req.SetBasicAuth(c.opts.Username, c.opts.Password)
	}
}

// isGiteaAPIPath reports whether path contains the Gitea API v1 prefix.
//
// Parameters:
//   - path: HTTP request path.
//
// Returns:
//   - bool: True when the path contains the Gitea API v1 prefix.
func isGiteaAPIPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "api" && parts[i+1] == "v1" {
			return true
		}
	}

	return false
}

// sanitizeRepository removes credentials and repository details from raw.
//
// Parameters:
//   - raw: Repository URL, SCP-style remote, or local path.
//
// Returns:
//   - string: Safe repository identifier, or empty when raw cannot be sanitized.
func sanitizeRepository(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "git@") {
		return sanitizeSCPRepository(raw)
	}

	if !strings.Contains(raw, "://") {
		if strings.Contains(raw, "@") && strings.Contains(raw, ":") {
			return ""
		}

		return stripRepositoryDetails(raw)
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || (parsed.Host == "" && parsed.Scheme != "file") {
		return ""
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""

	return parsed.String()
}

// sanitizeSCPRepository removes details from an SCP-style remote.
//
// Parameters:
//   - raw: SCP-style repository remote.
//
// Returns:
//   - string: Sanitized remote, or empty when its host or path is invalid.
func sanitizeSCPRepository(raw string) string {
	raw = stripRepositoryDetails(raw)

	_, remote, found := strings.Cut(raw, "@")
	if !found {
		return ""
	}

	separator := strings.LastIndexByte(remote, ':')
	if separator <= 0 || separator == len(remote)-1 {
		return ""
	}

	host := remote[:separator]
	path := remote[separator+1:]

	parsedHost, err := url.Parse("//" + host)
	if err != nil || parsedHost.Host == "" || parsedHost.User != nil || parsedHost.Path != "" {
		return ""
	}

	return "git@" + host + ":" + path
}

// stripRepositoryDetails removes query and fragment details from raw.
//
// Parameters:
//   - raw: Repository URL, SCP-style remote, or local path.
//
// Returns:
//   - string: Value without query or fragment details.
func stripRepositoryDetails(raw string) string {
	raw, _, _ = strings.Cut(raw, "#")
	raw, _, _ = strings.Cut(raw, "?")

	return raw
}

// sanitizeRepositoryMessage replaces repository details and secrets in message.
//
// Parameters:
//   - raw: Repository value whose details must be removed.
//   - message: Error message to sanitize.
//
// Returns:
//   - string: Message with repository secrets redacted.
func sanitizeRepositoryMessage(raw, message string) string {
	replacement := sanitizeRepository(raw)
	if replacement == "" {
		return "remote operation failed"
	}

	const repositoryPlaceholder = "\x00watchtower-git-repository\x00"

	message = strings.ReplaceAll(message, raw, repositoryPlaceholder)
	for _, secret := range repositorySecrets(raw) {
		message = strings.ReplaceAll(message, secret, "redacted")
	}

	return strings.ReplaceAll(message, repositoryPlaceholder, replacement)
}

// repositorySecrets extracts credential-like values from raw repository details.
//
// Parameters:
//   - raw: Repository value to inspect for sensitive details.
//
// Returns:
//   - []string: Non-empty values that may be sensitive in a remote error.
func repositorySecrets(raw string) []string {
	var secrets []string

	_, rawQuery, hasQuery := strings.Cut(raw, "?")
	if hasQuery {
		rawQuery, _, _ = strings.Cut(rawQuery, "#")

		values, err := url.ParseQuery(rawQuery)
		if err == nil {
			for _, params := range values {
				for _, secret := range params {
					if secret != "" {
						secrets = append(secrets, secret)
					}
				}
			}
		}
	}

	_, rawFragment, hasFragment := strings.Cut(raw, "#")
	if hasFragment && rawFragment != "" {
		secrets = append(secrets, rawFragment)

		unescaped, err := url.QueryUnescape(rawFragment)
		if err == nil && unescaped != "" {
			secrets = append(secrets, unescaped)
		}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return secrets
	}

	if parsed.User != nil {
		password, hasPassword := parsed.User.Password()
		if hasPassword && password != "" {
			if user := parsed.User.String(); user != "" {
				secrets = append(secrets, user)
			}

			if username := parsed.User.Username(); username != "" {
				secrets = append(secrets, username)
			}

			secrets = append(secrets, password)
		}
	}

	return secrets
}

// credentialHostAllowed reports whether a process credential may be sent to requestHost.
//
// Parameters:
//   - requestHost: Hostname of the outgoing request, port ignored.
//   - cloneHost: Hostname from the clone URL, port ignored.
//
// Returns:
//   - bool: True when the hosts match, or the request is api.github.com for github.com.
func credentialHostAllowed(requestHost, cloneHost string) bool {
	requestHost = toHostname(requestHost)
	cloneHost = toHostname(cloneHost)

	if requestHost == "" || cloneHost == "" {
		return false
	}

	if requestHost == cloneHost {
		return true
	}

	return cloneHost == "github.com" && requestHost == "api.github.com"
}

// originOf returns the classified HTTP origin for host.
//
// Parameters:
//   - host: Hostname, optionally with brackets.
//
// Returns:
//   - url.URL: Stored origin, or https://host.
func (c *Client) originOf(host string) url.URL {
	host = toHostname(host)
	if c != nil && c.origins != nil {
		if origin, ok := c.origins[host]; ok {
			return origin
		}
	}

	return url.URL{Scheme: "https", Host: httpHost(host)}
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

// githubAPIAt builds a GitHub REST URL from an explicit API origin.
//
// Parameters:
//   - origin: HTTP API origin from a git-host label, or originOf(host).
//   - host: Clone hostname. github.com always uses api.github.com.
//   - segments: Path segments after the API root.
//
// Returns:
//   - *url.URL: Absolute API URL.
func (c *Client) githubAPIAt(origin url.URL, host string, segments ...string) *url.URL {
	if strings.EqualFold(toHostname(host), "github.com") &&
		(origin.Host == "" || strings.EqualFold(origin.Hostname(), "github.com")) {
		return (&url.URL{Scheme: "https", Host: "api.github.com"}).JoinPath(segments...)
	}

	if origin.Host == "" {
		origin = c.originOf(host)
	}

	return origin.JoinPath(append([]string{"api", "v3"}, segments...)...)
}

// gitlabAPIAt builds a GitLab REST URL from an explicit API origin.
//
// Parameters:
//   - origin: HTTP API origin from a git-host label, or originOf(host).
//   - host: Clone hostname, used when origin is empty.
//   - segments: Path segments after the API root.
//
// Returns:
//   - *url.URL: Absolute API URL.
func (c *Client) gitlabAPIAt(origin url.URL, host string, segments ...string) *url.URL {
	if origin.Host == "" {
		origin = c.originOf(host)
	}

	return origin.JoinPath(append([]string{"api", "v4"}, segments...)...)
}

// giteaAPIAt builds a Gitea or Forgejo REST URL from an explicit API origin.
//
// Parameters:
//   - origin: HTTP API origin from a git-host label, or originOf(host).
//   - host: Clone hostname, used when origin is empty.
//   - segments: Path segments after the API root.
//
// Returns:
//   - *url.URL: Absolute API URL.
func (c *Client) giteaAPIAt(origin url.URL, host string, segments ...string) *url.URL {
	if origin.Host == "" {
		origin = c.originOf(host)
	}

	return origin.JoinPath(append([]string{"api", "v1"}, segments...)...)
}

// newGitHTTPClient builds an HTTP client for REST probes and API ref checks.
//
// Parameters:
//   - timeout: Client timeout.
//   - insecure: When true, skip TLS verification.
//   - caPEM: Optional PEM CA bundle.
//
// Returns:
//   - *http.Client: Client with optional custom TLS.
func newGitHTTPClient(timeout time.Duration, insecure bool, caPEM []byte) *http.Client {
	client := &http.Client{
		Timeout:       timeout,
		CheckRedirect: rejectCrossHostRedirect,
	}
	if !insecure && len(caPEM) == 0 {
		return client
	}

	base, ok := http.DefaultTransport.(*http.Transport)

	var transport *http.Transport
	if ok {
		transport = base.Clone()
	} else {
		transport = &http.Transport{}
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if insecure {
		// Operator opt-in via git-insecure-skip-tls for private CAs that cannot be bundled.
		cfg.InsecureSkipVerify = true
	}

	if len(caPEM) > 0 {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}

		if pool.AppendCertsFromPEM(caPEM) {
			cfg.RootCAs = pool
		}
	}

	transport.TLSClientConfig = cfg
	client.Transport = transport

	return client
}

// toHostname lowercases host and strips IPv6 brackets.
//
// Parameters:
//   - host: Hostname or [IPv6] literal.
//
// Returns:
//   - string: Lowercase hostname.
func toHostname(host string) string {
	return strings.ToLower(strings.Trim(host, "[]"))
}

// joinEscaped appends path segments onto base, escaping each segment.
//
// Use this when a segment may contain slashes, such as GitLab project IDs.
//
// Parameters:
//   - base: API origin.
//   - segments: Path segments to escape and join.
//
// Returns:
//   - *url.URL: Joined URL. Empty when base is nil.
func joinEscaped(base *url.URL, segments ...string) *url.URL {
	if base == nil {
		return &url.URL{}
	}

	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}

	raw := strings.TrimSuffix(base.String(), "/") + "/" + strings.Join(escaped, "/")

	parsed, err := url.Parse(raw)
	if err != nil {
		return base.JoinPath(segments...)
	}

	return parsed
}

// rejectCrossHostRedirect blocks redirects that change scheme or host.
//
// Git API calls carry Authorization. Following a cross-origin Location would
// forward that token to an attacker-controlled host.
//
// Parameters:
//   - req: The request that would be issued for the redirect.
//   - via: Prior requests in the redirect chain.
//
// Returns:
//   - error: Non-nil when the redirect leaves the original origin.
func rejectCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}

	prev := via[len(via)-1]
	if req.URL == nil || prev.URL == nil {
		return errCrossHostRedirect
	}

	if !strings.EqualFold(req.URL.Scheme, prev.URL.Scheme) || !strings.EqualFold(req.URL.Host, prev.URL.Host) {
		return errCrossHostRedirect
	}

	return nil
}
