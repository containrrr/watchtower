package prelude

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

type HandlerFunc func(c *Context) Response

func (hf HandlerFunc) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", DefaultContentType)
	context := newContext(w, req)

	reqLog := context.Log.WithFields(log.Fields{
		"query": req.URL.RawQuery,
	})
	reqLog.Trace("Received API Request")

	res := hf(context)

	status := res.Status

	bytes, err := res.Bytes()
	if err != nil {
		context.Log.WithError(err).Errorf("Failed to create JSON payload for response")
		bytes = []byte(internalErrorPayload)
		status = http.StatusInternalServerError
		// Reset the content-type in case the handler changed it
		w.Header().Set("Content-Type", DefaultContentType)
	}

	reqLog.WithField("status", status).Trace("Handled API Request")

	w.WriteHeader(status)
	if _, err = w.Write(bytes); err != nil {
		localLog.Errorf("Failed to write HTTP response: %v", err)
	}
}
