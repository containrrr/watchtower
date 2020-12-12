package flags

import (
	. "time"
)

// WatchConfig is the global watchtower configuration created from flags and environment variables
type WatchConfig struct {
	Interval          int
	Schedule          string
	NoPull            bool `mapstructure:"no-pull"`
	NoRestart         bool `mapstructure:"no-restart"`
	NoStartupMessage  bool `mapstructure:"no-startup-message"`
	Cleanup           bool
	RemoveVolumes     bool `mapstructure:"remove-volumes"`
	EnableLabel       bool `mapstructure:"label-enable"`
	Debug             bool
	Trace             bool
	MonitorOnly       bool     `mapstructure:"monitor-only"`
	RunOnce           bool     `mapstructure:"run-once"`
	IncludeStopped    bool     `mapstructure:"include-stopped"`
	IncludeRestarting bool     `mapstructure:"include-restarting"`
	ReviveStopped     bool     `mapstructure:"revive-stopped"`
	LifecycleHooks    bool     `mapstructure:"enable-lifecycle-hooks"`
	RollingRestart    bool     `mapstructure:"rolling-restart"`
	HTTPAPI           bool     `mapstructure:"http-api"`
	HTTPAPIToken      string   `mapstructure:"http-api-token"`
	Timeout           Duration `mapstructure:"stop-timeout"`
	Scope             string
	NoColor           bool `mapstructure:"no-color"`
}
