// Package api runs Watchtower's HTTP API server.
//
// The server is built on Fiber. It serves the health probes, the authenticated
// endpoints that trigger and check updates and report Watchtower's state, and a
// Server-Sent Events stream of update events. It shuts down gracefully when its
// context is canceled.
package api
