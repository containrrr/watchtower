package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics provides a HTTP endpoint for Prometheus to fetch metrics from
type Metrics struct {
	scanned prometheus.Gauge
	updated prometheus.Gauge
	failed  prometheus.Gauge
	total   prometheus.Counter
	skipped prometheus.Counter
}

// NewMetrics is a factory function creating a new Metrics instance
func NewMetrics(port string) *Metrics {
	metrics := &Metrics{}
	metrics.scanned = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "watchtower_containers_scanned",
		Help: "Number of containers scanned for changes by watchtower during the last scan",
	})
	metrics.updated = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "watchtower_containers_updated",
		Help: "Number of containers updated by watchtower during the last scan",
	})
	metrics.failed = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "watchtower_containers_failed",
		Help: "Number of containers where update failed during the last scan",
	})
	metrics.total = promauto.NewCounter(prometheus.CounterOpts{
		Name: "watchtower_scans_total",
		Help: "Number of scans since last restart",
	})
	metrics.skipped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "watchtower_scans_skipped",
		Help: "Number of skipped scans since last restart",
	})
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":"+port, nil)
	}()

	return metrics
}

// RegisterSkipped increments scans and registers the last one as skipped (rescheduled)
func (metrics *Metrics) RegisterSkipped() {
	metrics.total.Inc()
	metrics.skipped.Inc()
}

// RegisterScan registers metrics for an executed scan
func (metrics *Metrics) RegisterScan(scanned float64, updated float64, failed float64) {
	metrics.total.Inc()
	metrics.scanned.Set(scanned)
	metrics.updated.Set(updated)
	metrics.failed.Set(failed)
}
