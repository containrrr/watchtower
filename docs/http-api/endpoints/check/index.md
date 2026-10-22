# Check

## Overview

The `v1/check` endpoint enables checking monitored containers for available image updates.

Containers on the registry path are checked by querying the registry for the latest digest (HTTP HEAD with GET fallback).
Containers associated for [Git monitoring](../../../advanced-features/git-monitoring/index.md) use the same watch split as scheduled updates. Watchtower reports whether the hosted Git ref is stale. It does not clone or build.

It does **not** download image layers and does **not** check against the configured [image cooldown](../../../advanced-features/image-cooldown/index.md), as the cooldown functionality remains an apply-time gate for scheduled updates and `/v1/update`.

When [no-pull](../../../configuration/update-behavior/index.md#disable_image_pulling) is enabled globally or via the container label, a registry-path container is checked against the local image cache only. The registry is not contacted.
A Git-watched container is not checked. The Git remote is not contacted, and the result is `update_available: false`.

Include `check` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this endpoint.

## Configuration

| Setting | Flag | Environment Variable | Default |
|:--------|:-----|:---------------------|:--------|
| Check API timeout | [`--http-api-check-timeout`](../../../configuration/http-api/index.md#http_api_check_timeout) | `WATCHTOWER_HTTP_API_CHECK_TIMEOUT` | `5m` |

## Parameters

### Image Name

The `image` parameter filters the check to only include containers running specific image names.

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/check?image=foo/bar:1.0"
```

### Container Name

The `container` parameter filters the check to only include specific containers by container name.

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/check?container=nginx"
```

### Timeout

The `timeout` parameter overrides the per-request timeout for this check.
It accepts Go durations such as `30s`, `2m`, or `5m`.
The value is capped by the configured check API timeout (`--http-api-check-timeout` / `WATCHTOWER_HTTP_API_CHECK_TIMEOUT`, default `5m`).

```bash
curl -X POST -H "Authorization: Bearer mytoken" "localhost:8080/v1/check?timeout=2m"
```

## Response Format

The `/v1/check` endpoint returns a JSON array of container check results:

```json
{
    "containers": [
        {
            "name": "nginx",
            "image": "nginx:latest",
            "image_id": "sha256:abc...",
            "digest": "sha256:old...",
            "update_available": true,
            "latest_image_id": "",
            "latest_digest": "sha256:new...",
            "update_source": "registry",
            "git_repo": "https://github.com/org/app.git",
            "git_ref": "main",
            "git_commit": "abc123def456",
            "changelog": "https://github.com/org/app/releases",
            "oci_source": "https://github.com/org/app",
            "current_image_version": "1.2.3",
            "current_revision": "1111111111",
            "timestamp": "2025-01-20T11:30:45Z"
        }
    ],
    "count": 1,
    "timestamp": "2025-01-20T11:30:45Z",
    "api_version": "v1"
}
```

- `name`: Container name
- `image`: Current image reference with tag
- `image_id`: Current local image ID
- `digest`: Current local registry digest when known
- `update_available`: Whether a newer image is available
- `latest_image_id`: Local image ID of the newer image when known. It is empty for Git checks, which do not build or pull an image
- `git_commit`: Resolved Git commit for a Git-watched container when known
- `latest_digest`: Newest registry digest when known
- `error`: Per-container error message when the check failed
- `update_source`: `registry` or `git` (the staleness path; not OCI `source`)
- `git_repo`, `git_ref`, `changelog`: Resolved Git metadata when present
- `oci_source`, `image_url`, `documentation`: OCI image annotations when present
- `current_image_version`, `current_revision`: OCI `version` and `revision` of the running image when present

!!! Note "No latest version here"
    This endpoint never pulls an image, so `latest_image_version` and `latest_revision` are always empty.
    Use `update_available` and `latest_digest` to detect a pending update, and read the new version from a [notification template](../../../notifications/templates/index.md#git_and_oci_report_fields) after the update session runs.

Containers associated for Git watching use the same watch split as scheduled updates.
The check does not clone or build.
See [Git Monitoring](../../../advanced-features/git-monitoring/index.md).
Git and OCI report fields are documented under [notification templates](../../../notifications/templates/index.md#git_and_oci_report_fields).

## HTTP Status Codes

| Status Code | Description                                    |
|:-----------:|:-----------------------------------------------|
|     200     | Check completed successfully                   |
|     401     | Invalid or missing authentication token        |
|     500     | Internal server error during request processing|

## SSE Events

When the [`/v1/events`](../events/index.md) SSE endpoint is also enabled, execution of the `v1/check` endpoint broadcasts the following events:

- `scan_started`:  Broadcasted before the check begins
- `scan_completed`: Broadcasted after the check finishes successfully
- `scan_failed`: Broadcasted if the check encounters an error

!!! Note
    The `scan_completed` payload always reports `updated: 0` because no updates are applied.
    The `failed` field counts containers whose per-container check returned an error.
