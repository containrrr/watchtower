package notifications

import "net/url"

// redactServiceURL returns a non-sensitive summary of a notification service URL.
//
// Scheme is preserved so operators can see the service type.
// userinfo, host, path, and query (often tokens) are replaced.
//
// Parameters:
//   - raw: Full service or webhook URL that may embed credentials.
//
// Returns:
//   - string: Redacted form such as "slack://[redacted]", or "[redacted]" when empty/invalid.
func redactServiceURL(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return "[redacted]"
	}

	return parsed.Scheme + "://[redacted]"
}

// redactServiceURLs redacts each URL in urls.
//
// Parameters:
//   - urls: Full service URLs that may embed credentials.
//
// Returns:
//   - []string: Redacted summaries in the same order.
func redactServiceURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}

	out := make([]string, len(urls))
	for i, raw := range urls {
		out[i] = redactServiceURL(raw)
	}

	return out
}
