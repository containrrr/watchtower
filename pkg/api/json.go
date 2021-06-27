package api

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"net/http"
)

// WriteJsonOrError writes the supplied response to the http.ResponseWriter, handling any errors by logging and
// returning an Internal Server Error response (status 500)
func WriteJsonOrError(writer http.ResponseWriter, response interface{}) {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		logrus.WithError(err).Error("failed to create json payload")
		writer.WriteHeader(500)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, err = writer.Write(data)
	if err != nil {
		logrus.WithError(err).Error("failed to write response")
	}
}
