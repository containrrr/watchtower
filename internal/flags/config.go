package flags

import "time"

// WatchConfig is the global watchtower configuration created from flags and environment variables
type WatchConfig struct {
	Cleanup           bool
	NoRestart         bool
	NoStartupMessage  bool
	RunOnce           bool
	MonitorOnly       bool
	EnableLabel       bool
	LifecycleHooks    bool
	RollingRestart    bool
	NoPull            bool
	IncludeStopped    bool
	IncludeRestarting bool
	ReviveStopped     bool
	RemoveVolumes     bool
	HTTPAPI           bool
	HTTPAPIToken      string
	Timeout           time.Duration
	Scope             string
	Schedule          string
}
