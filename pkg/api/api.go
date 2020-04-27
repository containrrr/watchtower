package api

import (
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

const tokenMissingMsg = "api token is empty or has not been set. exiting"

// API is the http server responsible for serving the HTTP API endpoints
type API struct {
	Token       string
	hasHandlers bool
}

// SetupHTTPUpdates configures the endpoint needed for triggering updates via http
func SetupHTTPUpdates(apiToken string, updateFunction func()) error {
	if apiToken == "" {
		return errors.New("api token is empty or has not been set. not starting api")
	}


func New(token string) *API {
	return &API{
		Token:       token,
		hasHandlers: false,
	}
}

// RegisterFunc is a wrapper around http.HandleFunc that also flips the bool used to determine whether to launch the API
func (api *API) RegisterFunc(path string, fn http.HandlerFunc) {
	api.hasHandlers = true
	http.HandleFunc(path, fn)
}

// RegisterHandler is a wrapper around http.Handler that also flips the bool used to determine whether to launch the API
func (api *API) RegisterHandler(path string, handler http.Handler) {
	api.hasHandlers = true
	http.Handle(path, handler)
}

// Start the API and serve over HTTP. Requires an API Token to be set.
func (api *API) Start(block bool) error {

	if !api.hasHandlers {
		log.Debug("Watchtower HTTP API skipped.")
		return nil
	}

	if api.Token == "" {
		log.Fatal(tokenMissingMsg)
	}

	log.Info("Watchtower HTTP API started.")
	if block {
		runHTTPServer()
	} else {
		go func() {
			runHTTPServer()
		}()
	}
	return nil
}

func runHTTPServer() {
	log.Info("Serving HTTP")
	log.Fatal(http.ListenAndServe(":8080", nil))
	os.Exit(0)
}
