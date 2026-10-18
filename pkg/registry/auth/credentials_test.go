package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformAuth(t *testing.T) {
	tests := []struct {
		name         string
		registryAuth string
		want         string
	}{
		{
			name:         "valid base64 encoded JSON credentials",
			registryAuth: "eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwicGFzc3dvcmQiOiJ0ZXN0cGFzcyJ9",
			want:         "dGVzdHVzZXI6dGVzdHBhc3M=",
		},
		{
			name:         "empty string returns empty",
			registryAuth: "",
			want:         "",
		},
		{
			name:         "valid base64 but invalid JSON returns original input",
			registryAuth: "aGVsbG8=",
			want:         "aGVsbG8=",
		},
		{
			name:         "invalid base64 returns original input",
			registryAuth: "not-valid-base64!!!",
			want:         "not-valid-base64!!!",
		},
		{
			name:         "valid base64 JSON with only username returns original",
			registryAuth: "eyJ1c2VybmFtZSI6InVzZXIifQ==",
			want:         "eyJ1c2VybmFtZSI6InVzZXIifQ==",
		},
		{
			name:         "valid base64 JSON with only password becomes Basic auth",
			registryAuth: "eyJwYXNzd29yZCI6InBhc3MifQ==",
			want:         "OnBhc3M=", // ":pass"
		},
		{
			name:         "valid base64 empty JSON object returns original",
			registryAuth: "e30=",
			want:         "e30=",
		},
		{
			name:         "base64 of plain text returns original input",
			registryAuth: "dGVzdA==",
			want:         "dGVzdA==",
		},
		{
			name:         "base64 with null username returns original",
			registryAuth: "eyJ1c2VybmFtZSI6bnVsbCwgcGFzc3dvcmQiOiJwYXNzIn0=",
			want:         "eyJ1c2VybmFtZSI6bnVsbCwgcGFzc3dvcmQiOiJwYXNzIn0=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TransformAuth(newTestLogger(t), tt.registryAuth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransformAuthURLEncodedJSONCredentials(t *testing.T) {
	// Password '?' (0x3f) yields URL-safe '-' in the JSON blob when URL-encoded,
	// which StdEncoding alone cannot decode.
	creds := map[string]string{
		"username": "u",
		"password": strings.Repeat("?", 8),
	}
	buf, err := json.Marshal(creds)
	require.NoError(t, err)

	urlEncoded := base64.URLEncoding.EncodeToString(buf)
	_, stdErr := base64.StdEncoding.DecodeString(urlEncoded)
	require.Error(t, stdErr, "fixture must fail StdEncoding so the URL path is exercised")

	got := TransformAuth(newTestLogger(t), urlEncoded)
	decoded, err := base64.StdEncoding.DecodeString(got)
	require.NoError(t, err)
	assert.Equal(t, "u:"+strings.Repeat("?", 8), string(decoded))
}

func TestTransformAuthIdentityToken(t *testing.T) {
	creds := map[string]string{
		"identitytoken": "ecr-session-token",
	}
	buf, err := json.Marshal(creds)
	require.NoError(t, err)

	encoded := base64.StdEncoding.EncodeToString(buf)
	got := TransformAuth(newTestLogger(t), encoded)
	decoded, err := base64.StdEncoding.DecodeString(got)
	require.NoError(t, err)
	assert.Equal(t, ":ecr-session-token", string(decoded))
}

func TestTransformAuthPasswordOnly(t *testing.T) {
	creds := map[string]string{
		"password": "ghcr-pat-token",
	}
	buf, err := json.Marshal(creds)
	require.NoError(t, err)

	encoded := base64.StdEncoding.EncodeToString(buf)
	got := TransformAuth(newTestLogger(t), encoded)
	decoded, err := base64.StdEncoding.DecodeString(got)
	require.NoError(t, err)
	assert.Equal(t, ":ghcr-pat-token", string(decoded))
}
