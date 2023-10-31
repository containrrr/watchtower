package middleware

import (
	"fmt"
	. "github.com/containrrr/watchtower/pkg/api/prelude"
)

// RequireToken returns a prelude.Middleware that checks token validity
func RequireToken(token string) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		want := fmt.Sprintf("Bearer %s", token)
		return func(c *Context) Response {
			auth := c.Request.Header.Get("Authorization")
			if auth == "" {
				return Error(ErrMissingToken)
			}

			if auth != want {
				return Error(ErrInvalidToken)
			}
			return next(c)
		}
	}
}
