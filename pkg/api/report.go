package api

import (
	"github.com/containrrr/watchtower/pkg/session"
	"net/http"
)

func ReportEndpoint(reportPtr **session.Report) (path string, handler http.HandlerFunc) {
	path = "/v1/report"
	handler = func(writer http.ResponseWriter, request *http.Request) {
		WriteJsonOrError(writer, *reportPtr)
	}
	return path, handler
}
