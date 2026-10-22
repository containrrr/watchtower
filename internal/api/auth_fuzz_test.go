package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

// isValidToken68 reports whether s is a valid RFC 7235 token68 value.
// Only characters A-Z, a-z, 0-9, -, ., _, ~, +, /, = are allowed.
func isValidToken68(s string) bool {
	if len(s) == 0 {
		return false
	}

	paddingStarted := false

	for _, c := range s {
		switch {
		case 'A' <= c && c <= 'Z',
			'a' <= c && c <= 'z',
			'0' <= c && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '+', c == '/':
			if paddingStarted {
				return false
			}
		case c == '=':
			paddingStarted = true
		default:
			return false
		}
	}

	return true
}

// fuzzBearerToken extracts the token portion from a Bearer Authorization
// header, validating it with the same token68 rules as fiber's extractor.
func fuzzBearerToken(authHeader string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}

	token := authHeader[len(prefix):]
	if token == "" {
		return "", false
	}

	if !isValidToken68(token) {
		return "", false
	}

	return token, true
}

// FuzzNewAPIAuthMiddleware verifies that the middleware never panics and
// returns a valid HTTP status code for any combination of configured token
// and Authorization header value, including malformed, empty, or binary inputs.
func FuzzNewAPIAuthMiddleware(f *testing.F) {
	f.Add("secret-token", "Bearer secret-token")
	f.Add("secret-token", "Bearer wrong-token")
	f.Add("secret-token", "")
	f.Add("", "Bearer anything")
	f.Add("token-with-special-chars!@#", "Bearer token-with-special-chars!@#")

	f.Fuzz(func(t *testing.T, token, authHeader string) {
		middleware := newAPIAuthMiddleware(newNopLogger(), token)

		app := fiber.New(fiber.Config{})
		app.Get("/test", middleware, func(c fiber.Ctx) error {
			return c.SendString("authenticated")
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := app.Test(req)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"expected valid HTTP status code, got %d", resp.StatusCode)

		if token == "" {
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
				"empty configured token should always return 401")

			return
		}

		if authHeader == "" {
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
				"missing auth header should return 401")

			return
		}

		provided, ok := fuzzBearerToken(authHeader)
		if !ok {
			return
		}

		expectedHash := sha256.Sum256([]byte(token))
		providedHash := sha256.Sum256([]byte(provided))

		if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1 {
			assert.Equal(t, fiber.StatusOK, resp.StatusCode,
				"valid credentials should return 200")
		} else {
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
				"invalid credentials should return 401")
		}
	})
}

// FuzzAPIAuthMiddleware_CookieFallback verifies the cookie-based authentication
// fallback path for any combination of token and Cookie header value without
// panicking.
func FuzzAPIAuthMiddleware_CookieFallback(f *testing.F) {
	f.Add("secret-token", "access_token=secret-token")
	f.Add("secret-token", "access_token=wrong-token")
	f.Add("secret-token", "")
	f.Add("secret-token", "other_cookie=value")
	f.Add("", "access_token=anything")

	f.Fuzz(func(t *testing.T, token, cookie string) {
		middleware := newAPIAuthMiddleware(newNopLogger(), token)

		app := fiber.New(fiber.Config{})
		app.Get("/test", middleware, func(c fiber.Ctx) error {
			return c.SendString("authenticated")
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		resp, err := app.Test(req)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"expected valid HTTP status code, got %d", resp.StatusCode)

		if token == "" {
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

			return
		}

		if !strings.HasPrefix(cookie, "access_token=") {
			return
		}

		provided := strings.TrimPrefix(cookie, "access_token=")

		// RFC 6265 cookie values exclude whitespace, quotes, commas, semicolons,
		// backslashes, and control characters, which the server does not carry
		// over, so only a valid cookie value can be compared as it was sent.
		if !isCookieValue(provided) {
			return
		}

		expectedHash := sha256.Sum256([]byte(token))
		providedHash := sha256.Sum256([]byte(provided))

		if subtle.ConstantTimeCompare(expectedHash[:], providedHash[:]) == 1 {
			assert.Equal(t, fiber.StatusOK, resp.StatusCode,
				"matching cookie should return 200")
		} else {
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
				"non-matching cookie should return 401")
		}
	})
}

// FuzzAPIAuthMiddleware_HeaderVariations exercises mixed-case schemes,
// whitespace, and tab characters in the Authorization header to confirm
// the middleware never panics and always returns a valid status code.
func FuzzAPIAuthMiddleware_HeaderVariations(f *testing.F) {
	f.Add("secret", "Bearer secret")
	f.Add("secret", "bearer secret")
	f.Add("secret", "BEARER secret")
	f.Add("secret", "Bearer  secret")
	f.Add("secret", "Bearer secret ")
	f.Add("secret", "Bearer\tsecret")

	f.Fuzz(func(t *testing.T, token, authHeader string) {
		middleware := newAPIAuthMiddleware(newNopLogger(), token)

		app := fiber.New(fiber.Config{})
		app.Get("/test", middleware, func(c fiber.Ctx) error {
			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := app.Test(req)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		assert.True(t, resp.StatusCode == fiber.StatusOK || resp.StatusCode == fiber.StatusUnauthorized,
			"expected 200 or 401, got %d", resp.StatusCode)
	})
}

// FuzzAPIAuthMiddleware_NoPanic ensures the middleware never panics regardless
// of token or header content, including binary data, very long strings, and
// special characters.
func FuzzAPIAuthMiddleware_NoPanic(f *testing.F) {
	f.Add("", "")
	f.Add("token", "Bearer token")
	f.Add(strings.Repeat("a", 10000), "Bearer "+strings.Repeat("a", 10000))

	f.Fuzz(func(t *testing.T, token, authHeader string) {
		middleware := newAPIAuthMiddleware(newNopLogger(), token)

		app := fiber.New(fiber.Config{})
		app.Get("/test", middleware, func(c fiber.Ctx) error {
			return c.SendString("ok")
		})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := app.Test(req)
		if err != nil {
			return
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		_ = body

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 600,
			"expected valid HTTP status code, got %d", resp.StatusCode)
	})
}

// isCookieValue reports whether value consists only of RFC 6265 cookie-octets.
//
// Parameters:
//   - value: Candidate cookie value.
//
// Returns:
//   - bool: True when every byte is a cookie-octet.
func isCookieValue(value string) bool {
	for i := range len(value) {
		b := value[i]
		if b < 0x21 || b > 0x7e || b == '"' || b == ',' || b == ';' || b == '\\' {
			return false
		}
	}

	return true
}
