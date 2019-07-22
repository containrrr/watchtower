package actions

import (
	t "github.com/containrrr/watchtower/pkg/types"
	"time"
)

// UpdateParams contains all different options available to alter the behavior of the Update func
type UpdateParams struct {
	Filter      t.Filter
	Cleanup     bool
	NoRestart   bool
	Timeout     time.Duration
	MonitorOnly bool
}
