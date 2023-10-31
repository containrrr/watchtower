package middleware

import (
	"github.com/containrrr/watchtower/pkg/api/prelude"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	token = "123123123"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Middleware Suite")
}

var _ = Describe("API", func() {
	requireToken := RequireToken(token)

	Describe("RequireToken middleware", func() {
		It("should return 401 Unauthorized when token is not provided", func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)

			requireToken(testHandler).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			Expect(rec.Body).To(MatchJSON(`{
				"code":  "MISSING_TOKEN",
				"error": "No authentication token was supplied"
			}`))
		})

		It("should return 401 Unauthorized when token is invalid", func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)
			req.Header.Set("Authorization", "Bearer 123")

			requireToken(testHandler).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			Expect(rec.Body).To(MatchJSON(`{
				"code":  "INVALID_TOKEN",
				"error": "The supplied token does not match the configured auth token"
			}`))
		})

		It("should return 200 OK when token is valid", func() {

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/hello", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			requireToken(testHandler).ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
		})
	})
})

func testHandler(_ *prelude.Context) prelude.Response {
	return prelude.OK("Hello!")
}
