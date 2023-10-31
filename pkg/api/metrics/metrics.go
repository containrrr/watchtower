package metrics

import (
	. "github.com/containrrr/watchtower/pkg/api/prelude"
	"github.com/containrrr/watchtower/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GetV1 creates a new metrics http handler
func GetV1() HandlerFunc {
	// Initialize watchtower metrics
	metrics.Init()
	return WrapHandler(promhttp.Handler().ServeHTTP)
}
