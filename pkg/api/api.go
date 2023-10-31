package api

import (
	"context"
	"errors"
	"github.com/containrrr/watchtower/pkg/api/metrics"
	"github.com/containrrr/watchtower/pkg/api/middleware"
	"github.com/containrrr/watchtower/pkg/api/prelude"
	"github.com/containrrr/watchtower/pkg/api/updates"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"
)

const tokenMissingMsg = "api token is empty or has not been set. exiting"

// API is the http server responsible for serving the HTTP API endpoints
type API struct {
	Token          string
	hasHandlers    bool
	mux            *http.ServeMux
	server         *http.Server
	running        *sync.Mutex
	router         router
	authMiddleware prelude.Middleware
	registered     bool
}

// New is a factory function creating a new API instance
func New(token string) *API {
	return &API{
		Token:          token,
		hasHandlers:    false,
		mux:            http.NewServeMux(),
		running:        &sync.Mutex{},
		router:         router{},
		authMiddleware: middleware.RequireToken(token),
		registered:     false,
	}
}

func (api *API) route(route string) methodHandlers {
	return api.router.route(route)
}

func (api *API) registerHandlers() {
	if api.registered {
		return
	}
	for path, route := range api.router {
		if len(route) < 1 {
			continue
		}
		api.hasHandlers = true
		api.mux.Handle(path, api.authMiddleware(route.Handler))
	}
	api.registered = true
	return
}

// Start the API and serve over HTTP. Requires an API Token to be set.
func (api *API) Start() error {

	api.registerHandlers()

	if !api.hasHandlers {
		log.Debug("Watchtower HTTP API skipped.")
		return nil
	}

	if api.Token == "" {
		log.Fatal(tokenMissingMsg)
	}

	api.running.Lock()
	go func() {
		defer api.running.Unlock()
		api.server = &http.Server{
			Addr:    ":8080",
			Handler: api.mux,
		}

		if err := api.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("HTTP Server error: %v", err)
		}
	}()

	return nil
}

// Stop tells the api server to shut down (if its running) and returns a sync.Mutex that is locked
// until the server has handled all remaining requests and shut down
func (api *API) Stop() *sync.Mutex {

	if api.server != nil {
		go func() {
			if err := api.server.Shutdown(context.Background()); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Errorf("Error stopping HTTP Server: %v", err)
			}
		}()
	}

	return api.running
}

// Handler is used to get a http.Handler for testing
func (api *API) Handler() http.Handler {
	api.registerHandlers()
	return api.mux
}

// EnableUpdates registers the `updates` endpoints
func (api *API) EnableUpdates(f updates.InvokedFunc, updateLock *sync.Mutex) {
	api.route("/v1/updates").post(updates.PostV1(f, updateLock))
	api.route("/v2/updates/apply").post(updates.PostV2Apply(f, updateLock))
	api.route("/v2/updates/check").post(updates.PostV2Check(f, updateLock))
}

// EnableMetrics registers the `metrics` endpoints
func (api *API) EnableMetrics() {
	api.route("/v1/metrics").get(metrics.GetV1())
}
