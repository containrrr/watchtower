# Registry Mirrors

Registry mirrors provide alternative locations for pulling Docker images, useful when access to the default registry is slow, unreliable, or restricted by network policy.
Watchtower uses these mirrors when checking whether containers need updates, ensuring it can detect new images even when the primary registry is unreachable.

## Overview

When the Docker daemon is configured with registry mirrors, Watchtower automatically detects and uses them during its update checks.
This means:

- **Digest comparisons** — For Docker Hub images, Watchtower fetches manifests from mirrors before falling back to the canonical registry, so it can detect updates even when Docker Hub is inaccessible.
- **Docker Hub only** — Daemon `registry-mirrors` apply to Docker Hub (`docker.io`, `index.docker.io`, and bare image names). GHCR, `lscr.io`, and other registries are checked on their canonical host.

!!! Note
    Mirror support in Watchtower covers **digest comparison only** — determining whether a newer image exists.
    The actual image pull is handled by the Docker daemon, which already uses configured mirrors natively.

## Configuring Registry Mirrors

Registry mirrors are configured in the Docker daemon, not in Watchtower itself.

### Configuration File

- **Linux**: `/etc/docker/daemon.json`
- **Windows**: `C:\ProgramData\docker\config\daemon.json`
- **Rootless mode**: `~/.config/docker/daemon.json`

After making changes, restart the daemon:

```bash title="Linux with systemd"
sudo systemctl restart docker
```

### Configuration Format

Docker's `registry-mirrors` setting is a Docker Hub mirror list.
Add Hub mirrors under the `registry-mirrors` key:

```json title="/etc/docker/daemon.json"
{
    "registry-mirrors": [
        "https://mirror.example.com"
    ]
}
```

When multiple mirrors are listed, they are tried in order until one succeeds.

### Command-Line Configuration

Mirrors can also be set via the `dockerd` command line:

```bash
dockerd --registry-mirror https://mirror.example.com
```

## How Watchtower Uses Mirrors

When checking if a Docker Hub image has been updated, Watchtower resolves the mirror to use in this order:

1. **Configured mirrors** — The daemon mirror list is tried first.
2. **Canonical registry** — If all mirrors fail without a rate-limit (429) response, Watchtower falls back to `index.docker.io`. A 429 stops the check instead of trying Docker Hub.

The first mirror to successfully respond with the image manifest wins.
This means a fast, nearby mirror is preferred over Docker Hub.

Images on other registries, such as `ghcr.io` and `lscr.io`, skip the mirror list and use their own host.

## Configuration Examples

### Single Global Mirror

A single mirror used for Docker Hub:

```json title="/etc/docker/daemon.json"
{
    "registry-mirrors": [
        "https://docker-mirror.company.com"
    ]
}
```

### Multiple Mirrors with Redundancy

Mirrors are tried in the order listed.
If the first is unreachable, the next is used:

```json title="/etc/docker/daemon.json"
{
    "registry-mirrors": [
        "https://primary-mirror.company.com",
        "https://backup-mirror.company.com"
    ]
}
```

### Corporate Proxy Environment

When using internal mirrors that may use self-signed certificates:

```json title="/etc/docker/daemon.json"
{
    "registry-mirrors": [
        "https://harbor.internal.company.com"
    ],
    "insecure-registries": [
        "harbor.internal.company.com"
    ]
}
```

## Troubleshooting

### Mirrors Not Being Used

1. **Verify daemon configuration** — Confirm that `/etc/docker/daemon.json` contains valid JSON and the Docker daemon has been restarted since the last change.
2. **Enable debug logging** — Run Watchtower with debug logging to see mirror resolution in action:

    ```bash
    docker run -d --name watchtower \
    -e WATCHTOWER_DEBUG=true \
    -v /var/run/docker.sock:/var/run/docker.sock \
    nickfedor/watchtower
    ```

    Look for log output containing `Resolved registry mirror configuration`.
    A non-Hub image logs `Skipping Docker Hub registry mirrors for non-Hub image` and is not a misconfiguration.
3. **Check network access** — Ensure the host can reach the mirror URLs (e.g., `curl -I https://mirror.example.com/v2/`).
4. **Verify mirror content** — Confirm the mirror is operational and has the required images.

### Authentication Issues

If a mirror requires authentication, configure credentials in the Docker daemon (e.g., `docker login`).
Watchtower uses the same credentials for mirrors as for the canonical registry.

### SSL/TLS Errors

For mirrors using self-signed or internal CA certificates, add them to the `insecure-registries` list:

```json
{
  "insecure-registries": ["internal-mirror.company.com"]
}
```

!!! Warning
    Insecure registries accept HTTPS with untrusted certificates.
    Only use this for internal mirrors under your control.

## Related Features

- [Private Registries](../../advanced-features/private-registries/index.md) — Authenticating with private image registries
- [Secure Connections](../../advanced-features/secure-connections/index.md) — TLS and certificate configuration
