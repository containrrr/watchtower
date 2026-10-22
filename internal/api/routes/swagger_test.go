package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/nicholas-fedor/watchtower/internal/api/config"
	_ "github.com/nicholas-fedor/watchtower/internal/api/swagger"
)

func TestRewriteSwaggerDocHost(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod(fiber.MethodGet)
	c.Request().SetRequestURI("http://api.example.com:9090/swagger/doc.json")
	c.Request().Header.SetHost("api.example.com:9090")

	doc := `{"swagger":"2.0","host":"localhost:8080","schemes":["http"],"paths":{}}`
	out, err := rewriteSwaggerDocHost(doc, c)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &spec))
	assert.Equal(t, "api.example.com:9090", spec["host"])
	schemes, ok := spec["schemes"].([]any)
	require.True(t, ok)
	require.Len(t, schemes, 1)
	assert.Equal(t, "http", schemes[0])
}

func TestRewriteSwaggerDocHost_UntrustedForwardedProtoIgnored(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	c.Request().Header.SetMethod(fiber.MethodGet)
	c.Request().SetRequestURI("http://secure.example.com/swagger/doc.json")
	c.Request().Header.SetHost("secure.example.com")
	// Without TrustedProxies, Fiber does not treat this as a trusted protocol.
	c.Request().Header.Set("X-Forwarded-Proto", "https")

	doc := `{"swagger":"2.0","host":"localhost:8080","schemes":["http"]}`
	out, err := rewriteSwaggerDocHost(doc, c)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &spec))
	assert.Equal(t, "secure.example.com", spec["host"])
	schemes := spec["schemes"].([]any)
	// Scheme follows c.Protocol() only; untrusted X-Forwarded-Proto is ignored.
	assert.Equal(t, "http", schemes[0])
}

func TestRegisterSwaggerRoute_PublicAccess(t *testing.T) {
	app := testApp()

	opts := config.Options{EnableSwaggerAPI: true}
	registerSwaggerRoute(app, opts)

	// Unauthenticated UI load → 200 (docs are public; /v1/* still auth-gated)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/swagger/index.html", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRegisterSwaggerRoute_DocJSONRewritesHost(t *testing.T) {
	app := testApp()

	opts := config.Options{EnableSwaggerAPI: true}
	registerSwaggerRoute(app, opts)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"http://watchtower.local:8443/swagger/doc.json",
		nil,
	)
	req.Host = "watchtower.local:8443"

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	_ = resp.Body.Close()

	var spec map[string]any
	require.NoError(t, json.Unmarshal(body, &spec))
	assert.Equal(t, "watchtower.local:8443", spec["host"])
	assert.Contains(t, spec, "securityDefinitions")
}
