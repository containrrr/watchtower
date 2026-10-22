# HTTP API

Watchtower exposes an HTTP API for triggering container updates, listing container
status, checking for updates, and exposing metrics. All endpoints (except health
checks) require authentication via a Bearer token or (on the events endpoint) an
events token.

## Endpoints

| Method | Path                     | Auth | Description                                                                                             |
|--------|--------------------------|------|---------------------------------------------------------------------------------------------------------|
| GET    | `/livez`                 | No   | Health check — always returns 200 when running                                                          |
| GET    | `/readyz`                | No   | Health check — verifies Docker client connectivity                                                      |
| GET    | `/startupz`              | No   | Health check — always returns 200 once started                                                          |
| POST   | `/v1/check`              | Yes  | Check containers for available updates (registry digest or Git ref)                                     |
| GET    | `/v1/containers`         | Yes  | List watched container statuses                                                                         |
| GET    | `/v1/containers/details` | Yes  | Detailed container information including config flags                                                   |
| GET    | `/v1/history`            | Yes  | Historical scan results from the in-memory ring buffer                                                  |
| GET    | `/v1/images`             | Yes  | Tracked images with digests and container counts                                                        |
| GET    | `/v1/config`             | Yes  | Active Watchtower configuration settings                                                                |
| GET    | `/v1/events`             | Yes  | Real-time operational events via SSE (`scan_started`, `scan_failed`, `image_cleanup`, `scan_completed`) |
| POST   | `/v1/update`             | Yes  | Trigger container update scan                                                                           |
| GET    | `/v1/status`             | Yes  | Last scan summary                                                                                       |
| GET    | `/v1/metrics`            | Yes  | Prometheus exposition format metrics                                                                    |
| GET    | `/swagger/*`             | No   | Swagger UI documentation (Try it out still needs Authorize for /v1/*)                                   |

## Authentication

All `/v1/` endpoints require authentication.

By default, a Bearer token is provided via the `Authorization` header, configured through the `http-api-token` configuration option:

```bash
curl -H "Authorization: Bearer $TOKEN" localhost:8080/v1/containers
```

The `/v1/events` SSE stream uses a separate `http-api-events-token` token and accepts it via either the `Authorization: Bearer` header or the `access_token` query parameter (for browser `EventSource` connections that cannot set headers).

## Query Parameters

### `/v1/update`

- `image` — Comma-separated image names to filter (repeatable).
- `container` — Comma-separated container name patterns to filter (repeatable, supports Go regex).
- `async` — When `true`, runs the update asynchronously and returns `202 Accepted`.
- `timeout` — Per-request timeout override (e.g. `30s`, `2m`). Bounded by the configured update API timeout.

### `/v1/check`

- `image` — Comma-separated image names to filter (repeatable).
- `container` — Comma-separated container names to filter (repeatable).
- `timeout` — Per-request timeout override (e.g. `30s`, `2m`). Bounded by the configured check API timeout.

### `/v1/containers`

- `name` — Filter by container name (exact match).
- `image` — Filter by image name (exact match).

### `/v1/containers/details`

- `name` — Filter by container name (exact match).
- `image` — Filter by image name (exact match).

### `/v1/history`

- `since` — Include entries at or after this RFC3339 timestamp.
- `until` — Include entries at or before this RFC3339 timestamp.
- `limit` — Maximum number of entries to return (default: all).

### `/v1/images`

- `name` — Filter by image name (exact match).
- `id` — Filter by image ID (sha256 digest).

## Swagger / OpenAPI

The `internal/api/swagger/` package contains the generated OpenAPI 2.0 (Swagger) spec.
It is produced from Go source annotations using [swaggo/swag](https://github.com/swaggo/swag).

### Installing the swag CLI

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Alternatively, download a pre-compiled binary from the [release page](https://github.com/swaggo/swag/releases) or use the Docker
image:

```bash
docker run --rm -v $(pwd):/code ghcr.io/swaggo/swag:latest
```

### Regenerating

```bash
go generate ./internal/api/swagger/
```

This runs `swag init` against `main.go` and writes `docs.go`, `swagger.json`, and `swagger.yaml` into this directory.
Always regenerate after changing any swag annotations on handler methods.

### Formatting Annotations

`swag fmt` automatically formats swag comments (like `go fmt` for Go code):

```bash
swag fmt
```

This project uses the following flags:

```bash
swag fmt -g main.go -d .
```

`swag fmt` requires a standard doc comment above each function with swag annotations so it can correctly indent the annotation lines with tabs.

### Annotation Format

Handlers use `// @Name value` line annotations.

Key rules:

- `@Description` must be a single complete sentence on one line (swaggo truncates multi-line descriptions).
- `@Success` / `@Failure` response schemas use inline types (`{string}` and `{object}`).
  Swaggo cannot resolve cross-package types in annotations.
- `@Tags` must not contain quoted strings; use separate `@Tag.name` and
  `@Tag.description` annotations in `main.go`.
- `@Param` format: `@Param name in type required "description"`.
