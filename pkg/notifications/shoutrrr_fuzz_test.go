package notifications

import (
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

var testFuzzLog = func() *zerolog.Logger {
	l := zerolog.Nop()

	return &l
}()

// FuzzGetShoutrrrTemplate fuzzes the getShoutrrrTemplate function to ensure robust parsing of template strings.
func FuzzGetShoutrrrTemplate(f *testing.F) {
	// Add seeded inputs with valid templates, malformed syntax, and edge cases
	f.Add("{{.}}")
	f.Add("{{range .}}{{.Message}}{{end}}")
	f.Add("porcelain.v1.summary-no-log")
	f.Add("{{ intentionalSyntaxError")
	f.Add("")
	f.Add(strings.Repeat("a", 1000))

	f.Fuzz(func(_ *testing.T, tplString string) {
		//nolint:godox
		// TODO: Remove legacy mode fuzzing when legacy notification types are removed.
		_, _ = getShoutrrrTemplate(testFuzzLog, tplString, true)
		_, _ = getShoutrrrTemplate(testFuzzLog, tplString, false)
	})
}

// FuzzSanitizeURLForLogging fuzzes the sanitizeURLForLogging function to ensure robust URL parsing.
// It tests various URL strings including valid URLs, malformed URLs, and edge cases.
func FuzzSanitizeURLForLogging(f *testing.F) {
	// Add seeded inputs with valid URLs, malformed URLs, and edge cases
	f.Add("https://example.com/path?query=value&another=param")
	f.Add("http://user:pass@host.com:8080/path/to/resource?query=1#fragment")
	f.Add("ftp://ftp.example.com/file.txt")
	f.Add("not-a-url")
	f.Add("://invalid")
	f.Add("")
	f.Add("http://")
	f.Add(
		"https://very-long-domain-name-that-exceeds-normal-limits.com/very/long/path/with/many/segments/and/parameters?param1=value1&param2=value2&param3=value3",
	)
	f.Add("http://localhost:3000/api/v1/users/123?filter=active&sort=name")
	f.Add("mailto:test@example.com")
	f.Add("data:text/plain;base64,SGVsbG8gV29ybGQ=")
	f.Add(strings.Repeat("a", 1000))
	f.Add("http://example.com/" + strings.Repeat("path/", 100))
	f.Add("https://example.com?query=" + strings.Repeat("value", 100))

	f.Fuzz(func(_ *testing.T, rawURL string) {
		// Call sanitizeURLForLogging to ensure it handles all inputs without panicking
		result := sanitizeURLForLogging(rawURL)
		// The function should always return a string, even for invalid inputs
		_ = result
	})
}
