package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/nicholas-fedor/watchtower/pkg/types"
)

func TestAuthMethod(t *testing.T) {
	t.Parallel()

	t.Run("token wins over basic and ssh", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Token:      "tok",
			Username:   "user",
			Password:   "pw",
			SSHKeyPath: filepath.Join(t.TempDir(), "missing"),
			Hosts:      map[string]string{"git.example.com": types.GitHostGitea},
		}}

		got, err := client.authMethod("https://git.example.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, tokenUserGitea, basic.Username)
		assert.Equal(t, "tok", basic.Password)
	})

	t.Run("github token username", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{Token: "ghp_xxx"}}
		got, err := client.authMethod("https://github.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, tokenUserGitHub, basic.Username)
		assert.Equal(t, "ghp_xxx", basic.Password)
	})

	t.Run("basic username and password on a trusted host", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Username: "octocat",
			Password: "pw",
			Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
		}}
		got, err := client.authMethod("https://git.example.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, "octocat", basic.Username)
		assert.Equal(t, "pw", basic.Password)
	})

	t.Run("username only on a trusted host", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Username: "octocat",
			Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
		}}
		got, err := client.authMethod("https://git.example.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, "octocat", basic.Username)
		assert.Empty(t, basic.Password)
	})

	t.Run("password only on a trusted host", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Password: "pw",
			Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
		}}
		got, err := client.authMethod("https://git.example.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Empty(t, basic.Username)
		assert.Equal(t, "pw", basic.Password)
	})

	t.Run("untrusted host is anonymous", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{Token: "tok", Username: "octocat", Password: "pw"}}
		got, err := client.authMethod("https://evil.example/org/app.git")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("plaintext http is anonymous", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Username: "octocat",
			Password: "pw",
			Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
		}}
		got, err := client.authMethod("http://git.example.com/org/app.git")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("anonymous when no credentials", func(t *testing.T) {
		t.Parallel()

		got, err := (&Client{}).authMethod("https://github.com/org/app.git")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("missing ssh key", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{SSHKeyPath: filepath.Join(t.TempDir(), "missing")}}
		_, err := client.authMethod("git@github.com:org/app.git")
		require.Error(t, err)
		assert.ErrorContains(t, err, "load ssh key:")
	})

	t.Run("ssh key without known hosts", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{SSHKeyPath: writeEd25519Key(t)}}
		got, err := client.authMethod("git@github.com:org/app.git")
		require.NoError(t, err)

		keys, ok := got.(*gitSSH.PublicKeys)
		require.True(t, ok)
		assert.Equal(t, tokenUserGit, keys.User)
		assert.Nil(t, keys.HostKeyCallback)
	})

	t.Run("ssh key with known hosts", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			SSHKeyPath:    writeEd25519Key(t),
			SSHKnownHosts: writeKnownHosts(t, "github.com"),
		}}
		got, err := client.authMethod("git@github.com:org/app.git")
		require.NoError(t, err)

		keys, ok := got.(*gitSSH.PublicKeys)
		require.True(t, ok)
		assert.Equal(t, tokenUserGit, keys.User)
		assert.NotNil(t, keys.HostKeyCallback)
	})

	t.Run("ssh origin uses key even when token is set", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Token:      "tok",
			Username:   "user",
			Password:   "pw",
			SSHKeyPath: writeEd25519Key(t),
		}}

		got, err := client.Auth("git@github.com:org/app.git")
		require.NoError(t, err)

		keys, ok := got.(*gitSSH.PublicKeys)
		require.True(t, ok)
		assert.Equal(t, tokenUserGit, keys.User)
	})

	t.Run("ssh url uses key even when token is set", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			Token:      "tok",
			SSHKeyPath: writeEd25519Key(t),
		}}

		got, err := client.Auth("ssh://git@github.com/org/app.git")
		require.NoError(t, err)

		_, ok := got.(*gitSSH.PublicKeys)
		assert.True(t, ok)
	})

	t.Run("ssh origin with token and no key is anonymous", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{Token: "tok", Password: "pw"}}
		got, err := client.Auth("git@github.com:org/app.git")
		require.NoError(t, err)
		assert.Nil(t, got)
		_, isBasic := got.(*gitHTTP.BasicAuth)
		assert.False(t, isBasic)
	})

	t.Run("Auth delegates to authMethod", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{Token: "tok"}}
		got, err := client.Auth("https://github.com/org/app.git")
		require.NoError(t, err)

		basic, ok := got.(*gitHTTP.BasicAuth)
		require.True(t, ok)
		assert.Equal(t, "tok", basic.Password)

		got, err = (*Client)(nil).Auth("https://github.com/org/app.git")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("TLSSettings", func(t *testing.T) {
		t.Parallel()

		bundle := []byte("-----BEGIN CERTIFICATE-----\n")
		client := &Client{opts: Options{CABundle: bundle, InsecureSkipTLS: true}}
		got, insecure := client.TLSSettings()
		assert.Equal(t, bundle, got)
		assert.True(t, insecure)

		got, insecure = (*Client)(nil).TLSSettings()
		assert.Nil(t, got)
		assert.False(t, insecure)
	})

	t.Run("missing known hosts", func(t *testing.T) {
		t.Parallel()

		client := &Client{opts: Options{
			SSHKeyPath:    writeEd25519Key(t),
			SSHKnownHosts: filepath.Join(t.TempDir(), "missing_hosts"),
		}}
		_, err := client.authMethod("git@github.com:org/app.git")
		require.Error(t, err)
		assert.ErrorContains(t, err, "load ssh known_hosts:")
	})
}

func TestBuildAuth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		client   *Client
		repo     string
		username string
		token    string
	}{
		{
			name:     "nil client is anonymous git",
			repo:     "https://github.com/org/app.git",
			username: tokenUserGit,
		},
		{
			name:     "empty client is anonymous git",
			client:   &Client{},
			repo:     "https://github.com/org/app.git",
			username: tokenUserGit,
		},
		{
			name: "token uses host username",
			client: &Client{opts: Options{
				Token:    "tok",
				Username: "ignored",
				Password: "ignored",
			}},
			repo:     "https://github.com/org/app.git",
			username: tokenUserGitHub,
			token:    "tok",
		},
		{
			name: "token on extra gitea host",
			client: &Client{opts: Options{
				Token: "tok",
				Hosts: map[string]string{"git.example.com": types.GitHostGitea},
			}},
			repo:     "https://git.example.com/org/app.git",
			username: tokenUserGitea,
			token:    "tok",
		},
		{
			name: "password with username on a trusted host",
			client: &Client{opts: Options{
				Username: "octocat",
				Password: "pw",
				Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
			}},
			repo:     "https://git.example.com/org/app.git",
			username: "octocat",
			token:    "pw",
		},
		{
			name: "password without username defaults to git",
			client: &Client{opts: Options{
				Password: "pw",
				Hosts:    map[string]string{"git.example.com": types.GitHostGitea},
			}},
			repo:     "https://git.example.com/org/app.git",
			username: tokenUserGit,
			token:    "pw",
		},
		{
			name:     "untrusted host is anonymous",
			client:   &Client{opts: Options{Token: "tok", Password: "pw"}},
			repo:     "https://evil.example/org/app.git",
			username: tokenUserGit,
		},
		{
			name: "plaintext http is anonymous",
			client: &Client{opts: Options{
				Token: "tok",
				Hosts: map[string]string{"git.example.com": types.GitHostGitea},
			}},
			repo:     "http://git.example.com/org/app.git",
			username: tokenUserGit,
		},
		{
			name:     "username without password is anonymous",
			client:   &Client{opts: Options{Username: "octocat"}},
			repo:     "https://github.com/org/app.git",
			username: tokenUserGit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			username, token := tt.client.BuildAuth(tt.repo)
			assert.Equal(t, tt.username, username)
			assert.Equal(t, tt.token, token)
		})
	}
}

func TestTokenUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		repo  string
		extra map[string]string
		want  string
	}{
		{name: "github.com", repo: "https://github.com/org/app.git", want: tokenUserGitHub},
		{name: "gitlab.com", repo: "https://gitlab.com/org/app.git", want: tokenUserGitLab},
		{name: "codeberg.org", repo: "https://codeberg.org/org/app.git", want: tokenUserGitea},
		{
			name:  "extra gitea host",
			repo:  "https://git.example.com/org/app.git",
			extra: map[string]string{"git.example.com": types.GitHostGitea},
			want:  tokenUserGitea,
		},
		{
			name:  "extra gitlab host",
			repo:  "https://gitlab.internal/group/app.git",
			extra: map[string]string{"gitlab.internal": types.GitHostGitLab},
			want:  tokenUserGitLab,
		},
		{
			name:  "extra github host",
			repo:  "https://github.company.com/org/app.git",
			extra: map[string]string{"github.company.com": types.GitHostGitHub},
			want:  tokenUserGitHub,
		},
		{
			name:  "forgejo maps to gitea username",
			repo:  "https://forgejo.example.com/org/app.git",
			extra: map[string]string{"forgejo.example.com": types.GitHostForgejo},
			want:  tokenUserGitea,
		},
		{name: "ssh github url", repo: "git@github.com:org/app.git", want: tokenUserGitHub},
		{name: "unknown host", repo: "https://git.unknown.example/org/app.git", want: tokenUserGit},
		{name: "empty extra map", repo: "https://git.example.com/org/app.git", extra: map[string]string{}, want: tokenUserGit},
		{name: "unparseable repo", repo: "not-a-repo", want: tokenUserGit},
		{name: "empty repo", repo: "", want: tokenUserGit},
		{name: "host only", repo: "https://github.com", want: tokenUserGit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tokenUsername(tt.repo, tt.extra))
		})
	}
}

// writeEd25519Key writes an unencrypted OpenSSH ed25519 private key.
//
// Parameters:
//   - t: Test handle.
//
// Returns:
//   - string: Path to the key file.
func writeEd25519Key(t *testing.T) string {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))

	return path
}

// writeKnownHosts writes a single OpenSSH known_hosts line for host.
//
// Parameters:
//   - t: Test handle.
//   - host: Hostname recorded in the file.
//
// Returns:
//   - string: Path to the known_hosts file.
func writeKnownHosts(t *testing.T, host string) string {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "known_hosts")
	line := host + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + "\n"
	require.NoError(t, os.WriteFile(path, []byte(line), 0o600))

	return path
}
