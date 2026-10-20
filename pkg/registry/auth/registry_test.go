package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_parseChallenge verifies that a bearer challenge's realm, service, and
// scope are read case-insensitively by key, unquoted, with commas inside quotes
// kept, and that a header that is not a bearer challenge yields nothing.
func Test_parseChallenge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   challengeValues
	}{
		{
			name:   "valid challenge with all fields",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/repo:pull"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: "repository:user/repo:pull"},
		},
		{
			name:   "valid challenge with realm and service only",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: ""},
		},
		{
			name:   "uppercase bearer prefix",
			header: `BEARER realm="https://ghcr.io/token",service="ghcr.io"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: ""},
		},
		{
			name:   "mixed-case parameter keys preserve value casing",
			header: `bearer Realm="https://ghcr.io/token",Service="ghcr.io",Scope="repository:user/repo:pull"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: "repository:user/repo:pull"},
		},
		{
			name:   "mixed-case parameter keys with mixed-case values preserve casing",
			header: `bearer Realm="https://GHCR.IO/token",Service="GHCR.IO",Scope="repository:User/Repo:Pull"`,
			want:   challengeValues{realm: "https://GHCR.IO/token", service: "GHCR.IO", scope: "repository:User/Repo:Pull"},
		},
		{
			name:   "mixed-case realm URL preserves original casing",
			header: `bearer realm="https://Registry.Example.com/Token",service="GHCR.IO",scope="repository:User/Repo:Pull"`,
			want:   challengeValues{realm: "https://Registry.Example.com/Token", service: "GHCR.IO", scope: "repository:User/Repo:Pull"},
		},
		{
			name:   "empty scope is ignored",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope=""`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: ""},
		},
		{
			name:   "extra unknown fields are ignored",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repo:pull",extra="value"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: "repo:pull"},
		},
		{
			name:   "malformed key without equals is ignored",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io",invalidkey`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: ""},
		},
		{
			name:   "empty header returns zero values",
			header: "",
			want:   challengeValues{},
		},
		{
			name:   "header with only bearer returns zero values",
			header: "bearer",
			want:   challengeValues{},
		},
		{
			name:   "bearer without delimiter returns zero values",
			header: "bearer123",
			want:   challengeValues{},
		},
		{
			name:   "quoted commas in scope value are preserved",
			header: `bearer realm="https://ghcr.io/token",service="ghcr.io",scope="repository:user/repo:pull,repository:user/other:pull"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: "repository:user/repo:pull,repository:user/other:pull"},
		},
		{
			name:   "quoted commas in realm value are preserved",
			header: `bearer realm="https://example.com/token,alt",service="ghcr.io"`,
			want:   challengeValues{realm: "https://example.com/token,alt", service: "ghcr.io", scope: ""},
		},
		{
			name:   "whitespace around key and value is trimmed",
			header: `bearer realm = "https://ghcr.io/token", service = "ghcr.io", scope = "repository:user/repo:pull"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: "repository:user/repo:pull"},
		},
		{
			name:   "tabs around key and value are trimmed",
			header: `bearer	realm	=	"https://ghcr.io/token",	service	=	"ghcr.io"`,
			want:   challengeValues{realm: "https://ghcr.io/token", service: "ghcr.io", scope: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseChallenge(tt.header)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test_extractChallengeHost verifies that the challenge host is the host of an
// HTTP or HTTPS realm URL, and empty for any other realm.
func Test_extractChallengeHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		realm string
		want  string
	}{
		{
			name:  "https URL with path",
			realm: "https://ghcr.io/token",
			want:  "ghcr.io",
		},
		{
			name:  "http URL with path",
			realm: "http://localhost:5000/token",
			want:  "localhost:5000",
		},
		{
			name:  "URL with trailing slash",
			realm: "https://registry.example.com/token/",
			want:  "registry.example.com",
		},
		{
			name:  "URL without path",
			realm: "https://ghcr.io",
			want:  "ghcr.io",
		},
		{
			name:  "URL with port number",
			realm: "https://registry.example.com:5000/token",
			want:  "registry.example.com:5000",
		},
		{
			name:  "URL with query parameters returns host only",
			realm: "https://ghcr.io/token?foo=bar",
			want:  "ghcr.io",
		},
		{
			name:  "URL with fragment returns host only",
			realm: "https://ghcr.io/token#section",
			want:  "ghcr.io",
		},
		{
			name:  "unsupported scheme returns empty",
			realm: "ftp://ghcr.io/token",
			want:  "",
		},
		{
			name:  "invalid URL returns empty",
			realm: "not-a-url",
			want:  "",
		},
		{
			name:  "realm without scheme returns empty",
			realm: "ghcr.io/token",
			want:  "",
		},
		{
			name:  "empty realm returns empty",
			realm: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractChallengeHost(newTestLogger(t), tt.realm)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetRegistryAddress verifies the registry host of an image reference:
// Docker Hub references, with or without a registry, use index.docker.io,
// lscr.io references use ghcr.io, other hosts pass through, and a reference
// that does not parse is an error.
func TestGetRegistryAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		imageRef string
		want     string
		wantErr  bool
	}{
		{
			name:     "docker.io maps to index.docker.io",
			imageRef: "docker.io/library/nginx:latest",
			want:     "index.docker.io",
		},
		{
			name:     "docker.io without path maps to index.docker.io",
			imageRef: "docker.io/nginx",
			want:     "index.docker.io",
		},
		{
			name:     "official image without a registry uses Docker Hub",
			imageRef: "watchtower",
			want:     "index.docker.io",
		},
		{
			name:     "user image without a registry uses Docker Hub",
			imageRef: "nickfedor/watchtower",
			want:     "index.docker.io",
		},
		{
			name:     "index.docker.io passes through",
			imageRef: "index.docker.io/library/nginx:latest",
			want:     "index.docker.io",
		},
		{
			name:     "lscr.io maps to ghcr.io",
			imageRef: "lscr.io/linuxserver/nginx:latest",
			want:     "ghcr.io",
		},
		{
			name:     "plain host passes through",
			imageRef: "ghcr.io/example/app:latest",
			want:     "ghcr.io",
		},
		{
			name:     "quay.io passes through",
			imageRef: "quay.io/redhat/ubi8:latest",
			want:     "quay.io",
		},
		{
			name:     "gcr.io passes through",
			imageRef: "gcr.io/project/image:v1.0",
			want:     "gcr.io",
		},
		{
			name:     "ECR host passes through",
			imageRef: "123456789012.dkr.ecr.us-east-1.amazonaws.com/image:tag",
			want:     "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:     "localhost passes through",
			imageRef: "localhost:5000/myimage:latest",
			want:     "localhost:5000",
		},
		{
			name:     "localhost without a port passes through",
			imageRef: "localhost/watchtower",
			want:     "localhost",
		},
		{
			name:     "local host with a port passes through",
			imageRef: "henk:80/watchtower",
			want:     "henk:80",
		},
		{
			name:     "fully qualified host passes through",
			imageRef: "github.com/nicholas-fedor/config",
			want:     "github.com",
		},
		{
			name:     "127.0.0.1 passes through",
			imageRef: "127.0.0.1:5000/myimage:latest",
			want:     "127.0.0.1:5000",
		},
		{
			name:     "empty image ref returns error",
			imageRef: "",
			wantErr:  true,
		},
		{
			name:     "invalid image ref returns error",
			imageRef: ":::invalid:::",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetRegistryAddress(newTestLogger(t), tt.imageRef)
			if tt.wantErr {
				require.ErrorIs(t, err, errFailedParseImageReference)
				assert.Empty(t, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
