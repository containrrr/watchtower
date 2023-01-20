// Package delayhttp creates http.HandlerFunc's that delays the response.
// Useful for testing timeout scenarios.
package delayhttp

import (
	"net/http"
	"time"
)

// WithChannel returns a handler that delays until it recieves something on returnChan
func WithChannel(returnChan chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Wait until channel sends return code
		<-returnChan
	}
}

// WithCancel returns a handler that delays until the cancel func is called.
// Useful together with defer to clean up tests.
func WithCancel() (http.HandlerFunc, func()) {
	returnChan := make(chan struct{}, 1)
	return WithChannel(returnChan), func() {
		returnChan <- struct{}{}
	}
}

// WithTimeout returns a handler that delays until the passed duration has elapsed
func WithTimeout(delay time.Duration) http.HandlerFunc {
	returnChan := make(chan struct{}, 1)
	go func() {
		time.Sleep(delay)
		returnChan <- struct{}{}
	}()
	return WithChannel(returnChan)
}
