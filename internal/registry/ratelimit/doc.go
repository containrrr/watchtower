// Package ratelimit paces registry requests and retries the ones a registry
// rate limits.
//
// It reads the wait a registry asks for from Retry-After headers, quota
// descriptions in response bodies, and Docker pull errors, and tracks each
// host's cooldown and advertised quota so that requests to the same host share
// its budget.
package ratelimit
