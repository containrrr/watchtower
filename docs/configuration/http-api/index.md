# HTTP API

## Deprecation Notice

!!! Warning "Watchtower v2 Legacy HTTP API Configuration Deprecation"
    Endpoint-specific configuration options will be removed with the release of Watchtower v2.

    Use the [`HTTP API Endpoints`](#http_api_endpoints) configuration option instead.

## HTTP API Host

Sets the host interface for binding the HTTP API.

```text
            Argument: --http-api-host
Environment Variable: WATCHTOWER_HTTP_API_HOST
                Type: String
             Default: empty (binds to all IPv4 interfaces)
```

!!! Note "See the [HTTP API Host documentation](../../http-api/configuration/host-and-port/index.md#http_api_host) for details"

## HTTP API Port

Sets the listening port for the HTTP API.

```text
            Argument: --http-api-port
Environment Variable: WATCHTOWER_HTTP_API_PORT
                Type: String
             Default: 8080
```

!!! Note "See the [HTTP API Port documentation](../../http-api/configuration/host-and-port/index.md#http_api_port) for details"

## HTTP API Token

Sets an authentication token for HTTP API requests.

```text
            Argument: --http-api-token
Environment Variable: WATCHTOWER_HTTP_API_TOKEN
                Type: String
             Default: None
```

!!! Note
    Supports file path for Docker Secrets (e.g., `/run/secrets/http_api_token`).

## HTTP API Events Token

Sets an authentication token with read-only permissions specifically for the events SSE endpoint (`/v1/events`).

```text
            Argument: --http-api-events-token
Environment Variable: WATCHTOWER_HTTP_API_EVENTS_TOKEN
                Type: String
             Default: None
```

This is a separate token from the [`http-api-token`](#http_api_token) in order to reduce credential exposure, because events tokens may be exposed in URL query parameters.

The token can be provided via the `Authorization` header (for programmatic clients) or as the `access_token` query parameter (for browser `EventSource` which cannot set custom headers).

!!! Warning "Events token leakage risk"
    When using the `access_token` query parameter (required for browser `EventSource`), the token appears in browser history, proxy logs, and access logs.
    This is a lower exposure than the main API token, but you should still treat the events token as a secret and rotate it if it has been exposed.
    Prefer header-based auth when possible.

!!! Important "This is **required** when the `events` endpoint is enabled via [`http-api-endpoints`](#http_api_endpoints)."

!!! Note
    Supports file path for Docker Secrets (e.g., `/run/secrets/http_api_events_token`).

## HTTP API Endpoints

Selects which HTTP API endpoints to enable.

```text
            Argument: --http-api-endpoints
Environment Variable: WATCHTOWER_HTTP_API_ENDPOINTS
                Type: String (comma or space separated list)
             Default: empty (HTTP API disabled)
```

!!! Note "For more information, review the [HTTP API](../../http-api/overview/index.md) documentation"

Valid names (case-insensitive):

| Name                                                         | Routes                                                                                                | Auth                                              |
|--------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|---------------------------------------------------|
| [`health`](../../http-api/endpoints/health/index.md)         | `/livez`, `/readyz`, `/startupz`                                                                      | None                                              |
| [`update`](../../http-api/endpoints/update/index.md)         | `POST /v1/update`                                                                                     | [`http-api-token`](#http_api_token)               |
| [`metrics`](../../http-api/endpoints/metrics/index.md)       | `GET /v1/metrics`, [`GET /v1/status`](../../http-api/endpoints/status/index.md)                       | [`http-api-token`](#http_api_token)               |
| [`containers`](../../http-api/endpoints/containers/index.md) | `GET /v1/containers`, [`/v1/containers/details`](../../http-api/endpoints/container-details/index.md) | [`http-api-token`](#http_api_token)               |
| [`check`](../../http-api/endpoints/check/index.md)           | `POST /v1/check`                                                                                      | [`http-api-token`](#http_api_token)               |
| [`history`](../../http-api/endpoints/history/index.md)       | `GET /v1/history`                                                                                     | [`http-api-token`](#http_api_token)               |
| [`images`](../../http-api/endpoints/images/index.md)         | `GET /v1/images`                                                                                      | [`http-api-token`](#http_api_token)               |
| [`config`](../../http-api/endpoints/config/index.md)         | `GET /v1/config`                                                                                      | [`http-api-token`](#http_api_token)               |
| [`events`](../../http-api/endpoints/events/index.md)         | `GET /v1/events`                                                                                      | [`http-api-events-token`](#http_api_events_token) |
| [`swagger`](../../http-api/endpoints/swagger/index.md)       | `GET /swagger/*`                                                                                      | None                                              |

!!! Warning
    - Protected `/v1/*` endpoints require configuring a [`http-api-token`](#http_api_token).
    - Enabling `events` requires configuring a [`http-api-events-token`](#http_api_events_token).
    - The `all` value enables every endpoint and **MUST** be the only value.
    - Watchtower will exit on startup if an incorrect name is used or combined with `all`.

!!! Note "Flag and environment value usage"
    - The CLI flag can be specified multiple times.
    - Defining the environment variable multiple times will not work (only the last value is used).
    - For environment variables use comma or space-separated values.

## HTTP API Rate Limit

Sets the maximum number of API requests allowed per minute per IP address.

```text
            Argument: --http-api-rate-limit
Environment Variable: WATCHTOWER_HTTP_API_RATE_LIMIT
                Type: Integer
             Default: 60
```

!!! Note
    When the limit is exceeded, the client receives HTTP 429 (Too Many Requests).

## HTTP API Check Timeout

Sets the maximum duration for the `/v1/check` API endpoint.

```text
            Argument: --http-api-check-timeout
Environment Variable: WATCHTOWER_HTTP_API_CHECK_TIMEOUT
                Type: Duration
             Default: 5m
```

!!! Note
    The `/v1/check` endpoint queries registries for the latest image digest.
    Increase this timeout if you see `context deadline exceeded` errors during checks.

## HTTP API Update Timeout

Sets the maximum duration for the `/v1/update` API endpoint.

```text
            Argument: --http-api-update-timeout
Environment Variable: WATCHTOWER_HTTP_API_UPDATE_TIMEOUT
                Type: Duration
             Default: 10m
```

!!! Note
    The `/v1/update` endpoint performs a full container update scan.
    Increase this timeout if you see `context deadline exceeded` errors during updates.

## HTTP API TLS Certificate

Path to the TLS certificate file for the HTTP API.

```text
            Argument: --http-api-tls-cert
Environment Variable: WATCHTOWER_HTTP_API_TLS_CERT
                Type: String
             Default: empty (HTTP)
```

!!! Important
    When both the [`http-api-tls-cert`](#http_api_tls_certificate) and [`http-api-tls-key`](#http_api_tls_key) are provided, the server uses HTTPS.

## HTTP API TLS Key

Path to the TLS private key file for the HTTP API.

```text
            Argument: --http-api-tls-key
Environment Variable: WATCHTOWER_HTTP_API_TLS_KEY
                Type: String
             Default: empty (HTTP)
```

!!! Important
    When both the [`http-api-tls-cert`](#http_api_tls_certificate) and [`http-api-tls-key`](#http_api_tls_key) are provided, the server uses HTTPS.

!!! Note "See the [HTTP API Host documentation](../../http-api/configuration/tls/index.md) for details"

## HTTP API Trusted Proxies

Comma-separated list of trusted proxy IP addresses or CIDR ranges for reverse proxy support.
When set, enables proxy header processing for client IP and scheme detection.

```text
            Argument: --http-api-trusted-proxies
Environment Variable: WATCHTOWER_HTTP_API_TRUSTED_PROXIES
                Type: String Array
             Default: (unset)
```

!!! Important
    Required for correct client IP detection and rate limiting behind a reverse proxy (Traefik, Caddy, Nginx, etc.).

## HTTP API Proxy Header

Header to read the real client IP from when behind a reverse proxy.

```text
            Argument: --http-api-proxy-header
Environment Variable: WATCHTOWER_HTTP_API_PROXY_HEADER
                Type: String
             Default: X-Forwarded-For
```

!!! Note
    Only used when [`http-api-trusted-proxies`](#http_api_trusted_proxies) is set.

Common values:

- `X-Forwarded-For` (Traefik, Caddy)
- `X-Real-IP` (Nginx)
- `CF-Connecting-IP` (Cloudflare)

## HTTP API CORS Origins

Comma-separated list of allowed CORS origins for cross-origin requests.
Use this to restrict which origins can access the API.

```text
            Argument: --http-api-cors-origins
Environment Variable: WATCHTOWER_HTTP_API_CORS_ORIGINS
                Type: String Array
             Default: Unset (same-origin only)
```

!!! Note
    When unset, CORS is disabled and only same-origin requests are allowed.
    Set this to permit specific cross-origin origins.

## Deprecated Configuration Options

/// details | The following legacy configuration options are deprecated and will be removed with the release of Watchtower v2.
    type: warning

### HTTP API Containers

Runs Watchtower with the read-only containers API enabled.

```text
            Argument: --http-api-containers
Environment Variable: WATCHTOWER_HTTP_API_CONTAINERS
                Type: Boolean
             Default: false
```

!!! Deprecated
    Add `containers` to the  [`http-api-endpoints`](#http_api_endpoints) configuration instead.

    For example: `WATCHTOWER_HTTP_API_ENDPOINTS=containers`

### HTTP API Metrics

Runs Watchtower with the Prometheus metrics API enabled.

```text
            Argument: --http-api-metrics
Environment Variable: WATCHTOWER_HTTP_API_METRICS
                Type: Boolean
             Default: false
```

!!! Deprecated
    Add `metrics` to the  [`http-api-endpoints`](#http_api_endpoints) configuration instead.

    For example: `WATCHTOWER_HTTP_API_ENDPOINTS=metrics`

### HTTP API Update

Runs Watchtower in HTTP API mode, so that image updates must be triggered by a request.

```text
            Argument: --http-api-update
Environment Variable: WATCHTOWER_HTTP_API_UPDATE
                Type: Boolean
             Default: false
```

!!! Deprecated
    Add `update` to the  [`http-api-endpoints`](#http_api_endpoints) configuration instead.

    For example: `WATCHTOWER_HTTP_API_ENDPOINTS=update`

///
