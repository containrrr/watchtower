package apply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteContext(t *testing.T) {
	t.Parallel()

	got, err := RemoteContext("https://github.com/org/app.git", "abc123", "")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/app.git#abc123", got)

	got, err = RemoteContext("https://github.com/org/app.git", "abc123", ".")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/app.git#abc123", got)

	got, err = RemoteContext("https://github.com/org/app.git", "abc123", "apps/api")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/app.git#abc123:apps/api", got)

	got, err = RemoteContext("git@github.com:org/app.git", "abc123", "")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/app.git#abc123", got)

	_, err = RemoteContext("https://github.com/org/app.git", "abc123", "../etc")
	require.ErrorIs(t, err, errPathEscape)

	_, err = RemoteContext("https://github.com/org/app.git", "", "")
	require.ErrorIs(t, err, errCommitEmpty)
}

func TestRemoteContextRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := RemoteContext("", "abc123", "")
	require.ErrorIs(t, err, errRemoteEmpty)

	_, err = RemoteContext("https://github.com/org/app.git", "abc#123", "")
	require.ErrorIs(t, err, errCommitInvalid)

	_, err = RemoteContext("https://github.com/org/app.git", "abc123", "/abs")
	require.ErrorIs(t, err, errPathEscape)

	_, err = RemoteContext("https://github.com/org/app.git", "abc123", `\\`)
	require.ErrorIs(t, err, errPathEscape)

	_, err = RemoteContext("https://github.com/org/app.git", "abc123", "foo:bar")
	require.ErrorIs(t, err, errPathEscape)

	_, err = RemoteContext("file:///tmp/app.git", "abc123", "")
	require.ErrorIs(t, err, errRemoteInvalid)

	_, err = RemoteContext("https://github.com/org/app.git", "not-a-sha", "")
	require.ErrorIs(t, err, errCommitInvalid)

	_, err = RemoteContext("https://github.com/org/app.git", "abc", "")
	require.ErrorIs(t, err, errCommitInvalid)

	got, err := RemoteContext("http://git.example.com/org/app", "abc123", "")
	require.NoError(t, err)
	assert.Equal(t, "http://git.example.com/org/app.git#abc123", got)

	got, err = RemoteContext("https://github.com:8443/org/app.git", "abc123", "")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com:8443/org/app.git#abc123", got)
}

// TestRemoteContextInvalidRemoteDoesNotExposeCredentials verifies that invalid remote errors omit repository credentials.
func TestRemoteContextInvalidRemoteDoesNotExposeCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		remote  string
		secrets []string
	}{
		{
			name:    "malformed HTTPS",
			remote:  "https://audit-user:https-password@git.example.com/%zz?token=https-query-secret",
			secrets: []string{"audit-user", "https-password", "https-query-secret"},
		},
		{
			name:    "HTTPS without host",
			remote:  "https://audit-user:https-password@/org/app.git?token=https-query-secret",
			secrets: []string{"audit-user", "https-password", "https-query-secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := RemoteContext(tt.remote, "abc123", "")
			require.ErrorIs(t, err, errRemoteInvalid)

			for _, secret := range tt.secrets {
				assert.NotContains(t, err.Error(), secret)
			}
		})
	}
}

func TestWithAuth(t *testing.T) {
	t.Parallel()

	got, err := WithAuth("https://github.com/org/app.git#abc", "x-access-token", "secret")
	require.NoError(t, err)
	assert.Equal(t, "https://x-access-token:secret@github.com/org/app.git#abc", got)

	got, err = WithAuth("https://github.com/org/app.git#abc", "git", "")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/app.git#abc", got)

	got, err = WithAuth("https://github.com/org/app.git#abc", "", "secret")
	require.NoError(t, err)
	assert.Equal(t, "https://git:secret@github.com/org/app.git#abc", got)

	_, err = WithAuth("http://git.example.com/org/app.git#abc", "git", "secret")
	require.ErrorIs(t, err, errNeedsHTTPS)

	_, err = WithAuth("ssh://git@github.com/org/app.git", "git", "secret")
	require.ErrorIs(t, err, errNeedsHTTPS)

	_, err = WithAuth("://", "git", "secret")
	require.Error(t, err)

	_, err = WithAuth("https:github.com/org/app.git#abc", "git", "secret")
	require.ErrorIs(t, err, errMalformedRemote)

	_, err = WithAuth("https:///org/app.git#abc", "git", "secret")
	require.ErrorIs(t, err, errMalformedRemote)

	_, err = WithAuth("https://:443/org/app.git", "git", "secret")
	require.ErrorIs(t, err, errMalformedRemote)

	got, err = WithAuth("https://git.example.com:8443/org/app.git#abc", "git", "secret")
	require.NoError(t, err)
	assert.Equal(t, "https://git:secret@git.example.com:8443/org/app.git#abc", got)

	got, err = WithAuth("https://[::1]/org/app.git#abc", "git", "secret")
	require.NoError(t, err)
	assert.Equal(t, "https://git:secret@[::1]/org/app.git#abc", got)
}

// TestWithAuthErrorsDoNotExposeRemoteCredentials verifies that authentication errors omit remote credentials for unsupported or malformed URLs.
func TestWithAuthErrorsDoNotExposeRemoteCredentials(t *testing.T) {
	t.Parallel()

	t.Run("malformed HTTPS", func(t *testing.T) {
		t.Parallel()

		remote := "https://audit-user:https-password@git.example.com/%zz?token=https-query-secret"
		_, err := WithAuth(remote, "git", "secret")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "audit-user")
		assert.NotContains(t, err.Error(), "https-password")
		assert.NotContains(t, err.Error(), "https-query-secret")
	})

	t.Run("SCP", func(t *testing.T) {
		t.Parallel()

		remote := "audit-user:scp-password@git.example.com:org/app.git?token=scp-query-secret"
		_, err := WithAuth(remote, "git", "secret")
		require.ErrorIs(t, err, errNeedsHTTPS)
		assert.NotContains(t, err.Error(), "audit-user")
		assert.NotContains(t, err.Error(), "scp-password")
		assert.NotContains(t, err.Error(), "scp-query-secret")
	})
}

func TestSafeRelPath(t *testing.T) {
	t.Parallel()

	got, err := SafeRelPath("build/docker/Dockerfile")
	require.NoError(t, err)
	assert.Equal(t, "build/docker/Dockerfile", got)

	got, err = SafeRelPath("")
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = SafeRelPath("../Dockerfile")
	require.ErrorIs(t, err, errPathEscape)

	_, err = SafeRelPath("/etc/passwd")
	require.ErrorIs(t, err, errPathEscape)

	_, err = SafeRelPath(`\\`)
	require.ErrorIs(t, err, errPathEscape)
}

func TestValidCommit(t *testing.T) {
	t.Parallel()

	assert.True(t, validCommit("abc123"))
	assert.True(t, validCommit("deadbeef"))
	assert.True(t, validCommit("ABCDEF012345"))
	assert.False(t, validCommit("abc"))
	assert.False(t, validCommit("not-hex!"))
	assert.False(t, validCommit("../etc"))
	assert.False(t, validCommit(""))
}

func TestHTTPSRemoteAndHostHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "[2001:db8::1]", httpHost("2001:db8::1"))
	assert.Equal(t, "git.example.com", httpHost("git.example.com"))

	assert.False(t, keepPort("https", 0))
	assert.False(t, keepPort("https", httpsPort))
	assert.True(t, keepPort("https", 8443))
	assert.False(t, keepPort("http", httpPort))
	assert.True(t, keepPort("http", 8080))
	assert.False(t, keepPort("ssh", sshPort))
	assert.False(t, keepPort("ssh", 2222))
	assert.False(t, keepPort("git", 9418))
}
