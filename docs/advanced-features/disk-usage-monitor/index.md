# Disk Usage Monitor

## Overview

By default, Watchtower does not monitor how much disk space container images occupy before pulling new layers.
If [`image cleanup`](../../configuration/update-behavior/index.md#cleanup_old_images) is not enabled, unused layers tend to accumulate, and each later session can still pull a newer image.
This can contribute to storage exhaustion over time.

Two optional settings address that problem:

- [`disk-space-max`](../../configuration/update-behavior/index.md#disk_space_max) stops the update session when current image usage already meets or exceeds a configured ceiling.
- [`disk-space-warn`](../../configuration/update-behavior/index.md#disk_space_warn) emits a warning when usage reaches a threshold, then continues.

Either option enables the monitor.
A warning without a maximum is valid and is useful when you only want visibility.

The monitor measures **Docker image usage only**.
It does not include container writable layers, named or anonymous volumes, or the build cache, so large application data cannot false-block updates.

!!! Warning "This is not a host free-space floor."
    The Docker Engine does not report how much free space remains on the host filesystem.
    Image usage can be under budget while the host disk is full of logs, bind mounts, or other data, and the reverse can also be true.
    Set the thresholds as the amount of **image** storage you are willing to let Docker keep, not as "leave 5 GB free on `/`".

## Configuration

The [`disk-space-max`](../../configuration/update-behavior/index.md#disk_space_max) and [`disk-space-warn`](../../configuration/update-behavior/index.md#disk_space_warn) configuration options are set via the Watchtower container using either environment variables or CLI flags.
No extra host bind mount is required, as the check uses the Docker API.

=== "Docker Compose"

    ```yaml title="docker-compose.yml"
    services:
        watchtower:
            image: nickfedor/watchtower
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            environment:
                - WATCHTOWER_DISK_SPACE_MAX=40GB
                - WATCHTOWER_DISK_SPACE_WARN=80%
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        --restart unless-stopped \
        nickfedor/watchtower \
        --disk-space-max 40GB \
        --disk-space-warn 80%
    ```

=== "Warn only"

    ```yaml title="Observability without blocking"
    services:
        watchtower:
            image: nickfedor/watchtower
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            environment:
                - WATCHTOWER_DISK_SPACE_WARN=20GiB
    ```

See [`disk-space-max`](../../configuration/update-behavior/index.md#disk_space_max) and [`disk-space-warn`](../../configuration/update-behavior/index.md#disk_space_warn) for accepted units, percentage rules, and defaults.

## Behavior

When the monitor is enabled, Watchtower asks the Docker daemon for image disk usage once at the start of each update session, before containers are listed.
That query is the same information shown by [`docker system df`](https://docs.docker.com/reference/cli/docker/system/df/){target="_blank" rel="noopener noreferrer"}, restricted to images so volume and build-cache walks are skipped.
Shared image layers are counted once.

| Condition                                      | Session       | Notification           |
|------------------------------------------------|---------------|------------------------|
| Both options unset                             | Runs normally | None from this feature |
| Usage below the configured thresholds          | Runs normally | Debug log only         |
| Usage at or above warn, max unset              | Runs normally | Warning                |
| Usage at or above warn, below a configured max | Runs normally | Warning                |
| Usage at or above max                          | Aborted       | Error                  |
| Image usage cannot be queried                  | Aborted       | Error                  |

An aborted session does not list containers, does not pull, and does not run [`cleanup`](../../configuration/update-behavior/index.md#cleanup_old_images).
If the daemon cannot report image usage, Watchtower treats pulling more images as unsafe and fails closed.

The check runs once per session, not before every pull.
Watchtower cannot reserve space for the next image, so a single large pull can still push usage over the budget after the session has started.
An out-of-space failure during a pull is reported as a normal container update failure.

## Metrics

When the [`metrics`](../../http-api/endpoints/metrics/index.md) endpoint is enabled through [`http-api-endpoints`](../../configuration/http-api/index.md#http_api_endpoints), each check updates:

| Gauge                                        | Meaning                                       |
|----------------------------------------------|-----------------------------------------------|
| `watchtower_docker_images_bytes`             | Image storage in use at the last check        |
| `watchtower_docker_images_reclaimable_bytes` | Image storage that unused images could free   |
| `watchtower_disk_space_max_bytes`            | Configured maximum, or `0` if unset           |
| `watchtower_disk_space_warn_bytes`           | Configured warning threshold, or `0` if unset |

Reclaimable bytes are informational.
The monitor does not prune unused images.

## Notifications

Warning and block events use the existing notification log hook.

The default simple template includes usage, the relevant threshold, reclaimable bytes, and image count.
See [Templates](../../notifications/templates/index.md).

## Interactions

- [`cleanup`](../../configuration/update-behavior/index.md#cleanup_old_images) runs only after a successful session, so a blocked session does not free image space.
- [`remove-volumes`](../../configuration/update-behavior/index.md#remove_anonymous_volumes) is unrelated.
  It deletes anonymous volumes when a container is recreated.
- [`no-pull`](../../configuration/update-behavior/index.md#disable_image_pulling) does not disable the monitor.
  The check still measures existing image usage.
- HTTP [`update`](../../http-api/endpoints/update/index.md) uses the same session-start gate as a scheduled run.
- HTTP [`check`](../../http-api/endpoints/check/index.md) compares digests only and does not apply the monitor.
- [`rolling-restart`](../../configuration/update-behavior/index.md#rolling_restart) is not re-checked between containers.
  If the session starts, rolling restart proceeds under that session's budget.

## Limitations

- The monitor cannot see host free space, logs, bind mounts, or other non-image data.
- Volumes and the build cache can still fill the Docker data root.
- The gate cannot reserve space for an incoming pull.
- Reclaimable figures from some compatible runtimes can be overstated when images share layers.
- There is no automatic prune.
  Use [`cleanup`](../../configuration/update-behavior/index.md#cleanup_old_images) after successful updates, or remove unused images yourself.
