# Slack Notifications

!!! Warning "Deprecated Slack Notification Options"
    The following legacy Slack notification options are deprecated and subject to removal with the release of Watchtower v2:

    - [`notification-slack-hook-url`](../../../configuration/notifications/index.md#slack_hook_url)
    - [`notification-slack-identifier`](../../../configuration/notifications/index.md#slack_identifier)
    - [`notification-slack-channel`](../../../configuration/notifications/index.md#slack_channel)

    Use the [`notification-url`](../../../configuration/notifications/index.md#notification_url) configuration option with a `slack://` Shoutrrr URL instead.

## Overview

Watchtower can use Shoutrrr's [Slack service](https://shoutrrr.nickfedor.com/latest/services/chat/slack/){target="_blank" rel="noopener noreferrer"} to send [Slack](https://slack.com/){target="_blank" rel="noopener noreferrer"} notifications.

## Examples

=== "Docker Compose"

    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower:latest
        environment:
          WATCHTOWER_NOTIFICATION_URL: "slack://hook:AAAA-BBBB-CCCC@webhook?botname=watchtower"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      nickfedor/watchtower \
      --notification-url "slack://hook:AAAA-BBBB-CCCC@webhook?botname=watchtower"
    ```

/// details | The following legacy Slack configuration options and examples are deprecated and will be removed with the release of Watchtower v2.
    type: warning

=== "Docker CLI (Env Vars)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e WATCHTOWER_NOTIFICATIONS=slack \
      -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
      -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
      -e WATCHTOWER_NOTIFICATION_SLACK_CHANNEL=C0123456789 \
      nickfedor/watchtower
    ```

=== "Docker CLI (Flags)"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      nickfedor/watchtower \
      --notifications slack \
      --notification-slack-hook-url "https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
      --notification-slack-identifier watchtower-server-1 \
      --notification-slack-channel "C0123456789"
    ```

=== "Docker Compose"

    ```yaml
    services:
      watchtower:
        image: nickfedor/watchtower:latest
        environment:
          WATCHTOWER_NOTIFICATIONS: slack
          WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL: "https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy"
          WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER: watchtower-server-1
          WATCHTOWER_NOTIFICATION_SLACK_CHANNEL: "C0123456789"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```

///
