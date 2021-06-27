package api

import (
	"github.com/containrrr/watchtower/pkg/metrics"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler is a HTTP handler for serving metric data
type MetricsHandler struct {
	Path    string
	Handle  http.HandlerFunc
	Metrics *metrics.Metrics
}

// NewMetricsHandler is a factory function creating a new Metrics instance
func NewMetricsHandler() *MetricsHandler {
	m := metrics.Default()
	handler := promhttp.Handler()

	return &MetricsHandler{
		Path:    "/v1/metrics",
		Handle:  handler.ServeHTTP,
		Metrics: m,
	}
}

func MetricsEndpoint() (path string, handler http.HandlerFunc) {
	mh := NewMetricsHandler()
	return mh.Path, mh.Handle
}
