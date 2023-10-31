package api

import (
	. "github.com/containrrr/watchtower/pkg/api/prelude"
	"net/http"
)

type router map[string]methodHandlers

type methodHandlers map[string]HandlerFunc

func (mh methodHandlers) Handler(c *Context) Response {
	handler, found := mh[c.Request.Method]
	if !found {
		return Error(ErrNotFound)
	}
	return handler(c)
}

func (mh methodHandlers) post(handlerFunc HandlerFunc) {
	mh[http.MethodPost] = handlerFunc
}
func (mh methodHandlers) get(handlerFunc HandlerFunc) {
	mh[http.MethodGet] = handlerFunc
}

func (r router) route(route string) methodHandlers {
	routeMethods, found := r[route]
	if !found {
		routeMethods = methodHandlers{}
		r[route] = routeMethods
	}
	return routeMethods
}
