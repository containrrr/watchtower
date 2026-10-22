# Synology Pre-Update Lifecycle Hook
>
> [!Warning]
>
> This is an untested example that requires further testing and validation.

## Overview

This directory provides an example of a [pre-update lifecycle hook](https://watchtower.nickfedor.com/latest/advanced-features/lifecycle-hooks/) with a focus on providing container shutdown functionality for Synology devices via Synology's built-in tooling.

Both the shell script and Go implementations enable the **graceful shutdown** of Docker containers on **Synology DSM** using the DSM Web API, preventing abrupt `SIGKILL` signals, and allowing containers to shut down cleanly (e.g., saving state, closing connections, etc).

Watchtower adds the `WT_CONTAINER` environment variable, a JSON object that can be parsed for container information, when invoking the hook. This is accessible via the monitored container that's being updated.

The Go implementation is provided as an alternative to shell scripting due to the limitations of available tooling in target containers (e.g., `jq` may not be available for parsing JSON).

The example hook itself is relatively simple. It performs the following actions:

- Authenticates via DSM login API.
- Gracefully stops the target container using the Docker API.
- Logs out to invalidate the session.

## Files

| File                                                                 | Description                             |
|----------------------------------------------------------------------|-----------------------------------------|
| [`synology-stop.go`](synology-stop.go)                               | Go binary                               |
| [`synology-stop.sh`](synology-stop.sh)                               | shell script                            |
| [`test_synology_parsing.sh`](test_synology_parsing.sh)               | Unit tests for the shell script         |
| [`DSM_Login_Web_API_Guide_enu.pdf`](DSM_Login_Web_API_Guide_enu.pdf) | Official Synology Web API documentation |

## Environment Variables

| Variable            | Description                                      | Default | Required? |
|---------------------|--------------------------------------------------|---------|-----------|
| `SYNO_URL`          | DSM base URL (e.g., `http://192.168.1.100:5000`) | -       | Yes       |
| `SYNO_USER`         | DSM username **with Docker permissions**         | -       | Yes       |
| `SYNO_PASS`         | DSM password                                     | -       | Yes       |
| `CLIENT_TIMEOUT`    | HTTP timeout (seconds)                           | `30`    | No        |
| `CLIENT_SSL_VERIFY` | TLS verification enabled (1=true, 0=skip)        | `1`     | No        |

> [!NOTE]
>
> - Environment variables must be set in the **monitored container** (the one being updated), **not** in Watchtower.
> - Set `CLIENT_SSL_VERIFY=0` for self-signed certs.
> - Increase `CLIENT_TIMEOUT` as needed to accommodate low-power Synology devices.

## Build

### Go Binary

```bash
cd examples/lifecycle-hooks/synology-stop
GOOS=linux GOARCH=arm64 go build -o synology-stop synology-stop.go
```

### Shell Script

```bash
chmod +x synology-stop.sh
```

## Example Deployment

> [!NOTE]
>
> This uses the Go implementation.

`docker-compose.yaml`:

```yaml
services:
    watchtower:
        image: nicholas-fedor/watchtower
        volumes:
            - /var/run/docker.sock:/var/run/docker.sock
        environment:
            WATCHTOWER_LIFECYCLE_HOOKS: true
        restart: unless-stopped

    nginx:
        image: nginx:latest
        volumes:
            - ./Scripts/synology-stop:/synology-stop:ro
        ports:
            - 8080:80
        labels:
            - com.centurylinklabs.watchtower.lifecycle.pre-update: "/synology-stop"
        environment:
            - SYNO_URL=http://192.168.1.100:5000
            - SYNO_USER=watchtower
            - SYNO_PASS=supersecretpassword
```

## Integration

### Watchtower

- Runs [`pre-update hook`](https://watchtower.nickfedor.com/latest/advanced-features/lifecycle-hooks/) **before** update/stop.
- Non-0 exit warns but continues.

### Synology DSM

- User needs **Docker** permissions.
- API flow:
    1. Login (`/webapi/auth.cgi`)
    2. Docker stop (`/webapi/Docker/engine.cgi?method=stop`)
    3. Logout
- See [`DSM_Login_Web_API_Guide_enu.pdf`](DSM_Login_Web_API_Guide_enu.pdf): §2.1 (workflow), §3.2 (auth).

## Usage Example

1. Build hook binary:

   ```bash
   go build -o synology-stop synology-stop.go && chmod +x synology-stop
   ```

2. Run Watchtower with the hook (see [example deployment](#example-deployment)).
3. Trigger or wait for scheduled update:

   ```bash
   docker run --rm nicholas-fedor/watchtower --run-once nginx
   ```

4. Check logs (expect *"Container stopped gracefully"*):

   ```bash
   docker logs watchtower
   ```

## Troubleshooting

### Common Errors

| Code  | Meaning                                  | Resolution                                                                  |
|-------|------------------------------------------|-----------------------------------------------------------------------------|
| `101` | No valid session (`sid` missing/expired) | Verify login creds; check `SYNO_*` env vars **in the monitored container**. |
| `400` | Bad Request                              | Invalid API params (e.g., wrong `WT_CONTAINER`, API version).               |
| `401` | Unauthorized                             | Wrong `SYNO_USER`/`SYNO_PASS`.                                              |
| `403` | Forbidden                                | User lacks Docker perms (DSM → User → Apps → Docker).                       |
| `404` | Not Found                                | Container ID invalid/missing.                                               |
| `410` | Gone                                     | Session expired; ensure logout not premature.                               |

| `timeout` | API request timeout | Increase `CLIENT_TIMEOUT` (Go) / `CURL_TIMEOUT` (sh). |

### Debug Steps

1. **Test login manually**:

   ```bash
   curl -k -d "account=youruser&pass=yourpass&session=Web" "${SYNO_URL}/webapi/auth.cgi?api=SYNO.API.Auth&method=login&version=6"
   ```

   Expect `{"success":true,"data":{"sid":"abc","session":"Web"}}`.

2. **Hook logs**: `docker logs watchtower` (look for API responses).

3. **Run standalone**:

   ```bash
   WT_CONTAINER='{"name":"abc123"}' SYNO_URL=... ./synology-stop
   ```

4. **Verify tests**: `./test_synology_parsing.sh`.

5. **Go vet**: `go vet ./...` (no issues expected).

## References

- [Watchtower Documentation](https://watchtower.nickfedor.com)
- [Synology DSM Login Web API](DSM_Login_Web_API_Guide_enu.pdf)
- [Synology Docker API](https://<your-nas>:5000/webman/login.cgi) (login, inspect Network → DSM Help).
