# Gotify Notifications

!!! Warning "Deprecated Gotify Notification Options"
    The following legacy Gotify notification options are deprecated and subject to removal with the release of Watchtower v2:

    - [`notification-gotify-url`](../../../configuration/notifications/index.md#gotify_url)
    - [`notification-gotify-token`](../../../configuration/notifications/index.md#gotify_token)
    - [`notification-gotify-tls-skip-verify`](../../../configuration/notifications/index.md#gotify_tls_skip_verify)

    Use the [`notification-url`](../../../configuration/notifications/index.md#notification_url) configuration option with a `gotify://` Shoutrrr URL instead.

## Overview

Watchtower can use Shoutrrr's [Gotify service](https://shoutrrr.nickfedor.com/latest/services/push/gotify/){target="_blank" rel="noopener noreferrer"} to send [Gotify](https://gotify.net/){target="_blank" rel="noopener noreferrer"} notifications.

## Examples

=== "Docker Compose"

    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower:latest
        environment:
          WATCHTOWER_NOTIFICATION_URL: "gotify://my.gotify.tld/SuperSecretToken"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      nickfedor/watchtower \
      --notification-url "gotify://my.gotify.tld/SuperSecretToken"
    ```

/// details | The following legacy Gotify configuration examples are deprecated and will be removed with the release of Watchtower v2.
    type: warning

=== "Docker Compose"

    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower:latest
        environment:
          WATCHTOWER_NOTIFICATIONS: gotify
          WATCHTOWER_NOTIFICATION_GOTIFY_URL: "https://my.gotify.tld/"
          WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN: "SuperSecretToken"
          WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY: true
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```

=== "Docker CLI (Env Vars)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e WATCHTOWER_NOTIFICATIONS=gotify \
      -e WATCHTOWER_NOTIFICATION_GOTIFY_URL="https://my.gotify.tld/" \
      -e WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN="SuperSecretToken" \
      -e WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY=true \
      nickfedor/watchtower
    ```

=== "Docker CLI (Flags)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      nickfedor/watchtower \
      --notifications gotify \
      --notification-gotify-url "https://my.gotify.tld/" \
      --notification-gotify-token "SuperSecretToken" \
      --notification-gotify-tls-skip-verify
    ```
///
