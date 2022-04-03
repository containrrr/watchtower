package api

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const tokenMissingMsg = "api token is empty or has not been set. exiting"

// DefaultPort is the default port used for the http server
const DefaultPort = 8080

// API is the http server responsible for serving the HTTP API endpoints
type API struct {
	Token       string
	hasHandlers bool
}

// New is a factory function creating a new API instance
func New(token string) *API {
	return &API{
		Token:       token,
		hasHandlers: false,
	}
}

// RequireToken is wrapper around http.HandleFunc that checks token validity
func (api *API) RequireToken(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := fmt.Sprintf("Bearer %s", api.Token)
		if auth != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		log.Debug("Valid token found.")
		fn(w, r)
	}
}

// RegisterFunc is a wrapper around http.HandleFunc that also sets the flag used to determine whether to launch the API
func (api *API) RegisterFunc(path string, fn http.HandlerFunc) {
	api.hasHandlers = true
	http.HandleFunc(path, api.RequireToken(fn))
}

// RegisterHandler is a wrapper around http.Handler that also sets the flag used to determine whether to launch the API
func (api *API) RegisterHandler(path string, handler http.Handler) {
	api.hasHandlers = true
	http.Handle(path, api.RequireToken(handler.ServeHTTP))
}

// Start the API and serve over HTTP. Requires an API Token to be set.
func (api *API) Start(block bool, port int) error {

	if !api.hasHandlers {
		log.Debug("Watchtower HTTP API skipped.")
		return nil
	}

	if api.Token == "" {
		log.Fatal(tokenMissingMsg)
	}

	if block {
		runHTTPServer(port)
	} else {
		go func() {
			runHTTPServer(port)
		}()
	}
	return nil
}

func runHTTPServer(port int) {
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
