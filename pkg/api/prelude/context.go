package prelude

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
)

type Context struct {
	Request *http.Request
	Log     *logrus.Entry
	writer  http.ResponseWriter
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	reqLog := localLog.WithField("endpoint", fmt.Sprintf("%v %v", req.Method, req.URL.Path))
	return &Context{
		Log:     reqLog,
		Request: req,
		writer:  w,
	}
}

func (c *Context) Headers() http.Header {
	return c.writer.Header()
}

type contextWrapper struct {
	context    *Context
	body       bytes.Buffer
	statusCode int
}

func (cw *contextWrapper) Header() http.Header {
	return cw.context.writer.Header()
}

func (cw *contextWrapper) Write(bytes []byte) (int, error) {
	return cw.body.Write(bytes)
}

func (cw *contextWrapper) WriteHeader(statusCode int) {
	cw.statusCode = statusCode
}

func WrapHandler(next http.HandlerFunc) HandlerFunc {
	return func(c *Context) Response {
		wrapper := contextWrapper{
			context: c,
			body:    bytes.Buffer{},
		}

		next(&wrapper, c.Request)

		return Response{
			Status: wrapper.statusCode,
			Body:   wrapper.body.Bytes(),
			Raw:    true,
		}
	}
}
