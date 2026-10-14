# Containers

## Overview

The `/v1/containers` endpoint lists monitored containers along with their current image ID.

Include `containers` in [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints) to enable this read-only endpoint.

## Parameters

### Container Name

The `name` parameter filters results to a specific container by exact name.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/containers?name=nginx"
```

### Image Name

The `image` parameter filters results to containers running a specific image.

```bash
curl -H "Authorization: Bearer mytoken" "localhost:8080/v1/containers?image=nginx:latest"
```

## Response Format

The `/v1/containers` endpoint returns a JSON array of watched containers:

```json
{
    "containers": [
        {
            "name": "nginx",
            "image": "nginx:latest",
            "image_id": "sha256:1111...",
            "digest": "sha256:2222..."
        }
    ],
    "count": 1,
    "timestamp": "2025-01-20T11:30:45Z",
    "api_version": "v1"
}
```

- `name`: Container name
- `image`: Image reference with tag
- `image_id`: Local image config ID
- `digest`: Registry manifest digest the image was pulled from
      - Taken from the image's `RepoDigests` and comparable to a registry's `Docker-Content-Digest`
      - For an image with digests from several repositories, the one for the container's image repository, or the first digest when none matches it
      - Empty for locally built images with no registry reference

!!! Note
    Combine endpoints in one allowlist (for example `containers,update,metrics`) via [`http-api-endpoints`](../../../configuration/http-api/index.md#http_api_endpoints).
