# Deprecation Notice

## Overview

Watchtower has a number of legacy notification configuration options that will be removed with the release of Watchtower v2.

This deprecation and eventual removal of legacy notification configuration options is a long-term migration that started with the refactoring of Shoutrrr into a separate notification library. The removal of the legacy notification configuration options is simply a removal of an unnecessary abstraction layer in favor of using Shoutrrr URL's directly.

In order to continue receiving notifications, users will need to add Shoutrrr URL's directly using the [`NOTIFICATION URL`](../../../configuration/notifications/index.md#notification_url) configuration option.

Watchtower users do not need to install any new software to implement Shoutrrr URL's.

Use Watchtower's CLI [`migration tool`](../migration-tool/index.md) to help convert legacy email configurations to Shoutrrr URLs or use the [Shoutrrr Playground](https://shoutrrr.nickfedor.com/latest/playground/){target="_blank" rel="noopener noreferrer"} to help convert configurations for other services to Shoutrrr URLs.

## General Options

- [`watchtower-notifications`](../../../configuration/notifications/index.md#notifications_1)

## [Email Notifications](../email/index.md)

- [`notification-email-from`](../../../configuration/notifications/index.md#email_from)
- [`notification-email-to`](../../../configuration/notifications/index.md#email_to)
- [`notification-email-server`](../../../configuration/notifications/index.md#email_server)
- [`notification-email-server-tls-skip-verify`](../../../configuration/notifications/index.md#email_server_tls_skip_verify)
- [`notification-email-server-user`](../../../configuration/notifications/index.md#email_server_user)
- [`notification-email-server-password`](../../../configuration/notifications/index.md#email_server_password)
- [`notification-email-subjecttag`](../../../configuration/notifications/index.md#email_subject_tag)
- [`notification-email-server-port`](../../../configuration/notifications/index.md#email_server_port)
- [`notification-email-delay`](../../../configuration/notifications/index.md#email_delay)

## [Gotify Notifications](../gotify/index.md)

- [`notification-gotify-url`](../../../configuration/notifications/index.md#gotify_url)
- [`notification-gotify-token`](../../../configuration/notifications/index.md#gotify_token)
- [`notification-gotify-tls-skip-verify`](../../../configuration/notifications/index.md#gotify_tls_skip_verify)

## [Microsoft Teams Notifications](../teams/index.md)

- [`notification-msteams-hook`](../../../configuration/notifications/index.md#microsoft_teams_hook)

## [Slack Notifications](../slack/index.md)

- [`notification-slack-hook-url`](../../../configuration/notifications/index.md#slack_hook_url)
- [`notification-slack-identifier`](../../../configuration/notifications/index.md#slack_identifier)
- [`notification-slack-channel`](../../../configuration/notifications/index.md#slack_channel)
- [`notification-slack-icon-emoji`](../../../configuration/notifications/index.md#slack_icon_emoji)
- [`notification-slack-icon-url`](../../../configuration/notifications/index.md#slack_icon_url)
