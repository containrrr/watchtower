// Package metrics provides tracking and exposure of Watchtower scan metrics.
// It integrates with Prometheus to monitor container scan outcomes.
//
// Key components:
//   - Metrics: Handles metric queuing and updates.
//   - NewMetric: Creates metrics from scan reports.
//
// Usage example:
//
//	m := metrics.Default()
//	m.RegisterScan(metrics.NewMetric(report))
//
// The package uses Prometheus for metrics exposure and integrates with types.Report.
package metrics
