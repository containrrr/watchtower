package api

import (
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

const TokenMissingMsg = "api token is empty or has not been set. exiting"

type API struct {
	Token string
	hasHandlers bool
}



func New(token string) *API {
	return &API {
		Token: token,
		hasHandlers: false,
	}
}

func (api *API) RegisterFunc(path string, fn http.HandlerFunc) {
	api.hasHandlers = true
	http.HandleFunc(path, fn)
}

func (api *API) RegisterHandler(path string, handler http.Handler) {
	api.hasHandlers = true
	http.Handle(path, handler)
}

func (api *API) Start(block bool) error {

	if !api.hasHandlers {
		log.Debug("Watchtower HTTP API skipped.")
		return nil
	}

	if api.Token == "" {
		log.Fatal(TokenMissingMsg)
	}

	log.Info("Watchtower HTTP API started.")
	if block {
		runHttpServer()
	} else {
		go func() {
			runHttpServer()
		}()
	}
	return nil
}

func runHttpServer() {
	log.Info("Serving HTTP")
	log.Fatal(http.ListenAndServe(":8080", nil))
	os.Exit(0)
}