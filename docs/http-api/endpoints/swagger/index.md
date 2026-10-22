# Swagger UI

## Overview

The `/swagger/` endpoint serves an interactive, browser-based Swagger UI that renders the Watchtower HTTP API specification.

To enable the Swagger UI, include `swagger` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) (for example `WATCHTOWER_HTTP_API_ENDPOINTS=swagger` or `all`).

## Endpoint

| **Name**  | **Method** | **Endpoint** | **Auth** | **Description**                                                |
|:---------:|:----------:|:------------:|:--------:|:---------------------------------------------------------------|
| Swagger   |   `GET`    | `/swagger/*` |   No     | Interactive API documentation rendered via Swagger UI          |

The endpoint is mounted as a wildcard route (`/swagger/`) so that all Swagger UI assets are served under this prefix.
Accessing `/swagger/index.html` returns the main documentation page and sub-paths serve the static assets the UI needs.

## Security

The Swagger UI page does not require authentication to access.

Calling protected APIs via **Try it out** still requires the [`http-api-token`](../../../configuration/http-api/index.md#http_api_token) (or events token for `/v1/events`) through Swagger UI’s **Authorize** control. That is the same protection as any other client.

The same guidance that applies to the rest of the HTTP API applies here as well:

- Never expose the HTTP API (including the Swagger UI) directly to the Internet — the UI documents every route surface.
- Always use TLS.
- Only enable the endpoints you actually need.

If you place the API behind a reverse proxy (Traefik, Caddy, Nginx, etc.), configure [trusted proxies](../../../configuration/http-api/index.md#http_api_trusted_proxies) and [CORS origins](../../../configuration/http-api/index.md#http_api_cors_origins) accordingly.

## OpenAPI Specification

The UI is powered by a machine-readable [OpenAPI Specification](https://spec.openapis.org/){target="_blank" rel="noopener noreferrer"} (OAS) document that describes every enabled Watchtower endpoint.
This project uses the **Swagger 2.0** format (OAS 2.0), rendered as `swagger.json` and `swagger.yaml`.

!!! Note
    Swagger UI renders the spec as CommonMark in most tooltip and description fields, per the [OAS rich text requirements](https://spec.openapis.org/#rich-text-formatting){target="_blank" rel="noopener noreferrer"}.

## Under the Hood

The Swagger 2.0 specification is generated from annotions in the codebase using [`swaggo/swag`](https://github.com/swaggo/swag){target="_blank" rel="noopener noreferrer"}.

Then, Fiber v3's [`gofiber/contrib/v3/swaggo`](https://docs.gofiber.io/contrib/v3_swaggo_v1.x.x/swaggo/){target="_blank" rel="noopener noreferrer"} middleware package is used to serve the bundled [Swagger UI](https://swagger.io/docs/open-source-tools/swagger-ui/){target="_blank" rel="noopener noreferrer"} assets and generated specification.

The generated files (`docs.go`, `swagger.json`, `swagger.yaml`) can be found in the repository at  [https://github.com/nicholas-fedor/watchtower/tree/main/internal/api/swagger](https://github.com/nicholas-fedor/watchtower/tree/main/internal/api/swagger){target="_blank" rel="noopener noreferrer"}.

## Using the Swagger UI

### Starting the Server

Enable the endpoint and start Watchtower:

=== "Without TLS"

    === "Docker Compose"

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                environment:
                    - WATCHTOWER_HTTP_API_ENDPOINTS=swagger
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                ports:
                    - "8080:8080"
                restart: unless-stopped
        ```

    === "Docker CLI"

        ```bash
        docker run -d \
            --name watchtower \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=swagger \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

    Then open the Swagger UI in a web browser:

    ```url
    http://<host>:8080/swagger/index.html
    ```

=== "With TLS"

    === "Docker Compose"

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                    - /opt/watchtower/certs:/certs:ro
                environment:
                    - WATCHTOWER_HTTP_API_ENDPOINTS=swagger
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                    - WATCHTOWER_HTTP_API_TLS_CERT=/certs/watchtower.crt
                    - WATCHTOWER_HTTP_API_TLS_KEY=/certs/watchtower.key
                ports:
                    - "8080:8080"
                restart: unless-stopped
        ```

    === "Docker CLI"

        ```bash
        docker run -d \
            --name watchtower \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v /opt/watchtower/certs:/certs:ro \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=swagger \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -e WATCHTOWER_HTTP_API_TLS_CERT=/certs/watchtower.crt \
            -e WATCHTOWER_HTTP_API_TLS_KEY=/certs/watchtower.key \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

    Then open the Swagger UI in a web browser:

    ```url
    https://<host>:8080/swagger/index.html
    ```

### Interacting with Authenticated Endpoints

### Try it out (call protected APIs from the UI)

1. Open `http://<host>:<port>/swagger/index.html` (or `https://` when TLS is enabled).
2. Click **Authorize** at the top-right of Swagger UI.
3. Under **BearerAuth**, paste the HTTP API token **only** (do not type the word `Bearer`. Swagger UI places the value in the `Authorization` header).
4. For `/v1/events`, under **EventsToken**, paste the events token only (auth works in Try it out, but the stream itself will not — see below).
5. Click **Authorize**, then **Close**.
6. Use **Try it out** on an operation (except the events stream).

Authorization for Try it out is persisted across page reloads when using Swagger UI’s persist-authorization setting.

Programmatic clients should still use `Authorization: Bearer <token>` (or the events token header/query form); see [Authentication](../../configuration/authentication/index.md).

!!! Warning "Events SSE is not supported in Swagger UI"
    **Try it out** on [`/v1/events`](../events/index.md) will authenticate, then hang on a permanent **Loading** spinner. That is a Swagger UI limitation, not a Watchtower bug.

    Swagger UI uses the browser `fetch` API and waits for a finished HTTP body. An SSE response is an open stream of `text/event-stream` events with no single “complete” body, so the UI never finishes rendering.

    Use one of these instead:

    ```bash
    # curl (keep the connection open with -N)
    curl -N -H "Authorization: Bearer $WATCHTOWER_HTTP_API_EVENTS_TOKEN" \
      "http://localhost:8080/v1/events"
    ```

    ```javascript
    // Browser EventSource (query token; EventSource cannot set Authorization headers)
    const es = new EventSource(
      "http://localhost:8080/v1/events?access_token=" + encodeURIComponent(eventsToken)
    );
    es.onmessage = (e) => console.log(e.data);
    ```

    Full details: [Events](../events/index.md).

### Query parameters

Swagger UI exposes any `@Param` query parameters documented in the spec.

For example, the `/v1/update` endpoint accepts the following documented query parameters:

| Parameter   | Type      | Required | Description                                                        |
|-------------|-----------|----------|--------------------------------------------------------------------|
| `image`     | `string`  | No       | Comma-separated image names or Go regex patterns to filter.        |
| `container` | `string`  | No       | Comma-separated container name patterns to filter.                 |
| `async`     | `boolean` | No       | Run the update asynchronously; returns `202 Accepted` when `true`. |

### `Try it out`

Swagger UI's **Try it out** button issues a real HTTP request from your browser directly to the Watchtower server.
Because Swagger UI is served from the same origin, no CORS preflight is required when both are accessed from the same host and port (the default setup).
When the UI is served from a different origin, ensure the [`http-api-cors-origins`](../../../configuration/http-api/index.md#http_api_cors_origins) configuration option allows the browser origin.

JSON endpoints (`/v1/config`, `/v1/containers`, `/v1/check`, etc.) work well with Try it out.
The events SSE stream does not (see the warning above).

## Reverse Proxy and TLS Termination

When Watchtower runs behind a TLS-terminating reverse proxy (Traefik, Caddy, Nginx, etc.), Swagger UI's **Try it out** generates request URLs from the OpenAPI spec `schemes` field.

The spec `schemes` value is derived from `c.Protocol()`. Fiber v3 only trusts `X-Forwarded-Proto` from a client when that client IP is listed in [`http-api-trusted-proxies`](../../../configuration/http-api/index.md#http_api_trusted_proxies).
If the proxy IP is not trusted, `c.Protocol()` reports `http` even though the external connection uses HTTPS, and Swagger UI will generate `http://` URLs.

To ensure correct scheme detection:

1. Set [`http-api-trusted-proxies`](../../../configuration/http-api/index.md#http_api_trusted_proxies) to include the proxy IP or CIDR range.
2. Ensure [`http-api-proxy-header`](../../../configuration/http-api/index.md#http_api_proxy_header) matches the header your proxy uses (default: `X-Forwarded-For`).

This is the same trusted-proxies configuration used for client IP detection and rate limiting, so it is not specific to Swagger.

## Troubleshooting

The `/swagger/*` endpoint and Swagger UI return standard HTTP status codes.

Common issues and their resolutions:

| HTTP Status | Cause                                                                 | Resolution                                                                                                                          |
|:-----------:|:----------------------------------------------------------------------|:------------------------------------------------------------------------------------------------------------------------------------|
|    `404`    | The endpoint may not be enabled                                       | Include `swagger` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints). |
|    `404`    | Requested a path outside `/swagger/*`, e.g. `/swagger` (no wildcard). | Navigate to `/swagger/index.html`.                                                                                                  |
|    `405`    | HTTP method other than `GET` used against `/swagger/*`.               | Use `GET` only. `POST`, `PUT`, and `DELETE` are not supported on this path.                                                         |

The server startup log confirms successful registration:

```text
2026/07/06 20:00:00 [INFO]  HTTP API listening on :8080
```

## References

- [OpenAPI Specification 3.1.1](https://spec.openapis.org/){target="_blank" rel="noopener noreferrer"} — the normative specification that defines the structure OAS documents (Swagger 2.0 is a prior version of the same standard; see [OAS Learn](https://learn.openapis.org/))
- [Swagger UI — official tooling](https://swagger.io/tools/swagger-ui/){target="_blank" rel="noopener noreferrer"} — the open-source UI that renders the interactive documentation
- [`gofiber/contrib/v3/swaggo`](https://docs.gofiber.io/contrib/v3_swaggo_v1.x.x/swaggo/){target="_blank" rel="noopener noreferrer"} — Fiber v3 integration middleware that serves the UI and spec
- [`swaggo/swag`](https://github.com/swaggo/swag){target="_blank" rel="noopener noreferrer"} — the Go annotation parser and code generator that produces the OpenAPI 2.0 spec from handler comments
