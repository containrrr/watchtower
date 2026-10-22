# Email Notifications

!!! Warning "Deprecated Email Notification Options"
    The following legacy email notification options are deprecated and subject to removal with the release of Watchtower v2:

    - [`notification-email-from`](../../../configuration/notifications/index.md#email_from)
    - [`notification-email-to`](../../../configuration/notifications/index.md#email_to)
    - [`notification-email-server`](../../../configuration/notifications/index.md#email_server)
    - [`notification-email-server-tls-skip-verify`](../../../configuration/notifications/index.md#email_server_tls_skip_verify)
    - [`notification-email-server-user`](../../../configuration/notifications/index.md#email_server_user)
    - [`notification-email-server-password`](../../../configuration/notifications/index.md#email_server_password)
    - [`notification-email-subjecttag`](../../../configuration/notifications/index.md#email_subject_tag)
    - [`notification-email-server-port`](../../../configuration/notifications/index.md#email_server_port)
    - [`notification-email-delay`](../../../configuration/notifications/index.md#email_delay)

    Use the [`notification-url`](../../../configuration/notifications/index.md#notification_url) configuration option with a `smtp://` Shoutrrr URL instead.

    Use the [legacy notifications migration tool](../../deprecations/migration-tool/index.md#smtp_migration_walkthrough) to help with migrating to a Shoutrrr URL.

## Overview

Watchtower uses Shoutrrr's [SMTP service](https://shoutrrr.nickfedor.com/latest/services/email/smtp/){target="_blank" rel="noopener noreferrer"} to send email notifications.

## Examples

=== "Docker Compose"

    ```yaml
    services:
    watchtower:
        image: nickfedor/watchtower:latest
        environment:
        WATCHTOWER_NOTIFICATION_URL: smtp://user:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com
        volumes:
        - /var/run/docker.sock:/var/run/docker.sock
    ```

=== "Docker CLI (Env Vars)"

    ```bash
    docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e WATCHTOWER_NOTIFICATION_URL="smtp://user:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com" \
    nickfedor/watchtower
    ```

=== "Docker CLI (Flags)"

    ```bash
    docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    nickfedor/watchtower \
    --notification-url "smtp://user:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com"
    ```

/// details | The following legacy SMTP configuration examples are deprecated and will be removed with the release of Watchtower v2.
    type: warning

=== "Docker Compose"

    === "Docker Compose"

        ```yaml
        services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
            WATCHTOWER_NOTIFICATIONS: email
            WATCHTOWER_NOTIFICATION_EMAIL_FROM: sender@example.com
            WATCHTOWER_NOTIFICATION_EMAIL_TO: recipient@example.com
            WATCHTOWER_NOTIFICATION_EMAIL_SERVER: smtp.example.com
            WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT: 587
            WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER: user
            WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD: secret
            WATCHTOWER_NOTIFICATION_EMAIL_DELAY: 10
            volumes:
            - /var/run/docker.sock:/var/run/docker.sock
        ```

    === "SMTP Relay"

        ```yaml
        services:
        watchtower:
            image: nickfedor/watchtower:latest
            environment:
            WATCHTOWER_NOTIFICATIONS: email
            WATCHTOWER_NOTIFICATION_EMAIL_FROM: sender@example.com
            WATCHTOWER_NOTIFICATION_EMAIL_TO: recipient@example.com
            WATCHTOWER_NOTIFICATION_EMAIL_SERVER: relay.example.com
            volumes:
            - /var/run/docker.sock:/var/run/docker.sock
        ```

        !!! Note
            The example assumes that your domain is called `example.com` and that you are going to use a valid certificate for `smtp.example.com`.

            This hostname has to be used as `WATCHTOWER_NOTIFICATION_EMAIL_SERVER`, otherwise the TLS connection will fail with `Failed to send notification email` or `connection: connection refused` errors.

            We also have to add a network for this setup in order to add an alias to it.

            If you also want to enable DKIM or other features on the SMTP server, then you will find more information at [freinet/postfix-relay](https://hub.docker.com/r/freinet/postfix-relay){target="_blank" rel="noopener noreferrer"}

=== "Docker CLI"

    === "Env Vars"

        ```bash
        docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -e WATCHTOWER_NOTIFICATIONS=email \
        -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=sender@example.com \
        -e WATCHTOWER_NOTIFICATION_EMAIL_TO=recipient@example.com \
        -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=smtp.example.com \
        -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT=587 \
        -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER=user \
        -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD=secret \
        -e WATCHTOWER_NOTIFICATION_EMAIL_DELAY=10 \
        nickfedor/watchtower
        ```

    === "Flags"

        ```bash
        docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --notifications email \
        --notification-email-from sender@example.com \
        --notification-email-to recipient@example.com \
        --notification-email-server smtp.example.com \
        --notification-email-server-port 587 \
        --notification-email-server-user user \
        --notification-email-server-password secret \
        --notification-email-delay 10
        ```

    === "Env Vars (SMTP Relay)"

        ```bash
        docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -e WATCHTOWER_NOTIFICATIONS=email \
        -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=sender@example.com \
        -e WATCHTOWER_NOTIFICATION_EMAIL_TO=recipient@example.com \
        -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=relay.example.com \
        nickfedor/watchtower
        ```

    === "Flags (SMTP Relay)"

        ```bash
        docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        nickfedor/watchtower \
        --notifications email \
        --notification-email-from sender@example.com \
        --notification-email-to recipient@example.com \
        --notification-email-server relay.example.com
        ```

        !!! Note
            This assumes that you already have an SMTP server up and running that you can connect to.
            If you don't or you want to bring up Watchtower with your own simple SMTP relay, then check out the Docker Compose example.

///

## Common SMTP Configurations

=== "Gmail"

    | Property    | Value         |
    |-------------|---------------|
    | Port        | `587`         |
    | Encryption  | `ExplicitTLS` |
    | UseStartTLS | `Yes`         |

    ```text
    smtp://user%40gmail.com:${PASSWORD}@smtp.gmail.com:587/?fromaddress=user%40gmail.com&toaddresses=user%40gmail.com&encryption=ExplicitTLS&usestarttls=yes&timeout=30s
    ```

    !!! Note
        For Gmail, use an [App Password](https://support.google.com/accounts/answer/185833){target="_blank" rel="noopener noreferrer"} if two-factor authentication is enabled.

        Omit spaces from the App Password. Google displays the 16-character passcode in groups of four, but those spaces are not part of the password.

        URL-encode `@` in the username as `%40`.

        Do not use `&auth=OAuth2` with an App Password. That method expects a SASL XOAUTH2 access token in the password field.

=== "AWS SES"

    | Property    | Value         |
    |-------------|---------------|
    | Port        | `587`         |
    | Encryption  | `ExplicitTLS` |
    | UseStartTLS | `Yes`         |

    ```text
    smtp://${USER}:${PASSWORD}@email-smtp.us-east-1.amazonaws.com:587/?fromaddress=${FROM}&toaddresses=${TO}&encryption=ExplicitTLS&usestarttls=yes&timeout=30s
    ```

=== "Microsoft 365"

    | Property    | Value         |
    |-------------|---------------|
    | Port        | `587`         |
    | Encryption  | `ExplicitTLS` |
    | UseStartTLS | `Yes`         |

    ```text
    smtp://${USER}:${PASSWORD}@smtp.office365.com:587/?fromaddress=${FROM}&toaddresses=${TO}&encryption=ExplicitTLS&usestarttls=yes&timeout=30s
    ```

=== "Generic (SSL)"

    | Property    | Value         |
    |-------------|---------------|
    | Port        | `465`         |
    | Encryption  | `ImplicitTLS` |
    | UseStartTLS | `No`          |

    ```text
    smtp://${USER}:${PASSWORD}@smtp.example.com:465/?fromaddress=${FROM}&toaddresses=${TO}&encryption=ImplicitTLS&usestarttls=no&timeout=30s
    ```

=== "Generic (Plain)"

    | Property    | Value  |
    |-------------|--------|
    | Port        | `25`   |
    | Encryption  | `None` |
    | UseStartTLS | `No`   |

    ```text
    smtp://${USER}:${PASSWORD}@smtp.example.com:25/?fromaddress=${FROM}&toaddresses=${TO}&encryption=None&usestarttls=no&timeout=30s
    ```

## Notes
<!-- markdownlint-disable -->
- **Timeout**:

    * The default SMTP timeout is 10 seconds.
    * If you experience timeouts (e.g., `failed to send: timed out: using smtp`), add `&timeout=30s` to the URL to allow more time for server responses, especially with proxies or slow networks.

- **Authentication**:

    * Use `&auth=Plain` for username and password authentication, including [Gmail App Passwords](https://support.google.com/accounts/answer/185833){target="_blank" rel="noopener noreferrer"}. This is the default when credentials are provided.
    * Use `&auth=OAuth2` only for [SASL XOAUTH2](https://shoutrrr.nickfedor.com/latest/services/email/smtp/#authentication){target="_blank" rel="noopener noreferrer"} with a static access token in the password field. App Passwords are not OAuth2.

- **Testing**:

    * Install Shoutrrr using one of the various [installation methods](https://shoutrrr.nickfedor.com/installation/){target="_blank" rel="noopener noreferrer"}.
    * Test your URL with the Shoutrrr CLI:
      ```bash
      shoutrrr send -u <URL> -m "Test message"
      ```

- **Proxy Issues**:

    * If using a Docker proxy (e.g., `tcp://dockerproxy:2375`), ensure it allows outbound connections to `${SMTP_HOST}:${SMTP_PORT}`.
    * Test connectivity with `telnet ${SMTP_HOST} ${SMTP_PORT}` inside the container.
<!-- markdownlint-restore -->
