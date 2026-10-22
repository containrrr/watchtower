# Copy Files

## Overview

When Watchtower recreates a container, it rebuilds that container from Docker API data. Bind mounts and volumes therefore carry over automatically.
Files that live only in the container's writable layer do not carry over.

[Docker Compose configs](https://docs.docker.com/reference/compose-file/configs/){target="_blank" rel="noopener noreferrer"} that use `content:` or `environment:` are written into that writable layer after the container is created, and the Engine does not record those paths on inspect.
Watchtower therefore cannot discover them on its own.

The [`com.centurylinklabs.watchtower.copy-file`](../../getting-started/container-selection/index.md#container_labels) label names the paths explicitly. Every named file is copied onto the replacement container.

## Setting the Label

The value is a comma-separated list of absolute paths inside the container.

=== "Docker Compose"

    ```yaml
    services:
        app:
            image: example/app:latest
            configs:
                - source: app_config
                  target: /etc/app/config.yml
            labels:
                - "com.centurylinklabs.watchtower.copy-file=/etc/app/config.yml"

    configs:
        app_config:
            content: |
                listen: 0.0.0.0:8080
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --label=com.centurylinklabs.watchtower.copy-file=/etc/app/config.yml \
        example/app:latest
    ```

=== "Dockerfile"

    ```dockerfile
    LABEL com.centurylinklabs.watchtower.copy-file="/etc/app/config.yml"
    ```

### Several Files at Once

Separate the paths with commas:

```yaml
labels:
    - "com.centurylinklabs.watchtower.copy-file=/etc/app/config.yml,/etc/app/extra.conf"
```

## When the Label Is Required

### Compose Config Sources

| Compose source | How Compose applies it | Watchtower |
| --- | --- | --- |
| [`file:`](https://docs.docker.com/reference/compose-file/configs/#example-1){target="_blank" rel="noopener noreferrer"} | Bind-mount of the host file | Preserved automatically. The label is not needed. |
| [`content:` or `environment:`](https://docs.docker.com/reference/compose-file/configs/){target="_blank" rel="noopener noreferrer"} | Copied in after create | Lost on recreate, unless the label names the in-container path. |

!!! Note "Swarm configs are a separate feature"
    Swarm [Docker configs](https://docs.docker.com/engine/swarm/configs/){target="_blank" rel="noopener noreferrer"} are an Engine feature for swarm services. This label covers standalone containers, including Compose on a non-swarm Engine.

### Targets Inside a Mount Point

A config whose target sits inside a mount point needs the label as well.
This includes a path under a volume that the image declares with `VOLUME`.
Compose writes that file into the mount on every create, and Watchtower now does the same.

## Path Rules

Every labeled path ends in one of three outcomes: copied, skipped, or the update stops.

### Copied

| Condition | Result |
| --- | --- |
| Absolute path with no `..` | Copied onto the replacement container. |
| Parent directory missing from the new image | Watchtower creates it with mode `0755` and writes the file into it. |
| File owner and permissions | The copy keeps the file's numeric UID, GID, and permission bits. See [File Ownership](#file-ownership). |
| Container with a writable root filesystem | Watchtower copies the file, even when the path sits inside a mount. |

### Paths Under a Mount

When a writable mount sits above the labeled path, Watchtower reads the file out of that mount and writes it back into it on every update, so a change made to the mount after the read is overwritten by the older content.
Watchtower reads labeled files while the old container is still running and writes them afterwards, so the two moments are never the same instant.

The write lands in the mount rather than in the container's own filesystem, and removing the container does not undo it.
If the update fails after the file is written, the mount keeps that content even though no container is running.
Use a [named volume](https://docs.docker.com/engine/storage/volumes/){target="_blank" rel="noopener noreferrer"} or a bind mount that you control if another process also writes to the same path, and avoid labeling a file that must never be rewritten.

### Skipped

A skipped path is logged and does not block the update.

| Condition | Why |
| --- | --- |
| The path is itself a mount destination | That mount already provides the file, and it survives recreation. |
| The path is under a read-only mount | The Engine would refuse the write, and that mount provides the file regardless. |
| The path does not exist | There is nothing to copy. |
| The container has a read-only root filesystem | Watchtower extracts at the container root, where the Engine refuses any write. This applies even to a path under a writable volume. |

### Update Stops

Watchtower copies labeled files in two phases and a failure in either one stops that container's update.

**Before the container is stopped.** Watchtower reads every labeled file out of the running container. A failure here leaves the container running and untouched.

| Condition | Why |
| --- | --- |
| The path is not absolute or contains `..` | The label value is invalid. |
| The path is a directory | A directory cannot be copied as a file. |
| The path is a symlink | Symlinks are rejected rather than followed. |
| The file is larger than 1 MiB | The file exceeds the per-file cap. |
| The archive is unsafe | The archive contains traversal entries, extra members, or special file types. |

**After the container is stopped.** Watchtower writes the files into the replacement container. A failure here discards the replacement, and the container is not recreated for that cycle.

| Condition | Why |
| --- | --- |
| The copy into the new container fails | The Engine refused the write, or the connection failed. |
| The daemon remaps user namespaces, the container sets no user, and the file is not owned by root | The Engine cannot map the file's owner into the new container. See [File Ownership](#file-ownership). |

### Size Limits

Each file is capped at **1 MiB**.
Watchtower holds at most **16 MiB** of copy-file data in memory at once, across every container in a single update.

## Volumes

### Named Volumes

A [named volume](https://docs.docker.com/engine/storage/volumes/){target="_blank" rel="noopener noreferrer"} is preserved on recreation, so its contents survive an update on their own.

### Anonymous Volumes

Docker gives each container a fresh anonymous volume for every `VOLUME` an image declares. Watchtower therefore creates a new empty one on each update.
Only the files named by the copy-file label are written back into it. Everything else under that path is discarded.

The [`--remove-volumes`](../../configuration/update-behavior/index.md#remove_anonymous_volumes){target="_blank" rel="noopener noreferrer"} flag deletes the previous anonymous volume after an update, which also removes the labeled files it held.

## How It Works

### Archive Endpoints

Watchtower uses two Docker Engine archive endpoints: [`GET /containers/{id}/archive`](https://docs.docker.com/reference/api/engine/latest/#tag/Container/operation/ContainerArchive){target="_blank" rel="noopener noreferrer"} and [`PUT /containers/{id}/archive`](https://docs.docker.com/reference/api/engine/latest/#tag/Container/operation/PutContainerArchive){target="_blank" rel="noopener noreferrer"}.
The first endpoint snapshots labeled files before the old container is removed. The second endpoint writes them into the new container before it starts.

### Missing Parent Directories

That is the same mechanism [Docker Compose](https://docs.docker.com/reference/compose-file/configs/){target="_blank" rel="noopener noreferrer"} uses to inject `content:` and `environment:` configs.
Both extract the archive at the container root and name the single member with the full in-container path, so the Engine creates any parent directory the new image does not contain.

### File Ownership

Watchtower writes each file with the numeric UID and GID it had in the old container, and with the same permission bits except setuid, setgid, and sticky.
The container's `user:` setting and the image's `USER` do not change the owner.
If a new image switches to a different user, the file keeps its previous owner and mode, which can leave it unreadable to that user.

!!! Note "Daemons with user namespace remapping"
    A daemon started with [`--userns-remap`](https://docs.docker.com/engine/security/userns-remap/){target="_blank" rel="noopener noreferrer"} reports file owners as host IDs when it reads a file, and rejects those IDs when it writes the file back.
    On these daemons Watchtower asks the Engine to give each copied file to the container's configured user and group instead of the original owner.
    Mapping that user into the container requires Docker Engine [29.7.0](https://docs.docker.com/engine/release-notes/29/#2970){target="_blank" rel="noopener noreferrer"} or later.
    Earlier Engines assign the overflow owner `65534:65534` instead.
    A user given by name must exist in the new image.
    A container with no user set receives files as root, so only files owned by root can be copied, and any other owner stops that container's update.
