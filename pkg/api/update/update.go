package update

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

var (
	lock chan bool
)

// New is a factory function creating a new  Handler instance
func New(updateFn func(), token string) *Handler {
	lock = make(chan bool, 1)
	lock <- true

	return &Handler{
		fn:    updateFn,
		token: token,
		Path:  "/v1/update",
	}
}

// Handler is an API handler used for triggering container update scans
type Handler struct {
	fn    func()
	token string
	Path  string
}

// Handle is the actual http.Handle function doing all the heavy lifting
func (handle *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	log.Info("Updates triggered by HTTP API request.")

	_, err := io.Copy(os.Stdout, r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	if r.Header.Get("Token") != handle.token {
		log.Error("Invalid token. Not updating.")
		return
	}

	log.Println("Valid token found. Attempting to update.")

	select {
	case chanValue := <-lock:
		defer func() { lock <- chanValue }()
		handle.fn()
	default:
		log.Debug("Skipped. Another update already running.")
	}

}
