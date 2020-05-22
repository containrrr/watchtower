package api

import (
	"errors"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	lock chan bool
)

func init() {
	lock = make(chan bool, 1)
	lock <- true
}

// SetupHTTPUpdates configures the endpoint needed for triggering updates via http
func SetupHTTPUpdates(apiToken string, updateFunction func()) error {
	if apiToken == "" {
		return errors.New("api token is empty or has not been set. not starting api")
	}

	log.Println("Watchtower HTTP API started.")

	http.HandleFunc("/v1/update", func(w http.ResponseWriter, r *http.Request) {
		log.Info("Updates triggered by HTTP API request.")

		_, err := io.Copy(os.Stdout, r.Body)
		if err != nil {
			log.Println(err)
			return
		}

		if r.Header.Get("Token") != apiToken {
			log.Println("Invalid token. Not updating.")
			return
		}

		log.Println("Valid token found. Attempting to update.")

		select {
		case chanValue := <-lock:
			defer func() { lock <- chanValue }()
			updateFunction()
		default:
			log.Debug("Skipped. Another update already running.")
		}

	})

	return nil
}

// WaitForHTTPUpdates starts the http server and listens for requests.
func WaitForHTTPUpdates() error {
	log.Fatal(http.ListenAndServe(":8080", nil))
	os.Exit(0)
	return nil
}
