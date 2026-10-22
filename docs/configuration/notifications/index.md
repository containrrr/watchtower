# Notifications

## Deprecation Notice

!!! Warning "Watchtower v2 Legacy Notification Deprecation"
    Deprecated configuration options will be removed with the release of Watchtower v2.

    Use the the [`NOTIFICATION URL`](#notification_url) with the appropriate Shoutrrr URL scheme instead.

## Notification URL

Configures the notification service URL(s).

```text
             Argument: --notification-url
 Environment Variable: WATCHTOWER_NOTIFICATION_URL
                   Type: String (comma-separated or space-separated)
               Default: None
```

!!! Note "This option can also reference a file, in which case the contents of the file are used."

!!! Note "Multiple Notification URLs"
    To send notifications to multiple services simultaneously, you can:

    - Use comma-separated URLs: `--notification-url="discord://xxx,telegram://yyy"`
    - Specify the flag multiple times: `--notification-url=discord://xxx --notification-url=telegram://yyy`

    See [Configuring Multiple Notification URLs](../../notifications/introduction/index.md#using_multiple_notification_services) for detailed examples.

!!! Note "CLI Flags vs Environment Variables"
    The CLI flag can be called multiple times as CLI arguments; however, defining the environment variable multiple times will NOT work and only the last value will be used.

    This is because CLI flags use a StringArray type that supports multiple invocations,  while environment variables are simple key-value pairs that get overwritten when defined multiple times.
    For environment variables, use comma-separated or space-separated values.

## Notification Split by Container

Send separate notifications for each updated container instead of grouping them.

```text
            Argument: --notification-split-by-container
Environment Variable: WATCHTOWER_NOTIFICATION_SPLIT_BY_CONTAINER
                Type: Boolean
             Default: false
```

!!! Note
    When disabled (default), notifications are grouped for all updated containers in a single session.
    When enabled, a separate notification is sent for each container update.

## Notification Template

Sets the Go template used for formatting notification messages.

```text
            Argument: --notification-template
Environment Variable: WATCHTOWER_NOTIFICATION_TEMPLATE
                Type: String
             Default: See default templates below
```

## Notification Template File

Specifies the path to a file containing the Shoutrrr text/template for notification messages.

```text
             Argument: --notification-template-file
 Environment Variable: WATCHTOWER_NOTIFICATION_TEMPLATE_FILE
                 Type: String
              Default: None
```

!!! Note
    If both the [`notification-template`](#notification_template) and [`notification-template-file`](#notification_template_file) configuration options are specified, then the file-based template takes precedence over the inline template.

!!! Warning "New Feature"
    This is a new feature and is not yet fully documented in the main configuration reference. It is intentionally omitted from the deprecation notice.

## Notification Report

Enables the session report as the notification template data, including container statuses and logs.

```text
            Argument: --notification-report
Environment Variable: WATCHTOWER_NOTIFICATION_REPORT
                Type: Boolean
             Default: false
```

## Notifications Level

The log level used for sending notifications. Possible values: panic, fatal, error, warn, info or debug.

```text
            Argument: --notifications-level
Environment Variable: WATCHTOWER_NOTIFICATIONS_LEVEL
                Type: String
             Default: info
```

## Notifications Hostname

Custom hostname for notification titles.

```text
            Argument: --notifications-hostname
Environment Variable: WATCHTOWER_NOTIFICATIONS_HOSTNAME
                Type: String
             Default: None
```

## Notifications Delay

Delay before sending notifications, expressed in seconds.

```text
            Argument: --notifications-delay
Environment Variable: WATCHTOWER_NOTIFICATIONS_DELAY
                Type: Integer
             Default: None
```

## Notification Title Tag

Title prefix tag for notifications.

```text
            Argument: --notification-title-tag
Environment Variable: WATCHTOWER_NOTIFICATION_TITLE_TAG
                Type: String
             Default: None
```

## Notification Skip Title

Do not pass the title param to notifications.

```text
            Argument: --notification-skip-title
Environment Variable: WATCHTOWER_NOTIFICATION_SKIP_TITLE
                Type: Boolean
             Default: false
```

## Notification Log Stdout

Write notification logs to stdout instead of logging (to stderr).

```text
            Argument: --notification-log-stdout
Environment Variable: WATCHTOWER_NOTIFICATION_LOG_STDOUT
                Type: Boolean
             Default: false
```

## Disable Startup Message

Suppresses the info-level notification sent when Watchtower starts.

```text
            Argument: --no-startup-message
Environment Variable: WATCHTOWER_NO_STARTUP_MESSAGE
                Type: Boolean
             Default: false
```

## Deprecated Configuration Options

/// details | The following legacy configuration options and examples are deprecated and will be removed with the release of Watchtower v2.
    type: warning

### General Options

#### Notifications

Specifies the notification services to use (comma-separated). Legacy providers (email, slack, etc.) are deprecated.

```text
            Argument: --notifications, -n
Environment Variable: WATCHTOWER_NOTIFICATIONS
                Type: String (comma-separated or multiple flags)
             Default: None
```

### SMTP

#### Email From

The e-mail address from which notifications will be sent.

```text
            Argument: --notification-email-from
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_FROM
                Type: String
             Default: None
```

#### Email To

The e-mail address to which notifications will be sent.

```text
            Argument: --notification-email-to
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_TO
                Type: String
             Default: None
```

#### Email Server

The SMTP server (IP or FQDN) to send notifications through.

```text
            Argument: --notification-email-server
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SERVER
                Type: String
             Default: None
```

#### Email Server TLS Skip Verify

Skip verification of the server certificate when using TLS.

```text
            Argument: --notification-email-server-tls-skip-verify
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY
                Type: Boolean
             Default: false
```

#### Email Server User

The username for the SMTP server if it requires authentication.

```text
            Argument: --notification-email-server-user
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER
                Type: String
             Default: None
```

#### Email Server Password

The password for the SMTP server if it requires authentication.

```text
            Argument: --notification-email-server-password
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD
                Type: String
             Default: None
```

!!! Note "This option can also reference a file, in which case the contents of the file are used."

#### Email Subject Tag

Subject prefix tag for notifications via mail.

```text
            Argument: --notification-email-subjecttag
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG
                Type: String
             Default: ""
```

#### Email Server Port

The port the SMTP server listens on.

```text
            Argument: --notification-email-server-port
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT
                Type: Integer
             Default: 25
```

#### Email Delay

The delay (in seconds) between sending notifications if multiple containers are updated at once.

```text
            Argument: --notification-email-delay
Environment Variable: WATCHTOWER_NOTIFICATION_EMAIL_DELAY
                Type: Integer
             Default: None
```

### Gotify

#### Gotify URL

The URL of the Gotify instance.

```text
            Argument: --notification-gotify-url
Environment Variable: WATCHTOWER_NOTIFICATION_GOTIFY_URL
                Type: String
             Default: None
```

#### Gotify Token

Sets the Gotify token for notifications.

```text
            Argument: --notification-gotify-token
Environment Variable: WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN
                Type: String
             Default: None
```

!!! Note "This option can also reference a file, in which case the contents of the file are used."

#### Gotify TLS Skip Verify

Skip verification of the server certificate when using TLS.

```text
            Argument: --notification-gotify-tls-skip-verify
Environment Variable: WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY
                Type: Boolean
             Default: false
```

!!! Warning "Deprecated: Use `disabletls=yes` query parameter in the `gotify://` URL instead."

### Microsoft Teams

#### Microsoft Teams Hook

Sets the Microsoft Teams webhook URL for notifications.

```text
            Argument: --notification-msteams-hook
Environment Variable: WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL
                Type: String
             Default: None
```

!!! Warning
    The value of `--notification-msteams-hook` **must** be an absolute URL using the `https://` scheme (including the host).
    Relative URLs and non-HTTPS schemes are rejected at runtime.

!!! Note "This option can also reference a file, in which case the contents of the file are used."

### Slack

#### Slack Hook URL

Sets the Slack webhook URL for notifications.

```text
            Argument: --notification-slack-hook-url
Environment Variable: WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL
                Type: String
             Default: None
```

#### Slack Identifier

Custom name under which messages are sent.

```text
            Argument: --notification-slack-identifier
Environment Variable: WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER
                Type: String
             Default: watchtower
```

!!! Deprecated
    Use the `botname` query parameter in the `slack://` URL instead.

#### Slack Channel

A Slack channel ID, such as `C0123456789`, which overrides the default channel of a legacy custom-integration webhook (optional).
Slack ignores this override for webhooks created by a Slack app, which always post to the channel chosen when the webhook was created.
A channel name such as `#ops` or a user such as `@alice` cannot be carried into the Shoutrrr URL, so Watchtower logs a warning and posts to the webhook's own channel.

```text
            Argument: --notification-slack-channel
Environment Variable: WATCHTOWER_NOTIFICATION_SLACK_CHANNEL
                Type: String
             Default: None
```

!!! Note "This option can also reference a file, in which case the contents of the file are used."

#### Slack Icon Emoji

An emoji code string to use in place of the default icon.

```text
            Argument: --notification-slack-icon-emoji
Environment Variable: WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI
                Type: String
             Default: None
```

!!! Deprecated
    Use the `icon_emoji` query parameter in the `slack://` or `discord://` URL instead.

#### Slack Icon URL

An icon image URL string to use in place of the default icon.

```text
            Argument: --notification-slack-icon-url
Environment Variable: WATCHTOWER_NOTIFICATION_SLACK_ICON_URL
                Type: String
             Default: None
```

!!! Deprecated
    Use the `icon_url` query parameter in the `slack://` or `discord://` URL instead.

///
