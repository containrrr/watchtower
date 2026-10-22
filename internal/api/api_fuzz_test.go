package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzGetAPIAddr verifies that GetAPIAddr correctly formats addresses for any
// host/port combination, including IPv6 addresses, without panicking.
func FuzzGetAPIAddr(f *testing.F) {
	f.Add("localhost", "8080")
	f.Add("127.0.0.1", "8080")
	f.Add("::1", "8080")
	f.Add("2001:db8::1", "8080")
	f.Add("[::1]", "8080")
	f.Add("", "")
	f.Add("example.com", "")
	f.Add("", "8080")
	f.Add("host with spaces", "8080")
	f.Add("host\x00null", "8080")

	f.Fuzz(func(t *testing.T, host, port string) {
		addr := GetAPIAddr(host, port)

		if host == "" && port == "" {
			assert.Empty(t, addr, "empty host and port should return empty")

			return
		}

		assert.NotEmpty(t, addr, "GetAPIAddr should not return empty for non-empty inputs")

		if port == "" {
			assert.Equal(t, host, addr, "empty port should return host only")

			return
		}

		if host == "" {
			assert.Equal(t, ":"+port, addr, "empty host should return :port")

			return
		}

		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			assert.True(t, strings.HasPrefix(addr, "["),
				"IPv6 host should be bracketed: %s", addr)
		}
	})
}
