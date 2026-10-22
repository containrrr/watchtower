# Docker Secrets

## Overview

Watchtower supports the use of [Docker Secrets](https://docs.docker.com/compose/how-tos/use-secrets/) to provide a way for using sensitive values without exposing them as environment variables.

The following supported configuration options allow for users to reference a filepath instead of directly referencing the secret value (e.g. `WATCHTOWER_HTTP_API_TOKEN=/run/secrets/api_token` instead of `WATCHTOWER_HTTP_API_TOKEN=secret_token`).

Watchtower will then check whether the provided value is a path to an existing file on disk.
Upon successful validation, the contents of the file are read and used as the value instead.

## Supported Configuration Options

| Configuration Option                                                                      | Deprecated |
|-------------------------------------------------------------------------------------------|------------|
| [HTTP API Token](../../configuration/http-api/index.md#http_api_token)                    | No         |
| [HTTP API Events Token](../../configuration/http-api/index.md#http_api_events_token)      | No         |
| [Notification URL](../../configuration/notifications/index.md#notification_url)           | No         |
| [Git Auth Token](../../configuration/git-monitoring/index.md#git_auth_token)              | No         |
| [Git Password](../../configuration/git-monitoring/index.md#git_password)                  | No         |
| [Email Server Password](../../configuration/notifications/index.md#email_server_password) | Yes        |
| [Gotify Token](../../configuration/notifications/index.md#gotify_token)                   | Yes        |
| [Microsoft Teams Hook](../../configuration/notifications/index.md#microsoft_teams_hook)   | Yes        |
| [Slack Hook URL](../../configuration/notifications/index.md#slack_hook_url)               | Yes        |

!!! Warning "Watchtower v2 Legacy Notification Deprecation"
    Deprecated notification configuration options will be removed with the release of Watchtower v2.

    Use the [`NOTIFICATION URL`](../../configuration/notifications/index.md#notification_url) with the appropriate Shoutrrr URL scheme instead.

!!! Note

    - For the [Notification URL](../../configuration/notifications/index.md#notification_url) option, when a value is a path to a file, each non-empty line in the file is treated as a separate notification URL.
    - This file-based support works with any mechanism that can make a file available inside the container at runtime.
    - You specify the path to the file inside the container (e.g. `/run/secrets/http_api_token`).

## Examples

### HTTP API Token

Provide the [HTTP API Token](../../configuration/http-api/index.md#http_api_token) from a file.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            secrets:
                - api_token
            environment:
                - WATCHTOWER_HTTP_API_TOKEN=/run/secrets/api_token
                # Enable an endpoint that requires the token
                - WATCHTOWER_HTTP_API_ENDPOINTS=metrics
            ports:
                - "8080:8080"
            restart: unless-stopped

    secrets:
        api_token:
            file: ./secrets/api_token.txt
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/secrets/api_token.txt:/run/secrets/api_token:ro \
        -e WATCHTOWER_HTTP_API_TOKEN=/run/secrets/api_token \
        -e WATCHTOWER_HTTP_API_ENDPOINTS=metrics \
        -p 8080:8080 \
        --restart unless-stopped \
        nickfedor/watchtower
    ```

### HTTP API Events Token

Provide the [HTTP API Events Token](../../configuration/http-api/index.md#http_api_events_token) from a file.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            secrets:
                - events_token
            environment:
                - WATCHTOWER_HTTP_API_EVENTS_TOKEN=/run/secrets/events_token
                - WATCHTOWER_HTTP_API_ENDPOINTS=events
            ports:
                - "8080:8080"
            restart: unless-stopped

    secrets:
        events_token:
            file: ./secrets/events_token.txt
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/secrets/events_token.txt:/run/secrets/events_token:ro \
        -e WATCHTOWER_HTTP_API_EVENTS_TOKEN=/run/secrets/events_token \
        -e WATCHTOWER_HTTP_API_ENDPOINTS=events \
        -p 8080:8080 \
        --restart unless-stopped \
        nickfedor/watchtower
    ```

### Notification URL

Provide the [Notification URL](../../configuration/notifications/index.md#notification_url) value(s) from a file. The file may contain one or more Shoutrrr URLs (one per line).

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            secrets:
                - notification_url
            environment:
                - WATCHTOWER_NOTIFICATION_URL=/run/secrets/notification_url
            restart: unless-stopped

    secrets:
        notification_url:
            file: ./secrets/notification_url.txt
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/secrets/notification_url.txt:/run/secrets/notification_url:ro \
        -e WATCHTOWER_NOTIFICATION_URL=/run/secrets/notification_url \
        --restart unless-stopped \
        nickfedor/watchtower
    ```

### Git Auth Token

Provide the [Git Auth Token](../../configuration/git-monitoring/index.md#git_auth_token) from a file.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            secrets:
                - git_auth_token
            environment:
                - WATCHTOWER_GIT_ENABLE=true
                - WATCHTOWER_GIT_AUTH_TOKEN=/run/secrets/git_auth_token
            restart: unless-stopped

    secrets:
        git_auth_token:
            file: ./secrets/git_auth_token.txt
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/secrets/git_auth_token.txt:/run/secrets/git_auth_token:ro \
        -e WATCHTOWER_GIT_ENABLE=true \
        -e WATCHTOWER_GIT_AUTH_TOKEN=/run/secrets/git_auth_token \
        --restart unless-stopped \
        nickfedor/watchtower
    ```

### Git Password

Provide the [Git Password](../../configuration/git-monitoring/index.md#git_password) from a file.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
            secrets:
                - git_password
            environment:
                - WATCHTOWER_GIT_ENABLE=true
                - WATCHTOWER_GIT_USERNAME=git
                - WATCHTOWER_GIT_PASSWORD=/run/secrets/git_password
            restart: unless-stopped

    secrets:
        git_password:
            file: ./secrets/git_password.txt
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v $(pwd)/secrets/git_password.txt:/run/secrets/git_password:ro \
        -e WATCHTOWER_GIT_ENABLE=true \
        -e WATCHTOWER_GIT_USERNAME=git \
        -e WATCHTOWER_GIT_PASSWORD=/run/secrets/git_password \
        --restart unless-stopped \
        nickfedor/watchtower
    ```
