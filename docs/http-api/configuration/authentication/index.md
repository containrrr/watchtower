# Authentication

## Overview

Watchtower's HTTP API uses token-based authentication to protect sensitive endpoints.

Two separate tokens are available:

- **[HTTP API Token](../../../configuration/http-api/index.md#http_api_token)**: Used for most authenticated endpoints.
- **[HTTP API Events Token](../../../configuration/http-api/index.md#http_api_events_token)**: Used exclusively for the [`/v1/events`](../../endpoints/events/index.md) endpoint.

Authentication is enforced at the route level after other middleware (rate limiting, CORS, etc.).

## HTTP API Token

Use the [HTTP API Token](../../../configuration/http-api/index.md#http_api_token) configuration option to set the primary authentication token.

This token is required when any of the following endpoints are enabled:

- [`/v1/check`](../../endpoints/check/index.md)
- [`/v1/config`](../../endpoints/config/index.md)
- [`/v1/containers`](../../endpoints/containers/index.md)
- [`/v1/containers/details`](../../endpoints/container-details/index.md)
- [`/v1/history`](../../endpoints/history/index.md)
- [`/v1/images`](../../endpoints/images/index.md)
- [`/v1/metrics`](../../endpoints/metrics/index.md)
- [`/v1/status`](../../endpoints/status/index.md)
- [`/v1/update`](../../endpoints/update/index.md)

Clients must include the token using the `Authorization: Bearer <token>` header:

```bash
curl -H "Authorization: Bearer your-secure-token" http://localhost:8080/v1/metrics
```

Invalid or missing tokens result in `401 Unauthorized`.
Failed authentication attempts are logged with the client IP address.

A cookie-based fallback is also supported for clients that cannot set custom headers.
The token may be provided in a cookie named `access_token`:

```http
Cookie: access_token=your-secure-token
```

!!! Note
    The cookie fallback exists primarily for browser-based or limited clients.
    The header `Authorization: Bearer` is the recommended method.

!!! Warning "Cookie auth and CSRF"
    When the [`http-api-cors-origins`](../../../configuration/http-api/index.md#http_api_cors_origins) is configured and credentials are allowed (for example, `Access-Control-Allow-Credentials: true`), the cookie fallback is vulnerable to cross-site request forgery (CSRF).
    A browser on an attacker-controlled origin can issue authenticated POST requests to the API without the user's knowledge.
    Prefer header-based `Authorization: Bearer` auth.

## HTTP API Events Token

The [`/v1/events`](../../endpoints/events/index.md) endpoint uses a separate token set via the [HTTP API Events Token](../../../configuration/http-api/index.md#http_api_events_token) configuration option.

This token is **required** when the [`/v1/events`](../../endpoints/events/index.md) endpoint is enabled.

The events token can be supplied in two ways:

- `Authorization: Bearer` header (recommended for most clients)
- `access_token` query parameter (required for browser `EventSource`, which cannot set custom headers)

Example using the header:

```bash
curl -N -H "Authorization: Bearer your-events-token" http://localhost:8080/v1/events
```

Example using the query parameter:

```bash
curl -N "http://localhost:8080/v1/events?access_token=your-events-token"
```

In JavaScript (for browsers):

```javascript
const eventSource = new EventSource('http://localhost:8080/v1/events?access_token=your-events-token');
```

!!! Important
    The events token is intentionally separate from the main API token. Query parameters can appear in access logs, browser history, and proxy logs. Using a dedicated token limits exposure.

See the [Events endpoint documentation](../../endpoints/events/index.md) for more details.

## Examples

Tokens can be provided to Watchtower using Docker Secrets, environment variables, or CLI flags.

=== "Docker Compose"

    === "Docker Secrets"

        Use Compose secrets to mount the token file inside the container and point the environment variable at the mounted path (commonly `/run/secrets/<name>`).

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                secrets:
                    - http_api_token
                environment:
                    - WATCHTOWER_HTTP_API_TOKEN=/run/secrets/http_api_token
                    # Enable an endpoint that requires authentication
                    - WATCHTOWER_HTTP_API_ENDPOINTS=metrics
                ports:
                    - "8080:8080"
                restart: unless-stopped

        secrets:
            http_api_token:
                file: ./secrets/http_api_token.txt
        ```

    === "Environment  Variables"

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                environment:
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                    # Enable an endpoint that requires authentication
                    - WATCHTOWER_HTTP_API_ENDPOINTS=metrics
                ports:
                    - "8080:8080"
                restart: unless-stopped
        ```

=== "Docker CLI"

    === "Environment Variables"

        ```bash
        docker run -d \
            --name watchtower \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=metrics \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

    === "CLI Flags"

        ```bash
        docker run -d \
            --name watchtower \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower \
            --http-api-token=your-secure-token \
            --http-api-endpoints=metrics
        ```

## Unauthenticated Endpoints

The following endpoints do not require authentication when enabled:

- [Health probes](../../endpoints/health/index.md): `/livez`, `/readyz`, `/startupz`
- [Swagger UI](../../endpoints/swagger/index.md): `/swagger/*` ("Try it out" functionality still requires authorization)

## Best Practices

- Generate strong, random tokens (for example: `openssl rand -base64 32`).
- Use a separate events token when enabling the [`/v1/events`](../../endpoints/events/index.md) endpoint.
- Always run the HTTP API behind TLS in production (see the [TLS documentation](../tls/index.md)).
- **Never** expose the HTTP API directly to the public internet.
- Store tokens using Docker Secrets or a secrets manager rather than plain environment variables or CLI flags when possible.
- Rotate tokens if they may have been exposed.
- Combine authentication with network controls (firewalls, reverse proxies with IP allowlists).

## Related Documentation

- [HTTP API Configuration](../../../configuration/http-api/index.md)
- [HTTP API TLS](../tls/index.md)
- [HTTP API Host and Port](../host-and-port/index.md)
- [Events Endpoint](../../endpoints/events/index.md)
- [Docker Secrets](../../../getting-started/docker-secrets/index.md)
- [HTTP API Overview](../../overview/index.md#security_considerations)
