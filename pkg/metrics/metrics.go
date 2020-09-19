package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var metrics *Metrics

type Metric struct {
	Scanned int
	Updated int
	Failed  int
}

type Metrics struct {
	channel chan *Metric
	scanned prometheus.Gauge
	updated prometheus.Gauge
	failed  prometheus.Gauge
	total   prometheus.Counter
	skipped prometheus.Counter
}

// RegisterScan registers metrics for an executed scan
func (metrics *Metrics) Register(metric *Metric) {
	metrics.channel <- metric
}

func New() *Metrics {
	metrics = &Metrics{
		scanned: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "watchtower_containers_scanned",
			Help: "Number of containers scanned for changes by watchtower during the last scan",
		}),
		updated: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "watchtower_containers_updated",
			Help: "Number of containers updated by watchtower during the last scan",
		}),
		failed: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "watchtower_containers_failed",
			Help: "Number of containers where update failed during the last scan",
		}),
		total: promauto.NewCounter(prometheus.CounterOpts{
			Name: "watchtower_scans_total",
			Help: "Number of scans since the watchtower started",
		}),
		skipped: promauto.NewCounter(prometheus.CounterOpts{
			Name: "watchtower_scans_skipped",
			Help: "Number of skipped scans since watchtower started",
		}),
		channel: make(chan *Metric, 10),
	}


	go metrics.HandleUpdate(metrics.channel)

	return metrics
}

func RegisterScan(metric *Metric) {
	metrics := New()
	metrics.Register(metric)
}

func (metrics *Metrics) HandleUpdate(channel <-chan *Metric) {
	for change := range channel {
		if change == nil {
			// Update was skipped and rescheduled
			metrics.total.Inc()
			metrics.skipped.Inc()
			continue
		}
		// Update metrics with the new values
		metrics.total.Inc()
		metrics.scanned.Set(float64(change.Scanned))
		metrics.updated.Set(float64(change.Updated))
		metrics.failed.Set(float64(change.Failed))
	}
}