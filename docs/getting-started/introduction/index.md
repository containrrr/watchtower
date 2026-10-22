# Introduction

## Overview

Watchtower is an application that will monitor your running Docker containers and watch for changes to the images that those containers were originally started from.
If Watchtower detects that an image has changed, it will automatically restart the container using the new image.

With Watchtower you can update the container by pushing a new image to the registry that is respective to the container's image.
Watchtower will pull the new image, gracefully shut down the existing container, and restart it with the same options that were used when it was deployed initially.

Alternatively, [Git monitoring](../../advanced-features/git-monitoring/index.md) uses a hosted Git branch or tag as the staleness signal.
When that ref changes, Watchtower asks the Docker host to rebuild the image from a [Git URL context](https://docs.docker.com/build/concepts/context/#git-repositories){target="_blank" rel="noopener noreferrer"} or a local Compose [project directory](https://docs.docker.com/reference/compose-file/build/#context){target="_blank" rel="noopener noreferrer"}, then uses the same container update process.

## How Image Updates Work

Watchtower monitors containers using their **exact image reference**, including any tags specified.

This means:

- A container running `nginx:1.29` will **only** be updated when new images become available under the `nginx:1.29` tag
- It will **not** update when `nginx:latest`, `nginx:1.30`, or other tags receive updates
- A container running `myapp` (without a tag) will update when `myapp:latest` changes

This tag-specific behavior provides fine-grained control over update scopes, allowing you to pin containers to specific versions while still receiving updates within that version series.

## Examples of Different Tagging Schemes

While semantic versioning is ideal for predictable updates, many popular Docker images use different approaches.

The following examples show how Watchtower handles various tag types:

=== "Latest Tags"

    Containers using the implicit or explicit `latest` tag will update when the image maintainer pushes new content to the `latest` tag.

    ```bash title="Running an nginx:latest container"
    CONTAINER ID   IMAGE          STATUS          PORTS                    NAMES
    abc123def456   nginx:latest   Up 10 minutes   0.0.0.0:80->80/tcp       webserver
    ```

    **Expected behavior:**

    Watchtower checks for updates to `nginx:latest`.
    If nginx releases a new version and tags it as `latest`, the container will be updated.

    !!! Warning "This is commonly used, but can lead to unexpected changes because `latest` may include breaking changes from major version updates."

=== "Semantic Version Tags"

    Semantic versioning (e.g., `1.29`) provides more predictable updates within a major version.

    ```bash title="Running an nginx:1.29 container"
    CONTAINER ID   IMAGE          STATUS          PORTS                    NAMES
    def456ghi789   nginx:1.29     Up 10 minutes   0.0.0.0:80->80/tcp       webserver
    ```

    **Expected behavior:**

    Watchtower monitors the image `nginx:1.29` specifically.
    When nginx releases a new patch for version 1.29 (e.g. `1.29.1`, `1.29.2`, etc.), the maintainers will typically build and release a new patched version of the image with the `1.29` tag being applied to the updated image.
    Watchtower will then update the container to reflect the updated, `1.29`-tagged image.

=== "Other Tagging Schemes"

    Some images use date-based or incremental versioning schemes instead of semantic versioning.

    ```bash title="Running a pihole:2023.11 container"
    CONTAINER ID   IMAGE               STATUS          PORTS                    NAMES
    ghi789jkl012   pihole/pihole:2023.11   Up 10 minutes   0.0.0.0:8080->80/tcp   pihole
    ```

    **Expected behavior:**

    Watchtower checks for updates to `pihole/pihole:2023.11`.
    Pi-hole releases updates like `2023.11.1`, `2023.11.2` within the same month/year tag.
    The container updates to the latest patch within that period but not to `2023.12` or newer major releases.
