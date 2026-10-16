package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		raw             string
		want            string
		wantErrContains string
		secrets         []string
	}{
		{name: "https", raw: "https://git.example.com:3000", want: "https://git.example.com:3000"},
		{name: "trailing slash", raw: "https://git.example.com/", want: "https://git.example.com"},
		{name: "hostname only", raw: "gitlab.internal", want: "https://gitlab.internal"},
		{name: "path prefix", raw: "https://git.example.com/gitlab", want: "https://git.example.com/gitlab"},
		{name: "http scheme", raw: "http://git.example.com", want: "http://git.example.com"},
		{name: "ipv6", raw: "https://[2001:db8::1]", want: "https://[2001:db8::1]"},
		{name: "strips userinfo", raw: "https://user:pass@git.example.com", want: "https://git.example.com"},
		{name: "empty", wantErrContains: "empty value"},
		{name: "ssh scheme", raw: "ssh://git.example.com", wantErrContains: "scheme must be http or https"},
		{name: "host equals type", raw: "git.example.com=gitea", wantErrContains: "host=type mappings are not supported"},
		{
			name:            "malformed credential URL",
			raw:             "https://audit-user:host-password@git.example.com/%zz?host-query-secret",
			wantErrContains: "malformed URL",
			secrets:         []string{"audit-user", "host-password", "host-query-secret"},
		},
		{
			name:            "missing credential URL host",
			raw:             "https://audit-user:host-password@/git.example.com?host-query-secret",
			wantErrContains: "missing hostname",
			secrets:         []string{"audit-user", "host-password", "host-query-secret"},
		},
		{
			name:            "credential URL mapping",
			raw:             "https://audit-user:host-password@git.example.com?token=host-query-secret",
			wantErrContains: "host=type mappings are not supported",
			secrets:         []string{"audit-user", "host-password", "host-query-secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseAPIOrigin(tt.raw)
			if tt.wantErrContains != "" {
				require.ErrorIs(t, err, errInvalidHost)
				assert.ErrorContains(t, err, tt.wantErrContains)

				for _, secret := range tt.secrets {
					assert.NotContains(t, err.Error(), secret)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
