package api

import (
	"github.com/containrrr/watchtower/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var (
	lock chan bool
)

// NewUpdateHandler is a factory function creating a new  Handler instance
func NewUpdateHandler(updateFn func() *metrics.Metric, updateLock chan bool) *UpdateHandler {
	if updateLock != nil {
		lock = updateLock
	} else {
		lock = make(chan bool, 1)
		lock <- true
	}

	return &UpdateHandler{
		fn:   updateFn,
		Path: "/v1/update",
	}
}

func UpdateEndpoint(updateFn func() *metrics.Metric, updateLock chan bool) (path string, handler http.HandlerFunc) {
	uh := NewUpdateHandler(updateFn, updateLock)
	return uh.Path, uh.Handle
}

// UpdateHandler is an API handler used for triggering container update scans
type UpdateHandler struct {
	fn   func() *metrics.Metric
	Path string
}

// Handle is the actual http.Handle function doing all the heavy lifting
func (handler *UpdateHandler) Handle(w http.ResponseWriter, _ *http.Request) {
	log.Info("Updates triggered by HTTP API request.")

	result := updateResult{}

	select {
	case chanValue := <-lock:
		defer func() { lock <- chanValue }()
		metric := handler.fn()
		metrics.RegisterScan(metric)
		result.Result = metric
		result.Skipped = false
	default:
		log.Debug("Skipped. Another update already running.")
		result.Skipped = true
	}
	WriteJsonOrError(w, result)
}

type updateResult struct {
	Skipped bool
	Result  *metrics.Metric
}
