# Ephemeral Self-Updates

## Overview

!!! Warning "This is an experimental feature"

The ephemeral self-update mechanism is an alternative to the default rename-based approach. It uses a short-lived orchestrator container to perform the container replacement, providing a more atomic handoff between old and new Watchtower instances.

## How It Works

1. Watchtower detects a new version of its own image is available and pulls it.
2. A short-lived orchestrator container is created from the new Watchtower image with the `--self-update-orchestrator` internal flag.
3. The orchestrator reaches the Docker daemon the same way Watchtower does. It receives Watchtower's Docker connection settings and mounts the socket that `DOCKER_HOST` names, such as a socket proxy, or Watchtower's Docker socket when `DOCKER_HOST` is not set. It then performs the following sequence:
    - Stops the old Watchtower container.
    - Creates a new container from the new image with the same configuration.
    - Starts the new Watchtower container.
    - Verifies the new container is running.
    - Removes the old container.
4. The orchestrator exits and is automatically removed.

## Enabling Ephemeral Self-Updates

=== "Docker Compose"

    ```yaml title="docker-compose.yml"
    services:
        watchtower:
            image: nickfedor/watchtower
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            environment:
                - WATCHTOWER_EPHEMERAL_SELF_UPDATE=true
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        --restart unless-stopped \
        -e WATCHTOWER_EPHEMERAL_SELF_UPDATE=true \
        nickfedor/watchtower \
    ```

## Differences from Default Self-Update

| Aspect                | Default (Rename)                   | Ephemeral                                         |
|-----------------------|------------------------------------|---------------------------------------------------|
| Mechanism             | Renames old container, creates new | Orchestrator handles stop/create/start            |
| Port conflicts        | Skipped automatically              | Self-update not skipped when ports are configured |
| Old container cleanup | Deferred to next startup           | Immediate removal by orchestrator                 |
| Failure recovery      | Old container persists (renamed)   | Old container preserved if new one fails          |

## Limitations

- For both update mechanisms, the Watchtower container requires the Docker socket to be mounted as a volume or a Docker host URL.
- The orchestrator container is identified by the `com.centurylinklabs.watchtower.ephemeral-orchestrator` label. Orphaned orchestrators from crashes are cleaned up on Watchtower startup.
