package git

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/transport"

	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

const (
	tokenUserGitHub = "x-access-token"
	tokenUserGitLab = "oauth2"
	tokenUserGitea  = "gitea"
	tokenUserGit    = "git"
)

// Auth returns the process-wide go-git auth method for repo.
//
// Parameters:
//   - repo: Clone or origin URL.
//
// Returns:
//   - transport.AuthMethod: Auth for go-git, or nil for anonymous access.
//   - error: Non-nil when an SSH key or known_hosts file cannot be loaded.
func (c *Client) Auth(repo string) (transport.AuthMethod, error) {
	if c == nil {
		//nolint:nilnil // Anonymous Git access is a nil AuthMethod, not an error.
		return nil, nil
	}

	return c.authMethod(repo)
}

// TLSSettings returns the process-wide Git HTTPS TLS settings.
//
// Returns:
//   - []byte: PEM CA bundle, or nil.
//   - bool: True when TLS verification is skipped.
func (c *Client) TLSSettings() ([]byte, bool) {
	if c == nil {
		return nil, false
	}

	return c.opts.CABundle, c.opts.InsecureSkipTLS
}

// BuildAuth returns HTTPS credentials the Docker builder can embed in a Git context URL.
//
// Credentials are returned only for a trusted host that is not plaintext HTTP.
// A container label cannot redirect the process token or password to another host.
//
// Parameters:
//   - repo: Associated clone URL.
//
// Returns:
//   - username: Basic user for the token or configured username.
//   - token: Token or password. Empty when the builder should clone anonymously.
func (c *Client) BuildAuth(repo string) (string, string) {
	if c == nil || !c.builderCredentialAllowed(repo) {
		return tokenUserGit, ""
	}

	if c.opts.Token != "" {
		return tokenUsername(repo, c.opts.Hosts), c.opts.Token
	}

	if c.opts.Password != "" {
		username := c.opts.Username
		if username == "" {
			username = tokenUserGit
		}

		return username, c.opts.Password
	}

	return tokenUserGit, ""
}

// authMethod returns the auth method that matches the remote URL scheme.
//
// HTTPS uses token, then username/password, then none, and only for a
// trusted host. Plaintext HTTP is anonymous. SSH and scp-like remotes use
// the SSH key only for a trusted host, then none. A token is never returned
// as HTTP BasicAuth for an SSH origin.
//
// Parameters:
//   - repo: Clone or origin URL used to pick the scheme and token username.
//
// Returns:
//   - transport.AuthMethod: Auth for go-git, or nil for anonymous access.
//   - error: Non-nil when an SSH key or known_hosts file cannot be loaded.
func (c *Client) authMethod(repo string) (transport.AuthMethod, error) {
	if !c.credentialHostTrusted(repo) {
		//nolint:nilnil // Anonymous Git access is a nil AuthMethod, not an error.
		return nil, nil
	}

	if sshRemote(repo) {
		return c.sshAuth()
	}

	if !httpsRemote(repo) {
		//nolint:nilnil // Plaintext HTTP is not given process credentials.
		return nil, nil
	}

	if c.opts.Token != "" {
		return &gitHTTP.BasicAuth{
			Username: tokenUsername(repo, c.opts.Hosts),
			Password: c.opts.Token,
		}, nil
	}

	if c.opts.Username != "" || c.opts.Password != "" {
		return &gitHTTP.BasicAuth{
			Username: c.opts.Username,
			Password: c.opts.Password,
		}, nil
	}

	//nolint:nilnil // Anonymous Git access is a nil AuthMethod, not an error.
	return nil, nil
}

// sshAuth loads the configured SSH key, or returns nil when none is set.
//
// Returns:
//   - transport.AuthMethod: PublicKeys auth, or nil when no key path is set.
//   - error: Non-nil when the key or known_hosts file cannot be loaded.
func (c *Client) sshAuth() (transport.AuthMethod, error) {
	if c.opts.SSHKeyPath == "" {
		//nolint:nilnil // Anonymous Git access is a nil AuthMethod, not an error.
		return nil, nil
	}

	publicKeys, err := gitSSH.NewPublicKeysFromFile(tokenUserGit, c.opts.SSHKeyPath, "")
	if err != nil {
		return nil, fmt.Errorf("load ssh key: %w", err)
	}

	if c.opts.SSHKnownHosts != "" {
		callback, err := gitSSH.NewKnownHostsCallback(c.opts.SSHKnownHosts)
		if err != nil {
			return nil, fmt.Errorf("load ssh known_hosts: %w", err)
		}

		publicKeys.HostKeyCallback = callback
	}

	return publicKeys, nil
}

// builderCredentialAllowed reports whether BuildAuth may embed process credentials.
//
// SSH clone URLs are allowed because the builder context is rewritten to HTTPS.
// Plaintext HTTP is not.
//
// Parameters:
//   - repo: Associated clone URL.
//
// Returns:
//   - bool: True when the host is trusted and the URL is not plaintext HTTP.
func (c *Client) builderCredentialAllowed(repo string) bool {
	if c == nil || !c.credentialHostTrusted(repo) || plaintextHTTP(repo) {
		return false
	}

	return true
}

// credentialHostTrusted reports whether repo's host may receive process credentials.
//
// Built-in provider hosts and operator host mappings are trusted. A container
// label cannot add a host.
//
// Parameters:
//   - repo: Clone or origin URL.
//
// Returns:
//   - bool: True when the hostname is a known or configured provider host.
func (c *Client) credentialHostTrusted(repo string) bool {
	if c == nil {
		return false
	}

	host, _, _, ok := splitRepo(repo)
	if !ok || host == "" {
		return false
	}

	return types.ResolveGitHostKind(host, c.opts.Hosts) != ""
}

// httpsRemote reports whether repo is an HTTPS Git URL.
//
// Parameters:
//   - repo: Clone or origin URL.
//
// Returns:
//   - bool: True for https:// remotes.
func httpsRemote(repo string) bool {
	endpoint, err := transport.NewEndpoint(repo)

	return err == nil && endpoint.Protocol == "https"
}

// plaintextHTTP reports whether repo is a plaintext HTTP Git URL.
//
// Parameters:
//   - repo: Clone or origin URL.
//
// Returns:
//   - bool: True for http:// remotes.
func plaintextHTTP(repo string) bool {
	endpoint, err := transport.NewEndpoint(repo)

	return err == nil && endpoint.Protocol == "http"
}

// sshRemote reports whether repo is an SSH or scp-like Git URL.
//
// Parameters:
//   - repo: Clone or origin URL.
//
// Returns:
//   - bool: True for ssh:// and git@host:path remotes.
func sshRemote(repo string) bool {
	endpoint, err := transport.NewEndpoint(repo)
	if err != nil {
		return false
	}

	return endpoint.Protocol == "ssh"
}

// tokenUsername returns the Basic-auth user for a hosted Git token.
//
// Parameters:
//   - repo: Clone URL.
//   - extra: Extra hostname to product mappings.
//
// Returns:
//   - string: Product-specific username, or git when the host is unknown.
func tokenUsername(repo string, extra map[string]string) string {
	host, _, _, ok := splitRepo(repo)
	if !ok {
		return tokenUserGit
	}

	switch types.ResolveGitHostKind(host, extra) {
	case types.GitHostGitHub:
		return tokenUserGitHub
	case types.GitHostGitLab:
		return tokenUserGitLab
	case types.GitHostGitea:
		return tokenUserGitea
	default:
		return tokenUserGit
	}
}
