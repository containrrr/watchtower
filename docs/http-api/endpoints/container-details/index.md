# Container Details

## Overview

The `/v1/containers/details` endpoint returns detailed information about each watched container, including running state, image identity, and configuration flags.

Include `containers` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this endpoint (container details are enabled alongside the containers list).

## Parameters

### Container Name

The `name` parameter filters results to a specific container by exact name.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/containers/details?name=nginx"
```

### Image Name

The `image` parameter filters results to containers running a specific image.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/containers/details?image=nginx:latest"
```

## Response Format

The `/v1/containers/details` endpoint returns a JSON array of container details:

```json
{
    "containers": [
        {
            "name": "nginx",
            "image": "nginx:latest",
            "image_id": "sha256:1111...",
            "digest": "sha256:2222...",
            "running": true,
            "watchtower": false,
            "monitor_only": false,
            "no_pull": false,
            "enabled": true,
            "stale": false,
            "scope": ""
        }
    ],
    "count": 1,
    "timestamp": "2025-01-20T11:30:45Z",
    "api_version": "v1"
}
```

- `name`: Container name
- `image`: Image reference with tag
- `image_id`: Local image config ID (sha256:...)
- `digest`: Registry manifest digest (sha256:...) for the repository in `image`, or the first digest when none matches it. Empty for locally-built images.
- `running`: Whether the container is currently running
- `watchtower`: Whether this is the Watchtower container itself
- `monitor_only`: Whether the container is in monitor-only mode
- `no_pull`: Whether image pulling is disabled for this container
- `enabled`: Whether the container is enabled for watching
- `stale`: Whether the container's image is outdated
- `scope`: Monitoring scope of the container

## HTTP Status Codes

| Status Code | Description                                    |
|:-----------:|:-----------------------------------------------|
|     200     | Container details retrieved successfully       |
|     401     | Invalid or missing authentication token        |
|     500     | Internal server error during request processing|
