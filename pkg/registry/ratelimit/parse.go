package ratelimit

import (
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxRetryAfterSeconds is the longest Retry-After delta, in seconds, that fits in a [time.Duration].
const maxRetryAfterSeconds = int64(math.MaxInt64 / time.Second)

var (
	retryAfterBody = regexp.MustCompile(`(?i)retry-after:\s*((?:[0-9]+(?:\.[0-9]+)?(?:ns|us|µs|μs|ms|s|m|h))+)\b`)
	allowedBody    = regexp.MustCompile(`(?i)allowed:\s*([0-9]+)\s*/\s*(seconds?|secs?|minutes?|mins?|hours?|hrs?)`)
	rateLimitLimit = regexp.MustCompile(`(?i)([0-9]+)(?:\s*,\s*[0-9]+)*\s*(?:;\s*w=([0-9]+))?`)
	status429      = regexp.MustCompile(
		`(?i)(?:\b(?:https?|status(?:\s+code)?)\s*[:\-]?\s*429\b|\breturned\s+429\b|\b429\s+(?:too many|error|response))`,
	)
)

// ParseRetryAfterHeader parses an HTTP Retry-After header.
//
// It accepts a delta-seconds integer or an HTTP-date.
// Dates in the past and a value of 0 are treated as absent.
// A delta too long for a [time.Duration] is capped at the longest duration.
//
// Parameters:
//   - header: Raw Retry-After header value.
//
// Returns:
//   - time.Duration: How long to wait.
//   - bool: True when the header contained a usable wait.
func ParseRetryAfterHeader(header string) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}

	// A value beyond the int64 range fails with ErrRange and returns the nearest
	// int64, which the checks below handle like any other value. ParseInt reports
	// ErrRange before it reads the rest of the value, so the value must also be
	// all digits after one optional sign.
	seconds, parseErr := strconv.ParseInt(header, 10, 64)

	digits := header
	if digits[0] == '+' || digits[0] == '-' {
		digits = digits[1:]
	}

	overflowed := errors.Is(parseErr, strconv.ErrRange) && strings.Trim(digits, "0123456789") == ""

	if parseErr == nil || overflowed {
		if seconds <= 0 {
			return 0, false
		}

		// Cap a wait too long for a duration so the conversion cannot overflow.
		if seconds > maxRetryAfterSeconds {
			return time.Duration(math.MaxInt64), true
		}

		return time.Duration(seconds) * time.Second, true
	}

	when, err := http.ParseTime(header)
	if err != nil {
		return 0, false
	}

	wait := time.Until(when)
	if wait <= 0 {
		return 0, false
	}

	return wait, true
}

// ParseQuotaMessage extracts retry-after and allowed-quota fields from a registry body.
//
// It understands GHCR-style text such as
// "toomanyrequests: retry-after: 331.163µs, allowed: 44000/minute".
//
// Parameters:
//   - message: Response body or Docker pull-stream error text.
//
// Returns:
//   - time.Duration: Parsed retry-after. Zero when absent.
//   - int: Advertised request budget. Zero when absent.
//   - time.Duration: Window for the budget. Zero when absent.
func ParseQuotaMessage(message string) (time.Duration, int, time.Duration) {
	var (
		retryAfter time.Duration
		allowed    int
		window     time.Duration
	)

	if match := retryAfterBody.FindStringSubmatch(message); len(match) == retryAfterCaptureCount {
		parsed, parseErr := time.ParseDuration(match[1])
		if parseErr == nil {
			retryAfter = parsed
		}
	}

	if match := allowedBody.FindStringSubmatch(message); len(match) == allowedCaptureCount {
		n, convErr := strconv.Atoi(match[1])
		if convErr == nil {
			allowed = n
			window = parseQuotaWindow(match[2])
		}
	}

	return retryAfter, allowed, window
}

// FromResponse builds a rate-limit error from an HTTP response and optional body.
//
// Header Retry-After wins over a body retry-after because it is the protocol
// signal. Body "allowed" values win over RateLimit-Limit when both are present
// because GHCR puts the live budget in the body.
//
// Parameters:
//   - resp: HTTP response. Must not be nil.
//   - body: Already-read body bytes. May be empty.
//
// Returns:
//   - *Error: Parsed rate-limit error.
func FromResponse(resp *http.Response, body []byte) *Error {
	info := &Error{
		StatusCode: http.StatusTooManyRequests,
		Message:    strings.TrimSpace(string(body)),
	}
	if resp != nil {
		info.StatusCode = resp.StatusCode

		if resp.Request != nil && resp.Request.URL != nil {
			info.Host = resp.Request.URL.Host
		}

		if wait, ok := ParseRetryAfterHeader(resp.Header.Get("Retry-After")); ok {
			info.RetryAfter = wait
		}

		if allowed, window := parseRateLimitLimit(resp.Header.Get("Ratelimit-Limit")); allowed > 0 {
			info.Allowed = allowed
			info.AllowedWindow = window
		}

		if info.Allowed == 0 {
			if allowed, window := parseRateLimitLimit(resp.Header.Get("X-Ratelimit-Limit")); allowed > 0 {
				info.Allowed = allowed
				info.AllowedWindow = window
			}
		}
	}

	retryAfter, allowed, window := ParseQuotaMessage(info.Message)
	if info.RetryAfter == 0 {
		info.RetryAfter = retryAfter
	}

	if allowed > 0 {
		info.Allowed = allowed
		info.AllowedWindow = window
	}

	return info
}

// FromErrorMessage builds a rate-limit error from Docker pull-stream text.
//
// Some registries advertise a throttle only through retry-after text, with no
// 429 or "too many requests" token. That path requires a duration that
// [time.ParseDuration] accepts.
//
// Parameters:
//   - message: Stream error or registry body. Empty or unrelated text returns nil.
//
// Returns:
//   - *Error: Parsed rate-limit error, or nil when the message is not a 429.
func FromErrorMessage(message string) *Error {
	retryAfter, allowed, window := ParseQuotaMessage(message)
	if retryAfter == 0 && !looksRateLimited(message) {
		return nil
	}

	return &Error{
		StatusCode:    http.StatusTooManyRequests,
		RetryAfter:    retryAfter,
		Allowed:       allowed,
		AllowedWindow: window,
		Message:       strings.TrimSpace(message),
	}
}

// ReadBody reads a small prefix of resp.Body for rate-limit parsing.
//
// Parameters:
//   - resp: HTTP response. May be nil.
//   - limit: Maximum bytes to read. Values below 1 use 4 KiB.
//
// Returns:
//   - []byte: Body prefix. Empty when the response has no body.
func ReadBody(resp *http.Response, limit int64) []byte {
	if resp == nil || resp.Body == nil {
		return nil
	}

	if limit < 1 {
		limit = DefaultBodyLimit
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return body
	}

	return body
}

// looksRateLimited reports whether message looks like a registry 429.
//
// A bare 429 is recognized only with HTTP or status context so byte counts,
// ports, and digest hashes are ignored.
//
// Parameters:
//   - message: Registry body or Docker pull-stream error text.
//
// Returns:
//   - bool: True when the text contains a rate-limit marker.
func looksRateLimited(message string) bool {
	lower := strings.ToLower(message)

	return strings.Contains(lower, "toomanyrequests") ||
		strings.Contains(lower, "too many requests") ||
		status429.MatchString(lower)
}

// parseQuotaWindow maps an allowed-quota unit word to a duration.
//
// Parameters:
//   - unit: Unit from an "allowed: N/unit" field such as minute or hour.
//
// Returns:
//   - time.Duration: Matching window. Zero when the unit is unknown.
func parseQuotaWindow(unit string) time.Duration {
	switch strings.ToLower(unit) {
	case "second", "seconds", "sec", "secs":
		return time.Second
	case "minute", "minutes", "min", "mins":
		return time.Minute
	case "hour", "hours", "hr", "hrs":
		return time.Hour
	default:
		return 0
	}
}

// parseRateLimitLimit parses a RateLimit-Limit or X-RateLimit-Limit header.
//
// The header may look like "100;w=3600" or "3000, 3000;w=60".
//
// Parameters:
//   - header: Raw rate-limit header value.
//
// Returns:
//   - int: Advertised request budget. Zero when the header is unusable.
//   - time.Duration: Window from the w= parameter. Zero when absent.
func parseRateLimitLimit(header string) (int, time.Duration) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, 0
	}

	match := rateLimitLimit.FindStringSubmatch(header)
	if match == nil {
		return 0, 0
	}

	allowed, err := strconv.Atoi(match[1])
	if err != nil || allowed <= 0 {
		return 0, 0
	}

	window := time.Duration(0)

	if match[2] != "" {
		seconds, convErr := strconv.Atoi(match[2])
		if convErr == nil && seconds > 0 {
			window = time.Duration(seconds) * time.Second
		}
	}

	return allowed, window
}
