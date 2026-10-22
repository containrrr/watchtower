# Config

## Overview

The `v1/config` endpoint returns the active Watchtower configuration settings.
Sensitive values (notification URLs, tokens) are redacted.

Include `config` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this endpoint.

## Response Format

The `/v1/config` endpoint returns the current configuration:

```json
{
    "config": {
        "monitor_only": false,
        "cleanup": true,
        "no_pull": false,
        "no_restart": false,
        "rolling_restart": false,
        "include_stopped": false,
        "include_restarting": false,
        "lifecycle_hooks": false,
        "label_enable": false,
        "filter_desc": "",
        "scope": "",
        "git_enable": false
    },
    "timestamp": "2025-01-20T11:30:45Z",
    "api_version": "v1"
}
```

| Field                | Type      | Description                                      |
|:---------------------|:----------|:-------------------------------------------------|
| `monitor_only`       | `boolean` | Whether Watchtower is in monitor-only mode       |
| `cleanup`            | `boolean` | Whether old images are removed after updating    |
| `no_pull`            | `boolean` | Whether image pulling is disabled                |
| `no_restart`         | `boolean` | Whether container restarting is disabled         |
| `rolling_restart`    | `boolean` | Whether containers are restarted one at a time   |
| `include_stopped`    | `boolean` | Whether stopped containers are included          |
| `include_restarting` | `boolean` | Whether restarting containers are included       |
| `lifecycle_hooks`    | `boolean` | Whether lifecycle hooks are enabled              |
| `label_enable`       | `boolean` | Whether label-based enabling is active           |
| `filter_desc`        | `string`  | Human-readable description of the applied filter |
| `scope`              | `string`  | Monitoring scope                                 |
| `git_enable`         | `boolean` | Process-wide Git watcher default                 |

Tokens, passwords, and SSH key paths are never included.

## HTTP Status Codes

| Status Code | Description                                    |
|:-----------:|:-----------------------------------------------|
|     200     | Configuration retrieved successfully           |
|     401     | Invalid or missing authentication token        |
|     500     | Internal server error during request processing|
