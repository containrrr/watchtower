package events

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzIsOriginAllowed verifies that isOriginAllowed never panics and returns
// a boolean for any combination of origin, host, and allowed origins.
func FuzzIsOriginAllowed(f *testing.F) {
	f.Add("https://example.com", "example.com:8080", "https://example.com")
	f.Add("", "example.com:8080", "")
	f.Add("null", "example.com:8080", "")
	f.Add("https://evil.com", "example.com:8080", "https://trusted.com")
	f.Add("example.com:8080", "example.com:8080", "")
	f.Add("http://example.com:8080", "example.com:8080", "")

	f.Fuzz(func(t *testing.T, origin, host, allowedOriginsStr string) {
		var allowedOrigins []string
		if allowedOriginsStr != "" {
			allowedOrigins = strings.Split(allowedOriginsStr, ",")
		}

		result := isOriginAllowed(origin, host, allowedOrigins)

		assert.True(t, result == true || result == false,
			"isOriginAllowed should return a boolean, got %v", result)

		if origin == "" || origin == "null" {
			assert.True(t, result, "empty or null origin should be allowed")
		}

		if origin == host ||
			origin == "http://"+host ||
			origin == "https://"+host {
			assert.True(t, result, "same-origin should be allowed")
		}

		for _, allowed := range allowedOrigins {
			if allowed == "*" {
				assert.True(t, result, "wildcard should allow all origins")

				break
			}

			if allowed == origin {
				assert.True(t, result, "explicitly allowed origin should be allowed")

				break
			}
		}
	})
}

// FuzzIsOriginAllowedURLParse exercises isOriginAllowed with inputs likely to
// trigger edge cases in url.Parse, including control characters, embedded
// credentials, and malformed schemes.
func FuzzIsOriginAllowedURLParse(f *testing.F) {
	f.Add("https://user:pass@example.com", "example.com", "")
	f.Add("https://example.com\x00/path", "example.com", "")
	f.Add("https://example.com\n", "example.com", "")
	f.Add("https://evil.com%00.example.com", "example.com", "")
	f.Add("javascript:alert(1)", "example.com", "")
	f.Add("file:///etc/passwd", "example.com", "")
	f.Add(strings.Repeat("a", 10000), "example.com", "")
	f.Add("http://[::1]:8080", "[::1]:8080", "")
	f.Add("https://example.com:443", "example.com", "")
	f.Add("\xef\xbb\xbfhttps://example.com", "example.com", "")

	f.Fuzz(func(t *testing.T, origin, host, allowedOriginsStr string) {
		var allowedOrigins []string
		if allowedOriginsStr != "" {
			allowedOrigins = strings.Split(allowedOriginsStr, ",")
		}

		result := isOriginAllowed(origin, host, allowedOrigins)

		assert.True(t, result == true || result == false,
			"isOriginAllowed should return a boolean for origin=%q host=%q, got %v", origin, host, result)

		if origin == "" || origin == "null" {
			assert.True(t, result, "empty or null origin should always be allowed")
		}

		if slices.Contains(allowedOrigins, "*") {
			assert.True(t, result, "wildcard should allow all origins regardless of URL parse outcome")
		}
	})
}
