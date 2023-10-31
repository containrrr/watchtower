package prelude

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Response struct {
	Body   any
	Status int
	Raw    bool
}

func (r *Response) Bytes() ([]byte, error) {
	if bytes, raw := r.Body.([]byte); raw {
		return bytes, nil
	}

	if str, raw := r.Body.(string); raw {
		return []byte(str), nil
	}

	return json.MarshalIndent(r.Body, "", "  ")
}

var localLog = log.WithField("notify", "no")

func OK(body any) Response {
	return Response{
		Status: http.StatusOK,
		Body:   body,
	}
}

func Error(err errorResponse) Response {
	return Response{
		Status: err.Status,
		Body:   err,
	}
}
