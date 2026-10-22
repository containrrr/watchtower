# Update Behavior

These options apply after Watchtower has decided a container is stale.
The default staleness signal is a registry digest change.
[Git monitoring](../git-monitoring/index.md) is an alternate signal that still ends in this same container update process (or Compose apply for a local path context).
See [Advanced Features → Git Monitoring](../../advanced-features/git-monitoring/index.md#interaction_with_other_update_options).

## Disable Container Restart

Stops and removes the old containers and creates new ones with the updated image, but does not start the new containers.
This is useful when an external system (e.g., systemd) manages the container lifecycle.

```text
            Argument: --no-restart
Environment Variable: WATCHTOWER_NO_RESTART
                Type: Boolean
             Default: false
```

!!! Warning
    Combining `--no-restart` with `--cleanup` during Watchtower self-update may leave a renamed Watchtower container running without starting a new one, preventing cleanup of the old image.

    Use cautiously for self-updating Watchtower instances and consider external lifecycle management (e.g., Docker Compose) to restart containers manually.

## Rolling Restart

Restarts containers one at a time to minimize downtime.
This is ideal for zero-downtime deployments with lifecycle hooks.
When containers have health checks configured, Watchtower waits for each container to become healthy before proceeding to the next one.

```text
            Argument: --rolling-restart
Environment Variable: WATCHTOWER_ROLLING_RESTART
                Type: Boolean
             Default: false
```

!!! Note
    When combined with `--cleanup`, image cleanup is deferred until all containers are updated, which may temporarily increase disk usage for large numbers of containers (>50).
    This is typically negligible for homelab setups but monitor disk space on resource-constrained hosts.

    If a container fails to become healthy within 5 minutes, Watchtower logs a warning but continues with the next container to avoid blocking the entire update process.

!!! Warning "This functionality is currently not supported when used in combination with linked-containers."
     This limitation exists because linked-containers require coordinated updates across dependency chains, which conflicts with the incremental nature of rolling restarts.

## Disk Space Max

Sets the maximum allowed Docker **image** usage and blocks the update session when the usage amount is greater than or equal to the configured allowance.

```text
            Argument: --disk-space-max
Environment Variable: WATCHTOWER_DISK_SPACE_MAX
                Type: String
             Default: unset
```

!!! Warning "This is not host free space and does not include volumes, container writable layers, or the build cache."

!!! Note
    - Accepted sizes: bare bytes, decimal `B`/`KB`/`MB`/`GB`/`TB`/`PB`, or binary `KiB`/`MiB`/`GiB`/`TiB`/`PiB`.
    - This option cannot be a percentage.
    - Either this option or [`disk-space-warn`](#disk_space_warn) enables the check and updates the Prometheus image-usage gauges.

See [Disk Usage Monitor](../../advanced-features/disk-usage-monitor/index.md) for more information.

## Disk Space Warn

Sets a warning threshold for when Docker image usage is greater than or equal to the configured amount.

```text
            Argument: --disk-space-warn
Environment Variable: WATCHTOWER_DISK_SPACE_WARN
                Type: String
             Default: unset
```

!!! Note
    - Accepted sizes: bare bytes, decimal `B`/`KB`/`MB`/`GB`/`TB`/`PB`, or binary `KiB`/`MiB`/`GiB`/`TiB`/`PiB`.
    - This option may also be a percent of [`disk-space-max`](#disk_space_max) (for example `80%`).
    - A percent value requires [`disk-space-max`](#disk_space_max).
    - When both are set, the warn amount must be less than the max.
    - Configuring a warning threshold without a maximum is valid and supported.

See [Disk Usage Monitor](../../advanced-features/disk-usage-monitor/index.md) for more information.

## Cleanup Old Images

Removes old images after updating containers to free disk space.

```text
            Argument: --cleanup
Environment Variable: WATCHTOWER_CLEANUP
                Type: Boolean
             Default: false
```

!!! Note
    During Watchtower self-updates, cleanup is deferred to the new container to prevent premature image deletion.

    Ensure `--no-restart` is not used with `--cleanup` to avoid incomplete updates.

## Remove Anonymous Volumes

Deletes anonymous volumes when updating containers.
Named volumes remain unaffected.

```text
            Argument: --remove-volumes
Environment Variable: WATCHTOWER_REMOVE_VOLUMES
                Type: Boolean
             Default: false
```

!!! Note
    Containers with the Docker `AutoRemove` option enabled are automatically removed by the Docker daemon after stopping.
    Watchtower skips explicit removal in such cases.
    This does not affect named volumes.

## Container Stop Timeout

Sets the timeout (e.g., `30s`) before forcibly stopping a container during updates.

```text
            Argument: --stop-timeout
Environment Variable: WATCHTOWER_TIMEOUT
                Type: Duration (e.g., 30s, 1m, 5m)
              Default: 30s
```

!!! Note
    Bare numeric values (e.g., `60` or `1.5`) without a time unit are interpreted as seconds.
    Using a unit suffix (`s`, `m`, etc.) is recommended and required for other time units.

## Monitor Only

Monitors for new images, sends notifications, and runs lifecycle hooks without updating containers.

```text
            Argument: --monitor-only
Environment Variable: WATCHTOWER_MONITOR_ONLY
                Type: Boolean
             Default: false
```

!!! Note
    Images may still be pulled due to Docker API limitations for digest comparison.

    Can be set per container via the `com.centurylinklabs.watchtower.monitor-only` label.

    See [Label Precedence](../container-selection/index.md#label_precedence).

## Disable Image Pulling

Prevents pulling new images from registries, monitoring only local image cache changes.
Useful for locally built images.

```text
            Argument: --no-pull
Environment Variable: WATCHTOWER_NO_PULL
                Type: Boolean
             Default: false
```

!!! Note
    Can be set per container via the `com.centurylinklabs.watchtower.no-pull` label.

    The HTTP API [`/v1/check`](../../http-api/endpoints/check/index.md) endpoint also respects no-pull and inspects the local cache only.

    No-pull also skips [Git monitoring](../../advanced-features/git-monitoring/index.md) rebuilds.
    `/v1/check` reports no update for a Git-watched container while no-pull applies to it.

    See [Label Precedence](../container-selection/index.md#label_precedence).

## Changelog Links

Adds a release-notes link to notifications, and resolves that link to the exact release being installed rather than the release index.

```text
            Argument: --enable-changelog
Environment Variable: WATCHTOWER_ENABLE_CHANGELOG
                Type: Boolean
             Default: false
```

!!! Warning "Extra registry requests"
    Resolving the exact release tag requires confirming which tag the maintainer actually published.
    Watchtower issues one or two extra manifest requests per updated container on a registry it has already contacted, which roughly doubles that session's manifest requests for the update.

    This is why the option is off by default.
    With it off, `.Changelog` still resolves to the unversioned release index and costs no requests.
    A Git-monitored container never needs the lookup, because its tag is already known.

!!! Note
    Can be set per container via the `com.centurylinklabs.watchtower.enable-changelog` label.

    The `com.centurylinklabs.watchtower.changelog-url` label supplies a changelog URL template but does **not** enable this option.
    A template that references `{tag}` stays unresolved until the option is turned on for that container.

    See [Label Precedence](../container-selection/index.md#label_precedence) and [Notification Templates](../../notifications/templates/index.md).

## Ephemeral Self-Update

Uses a short-lived orchestrator container to perform Watchtower self-updates instead of the default rename-based approach.

```text
            Argument: --ephemeral-self-update
Environment Variable: WATCHTOWER_EPHEMERAL_SELF_UPDATE
                Type: Boolean
             Default: false
```

!!! Warning "This is an experimental feature."

!!! Note
    The ephemeral self-update mechanism is only active when Watchtower is running in normal daemon mode.
    When Watchtower is started with the [`run-once`](../scheduling/index.md#run_once) configuration option, this flag is ignored because the process exits immediately after the initial update pass and there is no continuously running instance to replace.
    See [Advanced Features - Ephemeral Self-Updates](../../advanced-features/ephemeral-self-updates/index.md) for details on how this mechanism works.
