# Container Selection

## Overview

By default, Watchtower monitors and updates all running containers on the connected Docker daemon.
This behavior can be customized through a combination of configuration options and container labels to control exactly which containers are managed and how.

Container selection works through a **filter chain**: a series of criteria applied in sequence.
A container is monitored only if it passes every filter in the chain. The filters are evaluated in the following order:

| #  | Filter                                                       | Description                                                                                                                                                   |
|----|--------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1  | Old Watchtower container exclusion                           | Watchtower containers renamed with a `watchtower-old-` prefix during self-updates are always excluded.                                                        |
| 2  | [Disabled label check](#enabledisable_labels)                | Containers with the Docker label `com.centurylinklabs.watchtower.enable=false` are excluded.                                                                  |
| 3  | [Scope filter](#monitoring_scopes)                           | Only containers matching the configured scope are included (default: `"none"`).                                                                               |
| 4  | [Enable label filter](#enabledisable_labels)                 | If [label enable](../../configuration/container-selection/index.md#enable_label_filter) is set, only containers with the enable label present are included.   |
| 5  | [Disabled containers by label](#disable_containers_by_label) | Containers matching any [disabled label pair](../../configuration/container-selection/index.md#disable_containers_by_label) are excluded.                     |
| 6  | [Enabled containers by label](#enable_containers_by_label)   | If set, only containers matching at least one [enabled label pair](../../configuration/container-selection/index.md#enable_containers_by_label) are included. |
| 7  | [Image skip patterns](#exclude_specific_images)              | Containers whose image matches a [skip pattern](../../configuration/container-selection/index.md#skip_specific_images) are excluded.                          |
| 8  | [Monitored image name patterns](#monitor_specific_images)    | If set, only containers whose image matches a [monitored pattern](../../configuration/container-selection/index.md#monitor_specific_images) are included.     |
| 9  | [Disabled container names](#exclude_specific_containers)     | Containers whose name matches a [disable pattern](../../configuration/container-selection/index.md#disable_specific_containers) are excluded.                 |
| 10 | [Container name arguments](#container_name_filtering)        | If positional name arguments are provided, only containers matching at least one are included.                                                                |

!!! Note
    - If an image name pattern is configured, then Watchtower will exclusively manage only the respective container(s).
    - If a container name is provided as an argument, then Watchtower will exclusively manage only the specified container(s).

!!! Note "All criteria must be satisfied"
    A container must pass **every** filter in the chain to be monitored. If any single filter rejects it, the container is excluded.

## Management Modes

Watchtower supports two management modes for containers:

- **Full Management** (default): Watchtower checks for updates, pulls new images, recreates containers, and runs lifecycle hooks.
- [**Monitor Only Mode**](../../configuration/update-behavior/index.md#monitor_only): Watchtower checks for updates, sends notifications, and runs lifecycle hooks, but does **not** recreate containers.

## Container State Filtering

By default, Watchtower only processes containers in the `running` state.

Two configuration options extend this to include additional states:

| Option                                                                                                          | Environment Variable            | Effect                                        |
|-----------------------------------------------------------------------------------------------------------------|---------------------------------|-----------------------------------------------|
| [Include Stopped Containers](../../configuration/container-selection/index.md#include_stopped_containers)       | `WATCHTOWER_INCLUDE_STOPPED`    | Include `created` and `exited` containers     |
| [Include Restarting Containers](../../configuration/container-selection/index.md#include_restarting_containers) | `WATCHTOWER_INCLUDE_RESTARTING` | Include `restarting` containers (Docker only) |

!!! Note "Podman compatibility"
    The `restarting` state is not available on Podman and is automatically excluded when Podman is detected.

## Enable/Disable Labels

The `com.centurylinklabs.watchtower.enable` label controls whether Watchtower manages a container.

!!! Note "This label is set on the container you want to manage, not on the Watchtower instance."

### Default Behavior

When [label enable](../../configuration/container-selection/index.md#enable_label_filter) is **not** set:

- Containers **without** the label are **monitored**
- Containers with `enable=true` are **monitored**
- Containers with `enable=false` are **excluded**

### With Label Enable

When [label enable](../../configuration/container-selection/index.md#enable_label_filter) is set:

- Containers with `enable=true` are **monitored**
- Containers with `enable=false` are **excluded**
- Containers **without** the label are **excluded**

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        someimage:
            labels:
                - "com.centurylinklabs.watchtower.enable=true"
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --label=com.centurylinklabs.watchtower.enable=true someimage
    ```
=== "Dockerfile"
    ```dockerfile
    LABEL com.centurylinklabs.watchtower.enable="true"
    ```
<!-- markdownlint-restore -->

To exclude a container, set the label to `false`:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        someimage:
            labels:
                - "com.centurylinklabs.watchtower.enable=false"
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --label=com.centurylinklabs.watchtower.enable=false someimage
    ```
=== "Dockerfile"
    ```dockerfile
    LABEL com.centurylinklabs.watchtower.enable="false"
    ```
<!-- markdownlint-restore -->

## Monitor-Only Mode

Individual containers can be set to [monitor-only mode](../../configuration/update-behavior/index.md#monitor_only), where Watchtower checks for updates and sends notifications but does not recreate the container.

Set the `com.centurylinklabs.watchtower.monitor-only` label to `true` on the container:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        someimage:
            labels:
                - "com.centurylinklabs.watchtower.monitor-only=true"
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --label=com.centurylinklabs.watchtower.monitor-only=true someimage
    ```
=== "Dockerfile"
    ```dockerfile
    LABEL com.centurylinklabs.watchtower.monitor-only="true"
    ```
<!-- markdownlint-restore -->

!!! Note
    The per-container label has the same effect as the global [monitor-only](../../configuration/update-behavior/index.md#monitor_only) option, but applies only to that specific container.

    When combined with [label precedence](../../configuration/container-selection/index.md#label_precedence), the container label overrides the global option. Without label precedence, the container is monitor-only if **either** the label or the global option is set.

## Container Name Filtering

Watchtower can filter containers based on their container name using [Go regex pattern matching](#regex_pattern_matching).

### Include Specific Containers

Pass container names as positional arguments to Watchtower.
When provided, only containers matching at least one name are monitored.

!!! Warning
    - Dependencies on containers that are not named are ignored. See [Dependencies on Unmonitored Containers](../../advanced-features/linked-containers/index.md#dependencies_on_unmonitored_containers).

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            command: nginx redis
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        nginx redis
    ```
<!-- markdownlint-restore -->

### Exclude Specific Containers

Use the [disable containers](../../configuration/container-selection/index.md#disable_specific_containers) option to exclude containers by name.
This supports comma- or space-separated values and regex patterns.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_DISABLE_CONTAINERS=container1,container2
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_DISABLE_CONTAINERS="container1,container2" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --disable-containers container1,container2
    ```
<!-- markdownlint-restore -->

## Label-Based Container Filtering

Watchtower can include or exclude containers based on arbitrary Docker label key-value pairs.

### Enable Containers by Label

Use the [enable containers by label](../../configuration/container-selection/index.md#enable_containers_by_label) option to restrict monitoring to containers that have at least one of the specified label pairs.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL=env=prod,team=platform
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL="env=prod,team=platform" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --enable-containers-by-label "env=prod,team=platform"
    ```
<!-- markdownlint-restore -->

An empty value (`key=`) matches any container where the label key exists, regardless of its value.

For example, `WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL=env=` enables any container with the `env` label set to any value.

### Disable Containers by Label

Use the [disable containers by label](../../configuration/container-selection/index.md#disable_containers_by_label) option to exclude containers that have any of the specified label pairs.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL=managed-by=external
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL="managed-by=external" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --disable-containers-by-label "managed-by=external"
    ```
<!-- markdownlint-restore -->

!!! Note
    Values containing commas are not supported. Use individual `key=value` pairs separated by commas.

!!! Note
    A label entry with an empty value (`key=`) performs a presence check: the label must exist on the container with any value. A non-empty value requires an exact match.

## Image Name Filtering

Watchtower can filter containers based on their image name using [Go regex pattern matching](#regex_pattern_matching).

Image name patterns match against the **full image name including its tag** (e.g., `nginx:latest`, `docker.io/library/nginx:1.25`).

!!! Note "If no tag is specified in the image reference, `:latest` is assumed."

### Monitor Specific Images

Use the [monitor image names](../../configuration/container-selection/index.md#monitor_specific_images) configuration option to restrict monitoring to containers whose image matches at least one pattern.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_MONITOR_IMAGE_NAMES=nginx:.*,redis:7.*
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_MONITOR_IMAGE_NAMES="nginx:.*,redis:7.*" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --monitor-image-names "nginx:.*,redis:7.*"
    ```
<!-- markdownlint-restore -->

### Exclude Specific Images

Use the [skip image names](../../configuration/container-selection/index.md#skip_specific_images) option to exclude containers whose image matches at least one pattern.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_SKIP_IMAGE_NAMES=postgres:.*,mcr.microsoft.com/*
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_SKIP_IMAGE_NAMES="postgres:.*,mcr.microsoft.com/*" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --skip-image-names "postgres:.*,mcr.microsoft.com/*"
    ```
<!-- markdownlint-restore -->

## Monitoring Scopes

Scopes allow multiple Watchtower instances to run on the same Docker host without interfering with each other.
Each instance manages only the containers within its scope.

1. Use the [scope filter](../../configuration/container-selection/index.md#scope_filter) option to define a scope.
2. Then, use the `com.centurylinklabs.watchtower.scope` label on containers to assign them to that scope.

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        # Scoped Application
        app-production:
            image: myapp:latest
            labels:
                - "com.centurylinklabs.watchtower.scope=production"

      # Scoped Watchtower watching "production" scope
        watchtower-production:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_SCOPE=production
            labels:
                - "com.centurylinklabs.watchtower.scope=production"
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --name watchtower-production \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --scope production
    ```
<!-- markdownlint-restore -->

!!! Note
    - Without a [scope filter](../../configuration/container-selection/index.md#scope_filter), Watchtower defaults to scope `"none"` and manages only **unscoped** containers (those without a `com.centurylinklabs.watchtower.scope` label, or with it set to `"none"` or `""`).
    - Containers with a non-empty scope label (e.g., `scope=production`) are **not** monitored by an unscoped Watchtower instance.
    - Set the scope filter to `none` to explicitly manage only unscoped containers (same as the default behavior).
    - Two instances cannot share the same scope.
    - An unscoped instance coexists with scoped instances.

    See [Running Multiple Instances](../../advanced-features/running-multiple-instances/index.md) for a complete guide.

## Regex Pattern Matching

Container name and image name filters support regular expressions using [Go regex syntax](https://pkg.go.dev/regexp/syntax){target="_blank" rel="noopener noreferrer"}.

!!! Important "Patterns are anchored to match the **full** name"
    Patterns are automatically anchored with `^...$`, meaning they must match the entire container or image name. Use `.*` for wildcard matching instead of bare `*`.

### Container Name Patterns

Container names are normalized before matching (the leading `/` stripped). Both positional arguments and the [disable containers](../../configuration/container-selection/index.md#disable_specific_containers) option support regex.

| Pattern           | Matches                                  |
|-------------------|------------------------------------------|
| `container.*`     | "container1", "container-abc"            |
| `.*-dev`          | "web-dev", "api-dev", "db-dev"           |
| `.*`              | Any container name                       |
| `nginx\|redis`    | Either "nginx" or "redis"                |
| `db-.*\|cache-.*` | Any name starting with "db-" or "cache-" |

### Image Name Patterns

Image name patterns match against the full `name:tag` string. Use `.*` to match any tag.

| Pattern                 | Matches                            |
|-------------------------|------------------------------------|
| `nginx:latest`          | Only `nginx:latest`                |
| `nginx:.*`              | `nginx` with any tag               |
| `docker\.io/library/.*` | Any official Docker Hub image      |
| `.*\.azurecr\.io/.*`    | Any Azure Container Registry image |

### Examples

Exclude all containers starting with a prefix:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_DISABLE_CONTAINERS=web-.*
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_DISABLE_CONTAINERS="web-.*" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --disable-containers "web-.*"
    ```
<!-- markdownlint-restore -->

Include only containers matching specific patterns:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            command: ["nickfedor/watchtower", "db-.*", "cache-.*"]
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        "db-.*" "cache-.*"
    ```
<!-- markdownlint-restore -->

Monitor only containers using nginx or redis images with any tag:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_MONITOR_IMAGE_NAMES=nginx:.*,redis:.*
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_MONITOR_IMAGE_NAMES="nginx:.*,redis:.*" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --monitor-image-names "nginx:.*,redis:.*"
    ```
<!-- markdownlint-restore -->

## Label Precedence

By default, when a container-level label (e.g., `com.centurylinklabs.watchtower.monitor-only`) and a global option (e.g., [monitor-only](../../configuration/update-behavior/index.md#monitor_only)) are both set, the container uses the **combined** effect (either triggers the behavior).

With [label precedence](../../configuration/container-selection/index.md#label_precedence), container labels **override** the global options. This allows per-container control even when global options are set.

| [Label Precedence](../../configuration/container-selection/index.md#label_precedence) | Container Label | Global Option | Result            |
|---------------------------------------------------------------------------------------|-----------------|---------------|-------------------|
| false (default)                                                                       | not set         | false         | false             |
| false (default)                                                                       | not set         | true          | true              |
| false (default)                                                                       | true            | false         | true              |
| false (default)                                                                       | true            | true          | true              |
| true                                                                                  | not set         | any           | global flag value |
| true                                                                                  | true            | false         | true              |
| true                                                                                  | false           | true          | false             |

This applies to the [`monitor-only`](../../configuration/update-behavior/index.md#monitor_only) and [`no-pull`](../../configuration/update-behavior/index.md#disable_image_pulling) configuration options.

## Complete Configuration Reference

### CLI Flags and Environment Variables

| Flag                            | Environment Variable                     | Type     | Default | Description                           |
|---------------------------------|------------------------------------------|----------|---------|---------------------------------------|
| *(positional args)*             | N/A                                      | []string | []      | Container names/patterns to include   |
| `--disable-containers` / `-x`   | `WATCHTOWER_DISABLE_CONTAINERS`          | []string | []      | Container names/patterns to exclude   |
| `--enable-containers-by-label`  | `WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL`  | []string | []      | Label key=value pairs to include      |
| `--disable-containers-by-label` | `WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL` | []string | []      | Label key=value pairs to exclude      |
| `--monitor-image-names`         | `WATCHTOWER_MONITOR_IMAGE_NAMES`         | []string | []      | Image name patterns to monitor        |
| `--skip-image-names`            | `WATCHTOWER_SKIP_IMAGE_NAMES`            | []string | []      | Image name patterns to exclude        |
| `--label-enable` / `-e`         | `WATCHTOWER_LABEL_ENABLE`                | bool     | false   | Require enable label on containers    |
| `--scope`                       | `WATCHTOWER_SCOPE`                       | string   | ""      | Monitoring scope                      |
| `--include-stopped` / `-S`      | `WATCHTOWER_INCLUDE_STOPPED`             | bool     | false   | Include created and exited containers |
| `--include-restarting`          | `WATCHTOWER_INCLUDE_RESTARTING`          | bool     | false   | Include restarting containers         |
| `--label-take-precedence`       | `WATCHTOWER_LABEL_TAKE_PRECEDENCE`       | bool     | false   | Labels override global flags          |

### Container Labels

| Label                                                                                     | Values                          | Effect                                                                                                                     |
|-------------------------------------------------------------------------------------------|---------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `com.centurylinklabs.watchtower.enable`                                                   | true / false                    | Enable or disable management                                                                                               |
| `com.centurylinklabs.watchtower.monitor-only`                                             | true / false                    | Monitor without updating                                                                                                   |
| `com.centurylinklabs.watchtower.no-pull`                                                  | true / false                    | Skip image pulls                                                                                                           |
| `com.centurylinklabs.watchtower.scope`                                                    | any string                      | Assign to a monitoring scope                                                                                               |
| `com.centurylinklabs.watchtower.depends-on`                                               | comma-separated names           | Declare container dependencies                                                                                             |
| `com.centurylinklabs.watchtower.cooldown-delay`                                           | duration string                 | Minimum image age before updating                                                                                          |
| [`com.centurylinklabs.watchtower.copy-file`](../../advanced-features/copy-files/index.md) | comma-separated paths           | Copy named files onto the replacement container                                                                            |
| `com.centurylinklabs.watchtower.git-repo`                                                 | Git clone URL                   | Associate a Git repository. See [Git Monitoring](../../advanced-features/git-monitoring/index.md#labels).                  |
| `com.centurylinklabs.watchtower.git-ref`                                                  | branch or tag                   | Git ref to watch. `com.centurylinklabs.watchtower.git-branch` is an alias                                                  |
| `com.centurylinklabs.watchtower.git-host`                                                 | HTTP API base URL               | HTTP API origin when it is not the clone URL. See [Git Monitoring](../../advanced-features/git-monitoring/index.md#hosts)  |
| `com.centurylinklabs.watchtower.git-semver-policy`                                        | none / patch / minor / major    | Git semver tag advancement policy                                                                                          |
| `com.centurylinklabs.watchtower.git-watch`                                                | true / false / yes / no / 1 / 0 | Enable or disable the Git watcher for this container                                                                       |
| `com.centurylinklabs.watchtower.git-dockerfile`                                           | relative path                   | Dockerfile path relative to the build context                                                                              |
| `com.centurylinklabs.watchtower.git-context`                                              | relative path                   | Subdirectory of a Git URL context used as the Docker build context                                                         |
| `com.centurylinklabs.watchtower.compose-dir`                                              | path inside Watchtower          | Opt in to a local path context. See [Git Monitoring](../../advanced-features/git-monitoring/index.md#local_path_context). |

## Common Patterns

### Run Multiple Watchtower Instances

Run one instance for production containers and another for development:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower-prod:
            image: nickfedor/watchtower:latest
            command: --scope production --interval 300
            labels:
                - "com.centurylinklabs.watchtower.scope=production"
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock

        watchtower-dev:
            image: nickfedor/watchtower:latest
            command: --scope development --interval 30
            labels:
                - "com.centurylinklabs.watchtower.scope=development"
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI"
    ```bash
    docker run -d \
        --name watchtower-production \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --scope production --interval 300

    docker run -d \
        --name watchtower-development \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --scope development --interval 30
    ```
<!-- markdownlint-restore -->

### Exclude System Containers

Exclude Watchtower itself and other infrastructure containers:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_DISABLE_CONTAINERS=watchtower,traefik,portainer
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_DISABLE_CONTAINERS="watchtower,traefik,portainer" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --disable-containers "watchtower,traefik,portainer"
    ```
<!-- markdownlint-restore -->

### Exclude Containers by Label

Exclude containers managed by third-party orchestrators using arbitrary labels:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL=managed-by=external
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_DISABLE_CONTAINERS_BY_LABEL="managed-by=external" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --disable-containers-by-label "managed-by=external"
    ```
<!-- markdownlint-restore -->

### Monitor Containers by Label

Only monitor containers with specific labels:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL=env=prod,team=platform
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_ENABLE_CONTAINERS_BY_LABEL="env=prod,team=platform" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --enable-containers-by-label "env=prod,team=platform"
    ```
<!-- markdownlint-restore -->

### Monitor Only Specific Image Registries

Monitor only images from your private registry:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_MONITOR_IMAGE_NAMES=registry.example.com/.*
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
    ```
=== "Docker CLI (Env Vars)"
    ```bash
    docker run -d \
        -e WATCHTOWER_MONITOR_IMAGE_NAMES="registry.example.com/.*" \
        nickfedor/watchtower
    ```
=== "Docker CLI (Flags)"
    ```bash
    docker run -d \
        nickfedor/watchtower \
        --monitor-image-names "registry.example.com/.*"
    ```
<!-- markdownlint-restore -->

### Selective Monitoring with Enable Label

Use [label enable](../../configuration/container-selection/index.md#enable_label_filter) to explicitly opt containers into monitoring:

<!-- markdownlint-disable -->
=== "Docker Compose"
    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
                - WATCHTOWER_LABEL_ENABLE=true
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock

        myapp:
            image: myapp:latest
            labels:
                - "com.centurylinklabs.watchtower.enable=true"
    ```
=== "Docker CLI"
    ```bash
    # Start Watchtower with label filtering
    docker run -d \
        -e WATCHTOWER_LABEL_ENABLE=true \
        nickfedor/watchtower

    # Only this container will be monitored
    docker run -d \
        --label com.centurylinklabs.watchtower.enable=true \
        myapp:latest
    ```
<!-- markdownlint-restore -->
