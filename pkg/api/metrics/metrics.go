package metrics

import (
	"github.com/containrrr/watchtower/pkg/metrics"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler is an HTTP handle for serving metric data
type Handler struct {
	Path    string
	Handle  http.HandlerFunc
	Metrics *metrics.Metrics
}

// New is a factory function creating a new Metrics instance
func New() *Handler {
	m := metrics.Default()
	handler := promhttp.Handler()

	return &Handler{
		Path:    "/v1/metrics",
		Handle:  handler.ServeHTTP,
		Metrics: m,
	}
}
