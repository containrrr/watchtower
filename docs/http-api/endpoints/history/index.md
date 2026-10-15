# History

## Overview

The `/v1/history` endpoint returns historical scan results from an in-memory ring buffer (up to 500 entries).
Include `history` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this read-only endpoint.

## Parameters

### Since

The `since` parameter filters entries to those at or after the specified RFC3339 timestamp.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/history?since=2025-01-20T11:00:00Z"
```

### Until

The `until` parameter filters entries to those at or before the specified RFC3339 timestamp.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/history?until=2025-01-20T12:00:00Z"
```

### Limit

The `limit` parameter restricts the maximum number of entries returned.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/history?limit=10"
```

### Combining Parameters

Parameters can be combined to query a specific time range with a limit:

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/history?since=2025-01-20T11:00:00Z&until=2025-01-20T12:00:00Z&limit=50"
```

## Response Format

The `/v1/history` endpoint returns a JSON object with scan history entries:

```json
{
    "entries": [
        {
            "timestamp": "2025-01-20T11:30:45Z",
            "scanned": 8,
            "updated": 0,
            "failed": 0,
            "restarted": 0,
            "skipped": 2
        }
    ],
    "count": 1,
    "timestamp": "2025-01-20T11:35:00Z",
    "api_version": "v1"
}
```

An entry for a scan that ended in an error also has `"scan_failed": true`, and its counts are those the scan reached.
The field is omitted for other scans.

## HTTP Status Codes

| Status Code | Description                                    |
|:-----------:|:-----------------------------------------------|
|     200     | History retrieved successfully                 |
|     400     | Invalid query parameter                        |
|     401     | Invalid or missing authentication token        |
|     500     | Internal server error during request processing|
