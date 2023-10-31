package prelude

import "net/http"

type errorResponse struct {
	Error  string    `json:"error"`
	Code   ErrorCode `json:"code"`
	Status int       `json:"-"`
}

const internalErrorPayload string = `{ "error": "API internal error, check logs", "code": "API_INTERNAL_ERROR" }`

type ErrorCode string

var (
	ErrUpdateRunning = errorResponse{
		Code:   "UPDATE_RUNNING",
		Error:  "Update already running",
		Status: http.StatusConflict,
	}
	ErrNotFound = errorResponse{
		Code:   "NOT_FOUND",
		Error:  "Endpoint is not registered to a handler",
		Status: http.StatusNotFound,
	}
	ErrInvalidToken = errorResponse{
		Code:   "INVALID_TOKEN",
		Error:  "The supplied token does not match the configured auth token",
		Status: http.StatusUnauthorized,
	}
	ErrMissingToken = errorResponse{
		Code:   "MISSING_TOKEN",
		Error:  "No authentication token was supplied",
		Status: http.StatusUnauthorized,
	}
)
