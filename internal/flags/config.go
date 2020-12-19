package flags

import (
	"time"
)

// WatchConfig is the global watchtower configuration created from flags and environment variables
type WatchConfig struct {
	Interval               int
	Schedule               string
	NoPull                 bool   `mapstructure:"no-pull"`
	NoRestart              bool   `mapstructure:"no-restart"`
	NoStartupMessage       bool   `mapstructure:"no-startup-message"`
	Cleanup                bool
	RemoveVolumes          bool   `mapstructure:"remove-volumes"`
	EnableLabel            bool   `mapstructure:"label-enable"`
	Debug                  bool
	Trace                  bool
	MonitorOnly            bool   `mapstructure:"monitor-only"`
	RunOnce                bool   `mapstructure:"run-once"`
	IncludeStopped         bool   `mapstructure:"include-stopped"`
	IncludeRestarting      bool   `mapstructure:"include-restarting"`
	ReviveStopped          bool   `mapstructure:"revive-stopped"`
	LifecycleHooks         bool   `mapstructure:"enable-lifecycle-hooks"`
	RollingRestart         bool   `mapstructure:"rolling-restart"`
	HTTPAPIToken           string `mapstructure:"http-api-token"`
	Scope                  string
	EnableUpdateAPI        bool   `mapstructure:"http-api-update"`
	EnableMetricsAPI       bool   `mapstructure:"http-api-metrics"`
	UpdateAPIWithScheduler bool   `mapstructure:"http-api-periodic-polls"`
	WarnOnHeadFailed       string `mapstructure:"warn-on-head-failure"`
	NoColor                bool   `mapstructure:"no-color"`
	Timeout                time.Duration `mapstructure:"stop-timeout"`
}
