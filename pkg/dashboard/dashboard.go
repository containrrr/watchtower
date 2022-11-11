package dashboard

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Dashboard is the http server responsible for serving the static Dashboard files
type Dashboard struct {
}

// New is a factory function creating a new Dashboard instance
func New() *Dashboard {
	return &Dashboard{}
}

// Start the Dashboard and serve over HTTP
func (dashboard *Dashboard) Start() error {
	go func() {
		runHTTPServer()
	}()
	return nil
}

func runHTTPServer() {
	serveMux := http.NewServeMux()
	serveMux.Handle("/", getHandler())

	log.Debug("Starting http dashboard server")
	log.Fatal(http.ListenAndServe(":8001", serveMux))
}

func getHandler() http.Handler {
	return http.FileServer(http.Dir("./web/static"))
}
