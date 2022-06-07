package update

import (
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	lock chan bool
)

// New is a factory function creating a new  Handler instance
func New(updateFn func(images []string), updateLock chan bool) *Handler {
	if updateLock != nil {
		lock = updateLock
	} else {
		lock = make(chan bool, 1)
		lock <- true
	}

	return &Handler{
		fn:   updateFn,
		Path: "/v1/update",
	}
}

// Handler is an API handler used for triggering container update scans
type Handler struct {
	fn   func(images []string)
	Path string
}

// Handle is the actual http.Handle function doing all the heavy lifting
func (handle *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	log.Info("Updates triggered by HTTP API request.")

	_, err := io.Copy(os.Stdout, r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	var images []string
	imageQueries, found := r.URL.Query()["image"]
	if found {
		for _, image := range imageQueries {
			images = append(images, strings.Split(image, ",")...)
		}

	} else {
		images = nil
	}

	if len(images) > 0 {
		chanValue := <-lock
		defer func() { lock <- chanValue }()
		handle.fn(images)
	} else {
		select {
		case chanValue := <-lock:
			defer func() { lock <- chanValue }()
			handle.fn(images)
		default:
			log.Debug("Skipped. Another update already running.")
		}
	}

}
