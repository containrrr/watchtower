# Status

## Overview

The `/v1/status` endpoint returns a summary of the most recent Watchtower scan, including counts of scanned, updated, failed, restarted, and skipped containers.
Include `metrics` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable it (the status endpoint is enabled alongside metrics).

## Response Format

The `/v1/status` endpoint returns a JSON scan summary:

```json
{
    "summary": {
        "scanned": 8,
        "updated": 0,
        "failed": 0,
        "restarted": 0,
        "skipped": 2
    },
    "timestamp": "2025-01-20T11:30:45Z",
    "api_version": "v1"
}
```

- `scanned`: Number of containers scanned
- `updated`: Number of containers successfully updated
- `failed`: Number of containers where the update failed
- `restarted`: Number of containers restarted
- `skipped`: Number of containers skipped
- `scan_failed`: `true` when the scan ended in an error. The counts are then those the scan reached. Omitted for other scans
- `timestamp`: UTC timestamp of the last scan (RFC3339 format)

If no scan has been performed yet, the endpoint returns HTTP 204 (No Content).

## HTTP Status Codes

| Status Code | Description                                    |
|:-----------:|:-----------------------------------------------|
|     200     | Status retrieved successfully                  |
|     204     | No scan has been performed yet                 |
|     401     | Invalid or missing authentication token        |
|     500     | Internal server error during request processing|
