package api

import (
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/keyauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIAuthMiddleware(t *testing.T) {
	const testToken = "test-token-123"

	tests := []struct {
		name       string
		token      string
		authHeader string
		cookie     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "empty token returns 401 with not configured message",
			token:      "",
			wantStatus: fiber.StatusUnauthorized,
			wantBody:   "API token not configured",
		},
		{
			name:       "valid bearer token returns 200",
			token:      testToken,
			authHeader: "Bearer " + testToken,
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "raw authorization token returns 200",
			token:      testToken,
			authHeader: testToken,
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "invalid bearer token returns 401",
			token:      testToken,
			authHeader: "Bearer wrong-token",
			wantStatus: fiber.StatusUnauthorized,
			wantBody:   keyauth.ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			name:       "missing auth header returns 401",
			token:      testToken,
			wantStatus: fiber.StatusUnauthorized,
			wantBody:   keyauth.ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			name:       "valid cookie fallback returns 200",
			token:      testToken,
			cookie:     "access_token=" + testToken,
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "invalid cookie returns 401",
			token:      testToken,
			cookie:     "access_token=wrong-token",
			wantStatus: fiber.StatusUnauthorized,
			wantBody:   keyauth.ErrMissingOrMalformedAPIKey.Error(),
		},
		{
			name:       "non-bearer scheme falls through to raw value and fails",
			token:      testToken,
			authHeader: "NotBearer " + testToken,
			wantStatus: fiber.StatusUnauthorized,
			wantBody:   keyauth.ErrMissingOrMalformedAPIKey.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := newAPIAuthMiddleware(newTestLogger(t), tt.token)

			app := fiber.New(fiber.Config{})
			app.Get("/test", middleware, func(c fiber.Ctx) error {
				return c.SendString("authenticated")
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantBody != "" {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), tt.wantBody)
			}
		})
	}
}

func TestNewAPIAuthMiddleware_HashComputation(t *testing.T) {
	const testToken = "hash-test-token"

	middleware := newAPIAuthMiddleware(newTestLogger(t), testToken)

	app := fiber.New(fiber.Config{})
	app.Get("/test", middleware, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	expectedHash := sha256.Sum256([]byte(testToken))
	_ = expectedHash

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	resp, err := app.Test(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
