// Package config resolves Watchtower's settings into a single immutable Config.
//
// Settings are grouped by domain in its subpackages. Config projects them into
// the update parameters and Docker client options used by the run-once,
// scheduled, and HTTP API update paths, so every path works from the same
// resolved settings.
package config
