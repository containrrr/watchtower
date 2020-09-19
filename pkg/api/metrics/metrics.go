package metrics

import (
	metrics2 "github.com/containrrr/watchtower/pkg/metrics"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler is an HTTP handle for serving metric data
type Handler struct {
	Path    string
	Handle  http.HandlerFunc
	Metrics *metrics2.Metrics
}

// New is a factory function creating a new Metrics instance
func New() *Handler {
	metrics := metrics2.New()
	handler := promhttp.Handler()

	return &Handler{
		Path:    "/v1/metrics",
		Handle:  handler.ServeHTTP,
		Metrics: metrics,
	}
}
