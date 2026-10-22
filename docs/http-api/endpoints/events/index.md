# Events

## Overview

The `/v1/events` endpoint streams Watchtower operational events via Server-Sent Events (SSE).
Include `events` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this endpoint.

!!! Warning "Not usable via Swagger UI Try it out"
    Swagger UI will show a permanent **Loading** spinner on this endpoint even after successful authentication.
    The UI uses `fetch` and expects a finished response body; SSE is an open stream.
    Use `curl -N` or browser `EventSource` as shown below (see also [Swagger UI](../swagger/index.md)).

## Authentication

The events endpoint requires authentication via the [HTTP API Events Token](../../../configuration/http-api/index.md#http_api_events_token) configuration option.

This token can be provided in two formats:

- **Header-based auth** — For programmatic clients using `curl`, `fetch`, etc.:

  ```bash
  curl -N -H "Authorization: Bearer my-events-token" "http://localhost:8080/v1/events"
  ```

- **Query-parameter auth** — For browser `EventSource` API, which cannot send custom headers:

  ```javascript
  const eventSource = new EventSource('http://localhost:8080/v1/events?access_token=my-events-token');

  eventSource.addEventListener('scan_started', (e) => {
      console.log('Scan started:', JSON.parse(e.data));
  });

  eventSource.addEventListener('scan_completed', (e) => {
      console.log('Scan completed:', JSON.parse(e.data));
  });
  ```

  ```bash
  curl -N "http://localhost:8080/v1/events?access_token=my-events-token"
  ```

The events endpoint uses a separate token from the main API token to limit blast radius, since query parameters may appear in access logs, browser history, and proxy logs.

## Supported Events

### Scan Events

- Started
- Completed
- Failed

!!! Note
    Scan events are broadcasted only for updates (HTTP API or scheduled) or checks (HTTP API).

### Update Events

- Started
- Completed
- Failed

!!! Note
    Update events are broadcasted only for HTTP API or scheduled updates.

## Event Format

Each event is a Server-Sent Event with an event type and JSON data payload:

```text
event: scan_completed
data: {"type":"scan_completed","timestamp":"2025-01-20T11:30:45Z","data":{"scanned":8,"updated":0}}
```

## HTTP Status Codes

| Status Code | Description                             |
|:-----------:|:----------------------------------------|
|     200     | Event stream established                |
|     401     | Invalid or missing authentication token |
|     403     | Origin not allowed                      |
