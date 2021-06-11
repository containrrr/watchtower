package api

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"net/http"
)

const tokenMissingMsg = "api token is empty or has not been set. exiting"

// API is the http server responsible for serving the HTTP API endpoints
type API struct {
	Token           string
	hasHandlers     bool
	Server          *http.Server
	ShutdownContext *context.Context
}

// New is a factory function creating a new API instance
func New(token string) *API {

	api := &API{
		Token:       token,
		hasHandlers: false,
	}
	http.Handle("/v1/prepare-self-update", api.RequireToken(api.handlePrepareUpdate))
	return api
}

func (api *API) handlePrepareUpdate(writer http.ResponseWriter, _ *http.Request) {
	err := api.Server.Shutdown(context.Background())
	if err != http.ErrServerClosed && err != nil {
		writer.WriteHeader(500)
		_, _ = writer.Write([]byte(err.Error()))
	} else {
		writer.WriteHeader(201)
	}
}

// RequireToken is wrapper around http.HandleFunc that checks token validity
func (api *API) RequireToken(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", api.Token) {
			log.Tracef("Invalid token \"%s\"", r.Header.Get("Authorization"))
			log.Tracef("Expected token to be \"%s\"", api.Token)
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
		api.runHTTPServer()
	} else {
		go func() {
			api.runHTTPServer()
		}()
	}
	return nil
}

func (api *API) runHTTPServer() {
	log.Info("Serving HTTP")
	api.Server = &http.Server{Addr: ":8080", Handler: nil}
	if err := api.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
