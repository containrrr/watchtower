# Signal Notifications

## Overview

Watchtower uses Shoutrrr's [Signal service](https://shoutrrr.nickfedor.com/services/signal/){target="_blank" rel="noopener noreferrer"} to send Signal notifications.

Signal notifications require a Signal API server that can send messages on behalf of a registered Signal account.
This is typically done using [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api){target="_blank" rel="noopener noreferrer"} or [secured-signal-api](https://github.com/codeshelldev/secured-signal-api){target="_blank" rel="noopener noreferrer"}.

## Examples

=== "Docker Compose"

    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower:latest
        environment:
          WATCHTOWER_NOTIFICATION_URL: signal://localhost:8080/+1234567890/+0987654321
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```

=== "Docker CLI (Flags)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      nickfedor/watchtower \
      --notification-url "signal://localhost:8080/+1234567890/+0987654321"
    ```

=== "Docker CLI (Env Vars)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e WATCHTOWER_NOTIFICATION_URL=signal://localhost:8080/+1234567890/+0987654321 \
      nickfedor/watchtower
    ```

## Setting up a Signal API Server

1. **Phone Number**: A dedicated phone number registered with Signal
2. **API Server**: A server running signal-cli with REST API capabilities
3. **Account Linking**: Linking the server as a secondary device to your Signal account
4. **Optional Security Layer**: Authentication and endpoint restrictions via a proxy

The server must be able to receive SMS verification codes during initial setup and maintain a persistent connection to Signal's servers.

## Shoutrrr's Signal URL Format

```text
signal://[user:password@]host:port/source_phone/recipient1/recipient2
```

### Parameters

- `host`: Signal API server hostname or IP address
- `port`: Signal API server port (default: 8080)
- `user`: Username for HTTP Basic Authentication (optional)
- `password`: Password for HTTP Basic Authentication (optional)
- `source_phone`: Your Signal phone number with country code (e.g., +1234567890)
- `recipient1, recipient2`: Phone numbers or group IDs to send to

### TLS Configuration

- Use `signal://` for HTTPS (default, recommended)
- Use `signal://...?disabletls=yes` for HTTP (insecure, for local testing only)

### Attachments

The Signal service supports sending base64-encoded attachments:

```bash
shoutrrr send "signal://localhost:8080/+1234567890/+0987654321" \
  "Message with attachment" \
  --attachments "base64data1,base64data2"
```

!!! Note
    Attachments must be provided as base64-encoded data. The API server handles MIME type detection and file handling.

### Examples

Send to a single phone number:

```
signal://localhost:8080/+1234567890/+0987654321
```

Send to multiple recipients:

```
signal://localhost:8080/+1234567890/+0987654321/+1123456789/group.testgroup
```

Send to a group:

```
signal://localhost:8080/+1234567890/group.abcdefghijklmnop=
```

With authentication:

```
signal://user:password@localhost:8080/+1234567890/+0987654321
```

With API token (Bearer auth):

```
signal://localhost:8080/+1234567890/+0987654321?token=YOUR_API_TOKEN
```

Using HTTP instead of HTTPS:

```
signal://localhost:8080/+1234567890/+0987654321?disabletls=yes
```
