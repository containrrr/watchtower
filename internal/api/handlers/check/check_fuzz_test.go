package check

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxFuzzQueryLength bounds the encoded query of a fuzzed request, so the request
// line fits the server's 8 KiB read buffer, which rejects longer request lines
// before any handler runs.
const maxFuzzQueryLength = 4096

// FuzzExtractFilterParams verifies that extractFilterParams never panics and
// correctly parses query parameters with various delimiters and encodings.
func FuzzExtractFilterParams(f *testing.F) {
	f.Add("name", "nginx,redis,mysql")
	f.Add("name", "nginx")
	f.Add("name", "")
	f.Add("name", "nginx redis")
	f.Add("name", "nginx,,redis")
	f.Add("name", ",nginx,")
	f.Add("name", "nginx%20redis")
	f.Add("image", "nginx:latest,redis:7")
	f.Add("image", strings.Repeat("a,", 100))
	f.Add("image", "nginx:latest,redis:7,mysql:8.0,postgres:15")

	f.Fuzz(func(t *testing.T, key, rawValue string) {
		app := fiber.New(fiber.Config{})
		app.Get("/test", func(c fiber.Ctx) error {
			results := extractFilterParams(c, key)

			for _, r := range results {
				assert.NotEmpty(t, r, "extractFilterParams should not return empty strings")
			}

			return c.SendString("ok")
		})

		// Encode both the key and the value, so any fuzzed string reaches the
		// handler as it is instead of breaking the request line.
		query := url.Values{key: {rawValue}}.Encode()
		if len(query) > maxFuzzQueryLength {
			t.Skip("request line longer than the server reads")
		}

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test?"+query, nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		defer resp.Body.Close()

		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 500,
			"expected valid status code, got %d", resp.StatusCode)
	})
}
