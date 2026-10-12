# Update

## Overview

Include `update` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this mode.

## Configuration

| Setting | Flag | Environment Variable | Default |
|:--------|:-----|:---------------------|:--------|
| Update API timeout | [`--http-api-update-timeout`](../../../configuration/http-api/index.md#http_api_update_timeout) | `WATCHTOWER_HTTP_API_UPDATE_TIMEOUT` | `10m` |

## Parameters

### Image Name

Watchtower supports using the `image` URL query parameter to filter updates for only certain images.

#### No Image Filtering

The following `curl` command would trigger an update of all container images monitored by Watchtower:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update"
```

#### Image Filtering with Tags

You can specify image tags to target containers running a specific version (e.g., `foo/bar:1.0`).

For example, to update only containers using `foo/bar:1.0` and `foo/baz:latest`:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?image=foo/bar:1.0,foo/baz:latest"
```

#### Image Filtering without Tags

If no tag is provided, Watchtower matches containers regardless of their tag.

The following `curl` command would trigger an update for the images `foo/bar` and `foo/baz`:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?image=foo/bar,foo/baz"
```

### Container Name

Watchtower supports using the `container` URL query parameter to filter updates for only certain containers by name.

#### Container Name Patterns

You can specify exact container names or Go regex patterns to match containers.

For example, to update only containers with names starting with `web-`:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?container=^web-.*"
```

#### Multiple Container Patterns

Use commas to specify multiple patterns:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?container=^web-.*,^api-.*"
```

#### Regex Pattern Matching

Container patterns support Go regex syntax.
Invalid regex patterns are treated as literal strings for exact matching.

| Pattern       | Matches                          |
|:--------------|:---------------------------------|
| `mycontainer` | Exact match for `mycontainer`    |
| `^web-.*`     | Any name starting with `web-`    |
| `.*-prod$`    | Any name ending with `-prod`     |

### Timeout

The `timeout` parameter overrides the per-request timeout for this update.
It accepts Go durations such as `30s`, `2m`, or `10m`.
The value is capped by the configured update API timeout (`--http-api-update-timeout` / `WATCHTOWER_HTTP_API_UPDATE_TIMEOUT`, default `10m`).

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?timeout=15m"
```

!!! Note
    For synchronous updates, this bounds the time the HTTP response is held. For asynchronous updates (`?async=true`), this bounds how long the background update runs before being cancelled.

### Asynchronous Updates

The `/v1/update` endpoint supports an `async` query parameter to trigger updates without waiting for completion. This is useful for CI environments or automation that needs to fire-and-forget without maintaining a long-lived connection.

#### Asynchronous Update Trigger

Adding the `?async=true` parameter to a POST request causes the handler to spawn the update in a background goroutine and return immediately with HTTP 202 Accepted.

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?async=true"
```

Response:

```http
HTTP/1.1 202 Accepted
Content-Type: application/json
```

Equivalent example for a targeted async update:

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update?image=foo/bar:latest&async=true"
```

The same concurrency behavior applies to async requests: full updates return 429 if another update is already in progress, while targeted updates block until the lock is available before spawning the async goroutine.

#### Status Codes for Async Requests

| Status Code | Description                                                                                                                                 |
|:-----------:|:--------------------------------------------------------------------------------------------------------------------------------------------|
|     202     | Update triggered successfully and running asynchronously                                                                                    |
|     401     | Invalid or missing authentication token                                                                                                     |
|     429     | Another update is already in progress (full updates only) or the request was rate limited                                                   |
|     500     | Internal server error during request processing                                                                                             |
|     503     | Update handler timed out (exceeded configured limit, default 10m), or client cancelled while waiting on update lock (targeted updates only) |

The following example shows what happens when a full update is requested while another update is already running:

```bash
curl -i -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/update"
```

Response:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 30

{
  "error": "another update is already running",
  "api_version": "v1",
  "timestamp": "2025-01-20T11:30:45Z"
}
```

The client should wait at least 30 seconds (as indicated by the `Retry-After` header) before attempting another request.

## Response Format

The `/v1/update` endpoint returns a JSON response containing the results of the update operation:

```json
{
  "summary": {
    "scanned": 8,
    "updated": 0,
    "failed": 0,
    "restarted": 0,
    "skipped": 2
  },
  "timing": {
    "duration_ms": 1250,
    "duration": "1.25s"
  },
  "timestamp": "2025-01-20T11:30:45Z",
  "api_version": "v1"
}
```

### Summary Section

- `scanned`: Number of containers that were scanned for updates
- `updated`: Number of containers that were successfully updated
- `failed`: Number of containers where the update failed
- `restarted`: Number of containers that were restarted due to linked dependencies
- `skipped`: Number of containers that were skipped during the update

### Timing Section

- `duration_ms`: Execution time in milliseconds
- `duration`: Human-readable execution time

### Metadata

- `timestamp`: UTC timestamp when the response was generated (RFC3339 format)
- `api_version`: API version identifier

## HTTP Status Codes

The `/v1/update` endpoint returns the following HTTP status codes:

| Status Code | Description                                                                                                                                 |
|:-----------:|:--------------------------------------------------------------------------------------------------------------------------------------------|
|     200     | Update completed successfully                                                                                                               |
|     202     | Update triggered successfully and running asynchronously (with `?async=true`)                                                               |
|     401     | Invalid or missing authentication token                                                                                                     |
|     429     | Another update is already in progress (full updates only) or the request was rate limited                                                   |
|     500     | Internal server error during request processing                                                                                             |
|     503     | Update handler timed out (exceeded configured limit, default 10m), or client cancelled while waiting on update lock (targeted updates only) |

## Error Response Format

When an error occurs, the API returns a JSON response with the following structure:

```json
{
  "error": "another update is already running",
  "api_version": "v1",
  "timestamp": "2025-01-20T11:30:45Z"
}
```

- `error`: A human-readable error message describing what went wrong
- `api_version`: API version identifier
- `timestamp`: UTC timestamp when the error response was generated (RFC3339 format)

## Concurrency Behavior

The `/v1/update` endpoint handles concurrent requests differently based on whether targeted or full updates are being performed:

**Full Updates (no `?image=` or `?container=` parameter):**

- Returns HTTP 429 immediately if another update is already in progress
- Includes a `Retry-After: 30` header suggesting when to retry the request
- Does not block or wait for the existing update to complete

**Targeted Updates (with `?image=` or `?container=` parameter):**

- Blocks until the update lock is available
- Waits for any in-progress update to complete before proceeding
- Does not return HTTP 429

This behavior ensures that full updates (which may be resource-intensive) are not queued up, while targeted updates (which are typically faster) can wait for their turn.

## Security

### Rate Limiting

Watchtower enforces two independent mechanisms that can each return HTTP 429 (Too Many Requests):

**Per-IP request-rate limiting** (applies globally to all HTTP API endpoints):

- Every incoming request is checked against a per-IP rate limiter using a **sliding window** algorithm **before** authentication is evaluated.
- Default limit: 60 requests per minute.
- Configurable via the [HTTP API Rate Limit](../../../configuration/http-api/index.md#http_api_rate_limit) configuration option.
- Rate-limited requests receive HTTP 429 with no body.
- Rate limit state is tracked per client IP address.

**Concurrency-based update limiting** (applies only to `/v1/update`):

- The `/v1/update` handler uses an internal lock to ensure only one update runs at a time.
- If a full update (no `image` or `container` query parameter) is requested while another update is already in progress, the handler immediately returns HTTP 429 with a JSON error body and a `Retry-After: 30` header.
- Targeted updates (with `image` or `container` query parameter) block until the lock is available rather than returning 429.

**Precedence:** Per-IP rate limiting is evaluated first. If a request passes the rate limit, it proceeds to the endpoint handler where concurrency limiting may apply for `/v1/update`.

### Request Body Protection

- Request bodies are capped at 1 MiB to prevent resource exhaustion from large uploads
- Requests exceeding this limit will be rejected with HTTP 413 (Payload Too Large)

### Update Handler Timeout

- The `/v1/update` handler has a timeout enforced by Fiber's timeout middleware. The default is 10 minutes.
- The timeout covers the full lifecycle: waiting for the concurrency lock, performing the container update scan, and returning results.
- When the timeout is exceeded, the handler returns HTTP 503 (Service Unavailable) and the handler goroutine is abandoned.
- The handler listens on `c.Context().Done()` for cooperative cancellation — targeted updates waiting for the lock will return HTTP 503 when the request context is cancelled.
- You can override the timeout per request with the `?timeout=` query parameter, up to the configured maximum.

## Using the HTTP API and Periodic Updates

By default, enabling the HTTP API prevents periodic updates (i.e. [scheduled](../../../configuration/scheduling/index.md#schedule) or [interval](../../../configuration/scheduling/index.md#interval) polling).

Use the [HTTP API Periodic Polls](../../../configuration/scheduling/index.md#http_api_periodic_polls) configuration option to enable periodic updates while using the HTTP API.

## Example

```yaml title="Example Docker Compose Configuration"
services:
  app-monitored-by-watchtower:
    image: myapps/monitored-by-watchtower
    labels:
      - "com.centurylinklabs.watchtower.enable=true"

  watchtower:
    image: nickfedor/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - WATCHTOWER_HTTP_API_TOKEN=mytoken
      - WATCHTOWER_HTTP_API_ENDPOINTS=update,metrics
    labels:
      - "com.centurylinklabs.watchtower.enable=false"
    ports:
      - 8080:8080
    restart: unless-stopped
```

!!! Note
    Enable multiple endpoints in one allowlist (for example `update,metrics`) via [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints).

!!! Warning
    Enabling the HTTP API with port mappings will automatically disable Watchtower's self-update functionality to prevent port conflicts during container recreation. See [Updating Watchtower](../../../getting-started/updating-watchtower/index.md#port_configuration_limitation) for more details.

## SSE Events

When the [`/v1/events`](../events/index.md) SSE endpoint is also enabled, the update process broadcasts the following events:

- `scan_started`:  Broadcasted before the update scan begins
- `scan_completed`:  Broadcasted after the update scan finishes
- `scan_failed`:  Broadcasted if the update scan encounters an error
- `image_cleanup`:  Broadcasted after image cleanup if enabled and images were removed
