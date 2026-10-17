// Package ratelimit parses registry 429 responses and retries them with backoff.
//
// It honors HTTP Retry-After headers, body quotas, such as "allowed: 44000/minute"
// and Docker pull-stream toomanyrequests messages.
// A [Limiter] holds the cooldowns and quotas, so requests share a host's budget
// when they go through the same Limiter. Advertised quotas are treated as a fill rate.
// After a throttle, each host is limited to one outstanding token so concurrent
// checks cannot dump the advertised budget in a burst.
// Unauthenticated GHCR traffic is serialized from the first request onto a
// separate limiter key from [Scope] so anonymous 429s cannot pace authenticated
// checks. A Retry-After below the floor is a token-bucket signal, so it is retried
// with exponential backoff from the floor toward the honor window for up to the
// bucket budget, counted from the first throttle. A usable Retry-After is
// honored as sent for up to the honor window. In-cycle retries are debug. Giving
// up is a warning only when the registry asked for a wait beyond the honor
// window.
package ratelimit
