# Notifications

Watchtower can send notifications when containers are updated. Notifications are sent via hooks in the logging
system, [logrus](http://github.com/sirupsen/logrus). 

!!! note "Using multiple notifications with environment variables"
    There is currently a bug in Viper (https://github.com/spf13/viper/issues/380), which prevents comma-separated slices to
    be used when using the environment variable.  
    A workaround is available where we instead put quotes around the environment variable value and replace the commas with
    spaces:
    ```
    WATCHTOWER_NOTIFICATIONS="slack msteams"
    ```
    If you're a `docker-compose` user, make sure to specify environment variables' values in your `.yml` file without double
    quotes (`"`). This prevents unexpected errors when watchtower starts.

## Settings

-   `--notifications-level` (env. `WATCHTOWER_NOTIFICATIONS_LEVEL`): Controls the log level which is used for the notifications. If omitted, the default log level is `info`. Possible values are: `panic`, `fatal`, `error`, `warn`, `info`, `debug` or `trace`.
-   `--notifications-hostname` (env. `WATCHTOWER_NOTIFICATIONS_HOSTNAME`): Custom hostname specified in subject/title. Useful to override the operating system hostname.
-   `--notifications-delay` (env. `WATCHTOWER_NOTIFICATIONS_DELAY`): Delay before sending notifications expressed in seconds.
-   Watchtower will post a notification every time it is started. This behavior [can be changed](https://containrrr.github.io/watchtower/arguments/#without_sending_a_startup_message) with an argument.
-   `--notification-title-tag` (env. `WATCHTOWER_NOTIFICATION_TITLE_TAG`): Prefix to include in the title. Useful when running multiple watchtowers.
-   `--notification-skip-title` (env. `WATCHTOWER_NOTIFICATION_SKIP_TITLE`): Do not pass the title param to notifications. This will not pass a dynamic title override to notification services. If no title is configured for the service, it will remove the title all together.
-   `--notification-log-stdout` (env. `WATCHTOWER_NOTIFICATION_LOG_STDOUT`): Enable output from `logger://` shoutrrr service to stdout.

## [Shoutrrr](https://github.com/containrrr/shoutrrr) notifications

To send notifications via shoutrrr, the following command-line options, or their corresponding environment variables, can be set:

-   `--notification-url` (env. `WATCHTOWER_NOTIFICATION_URL`): The shoutrrr service URL to be used.  This option can also reference a file, in which case the contents of the file are used.


Go to [containrrr.dev/shoutrrr/v0.8/services/overview](https://containrrr.dev/shoutrrr/v0.8/services/overview) to
learn more about the different service URLs you can use. You can define multiple services by space separating the
URLs. (See example below)

You can customize the message posted by setting a template.

-   `--notification-template` (env. `WATCHTOWER_NOTIFICATION_TEMPLATE`): The template used for the message.

The template is a Go [template](https://golang.org/pkg/text/template/) that either format a list
of [log entries](https://pkg.go.dev/github.com/sirupsen/logrus?tab=doc#Entry) or a `notification.Data` struct.

Simple templates are used unless the `notification-report` flag is specified:

-   `--notification-report` (env. `WATCHTOWER_NOTIFICATION_REPORT`): Use the session report as the notification template data.

## Simple templates

The default value if not set is `{{range .}}{{.Message}}{{println}}{{end}}`. The example below uses a template that also
outputs timestamp and log level.

!!! tip "Custom date format"
    If you want to adjust the date/time format it must show how the
    [reference time](https://golang.org/pkg/time/#pkg-constants) (_Mon Jan 2 15:04:05 MST 2006_) would be displayed in your
    custom format.  
    i.e., The day of the year has to be 1, the month has to be 2 (february), the hour 3 (or 15 for 24h time) etc.

!!! note "Skipping notifications"
    To skip sending notifications that do not contain any information, you can wrap your template with `{{if .}}` and `{{end}}`.


Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATION_URL="discord://token@channel slack://watchtower@token-a/token-b/token-c" \
  -e WATCHTOWER_NOTIFICATION_TEMPLATE="{{range .}}{{.Time.Format \"2006-01-02 15:04:05\"}} ({{.Level}}): {{.Message}}{{println}}{{end}}" \
  containrrr/watchtower
```

## Report templates

The default template for report notifications are the following:
```go
{{- if .Report -}}
  {{- with .Report -}}
    {{- if ( or .Updated .Failed ) -}}
{{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed
      {{- range .Updated}}
- {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
      {{- end -}}
      {{- range .Fresh}}
- {{.Name}} ({{.ImageName}}): {{.State}}
	  {{- end -}}
	  {{- range .Skipped}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
	  {{- end -}}
	  {{- range .Failed}}
- {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
	  {{- end -}}
    {{- end -}}
  {{- end -}}
{{- else -}}
  {{range .Entries -}}{{.Message}}{{"\n"}}{{- end -}}
{{- end -}}
```

It will be used to send a summary of every session if there are any containers that were updated or which failed to update.

!!! note "Skipping notifications"
    Whenever the result of applying the template results in an empty string, no notifications will
    be sent. This is by default used to limit the notifications to only be sent when there something noteworthy occurred.

    You can replace `{{- if ( or .Updated .Failed ) -}}` with any logic you want to decide when to send the notifications.

Example using a custom report template that always sends a session report after each run:

=== "docker run"

    ```bash
    docker run -d \
      --name watchtower \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e WATCHTOWER_NOTIFICATION_REPORT="true" \
      -e WATCHTOWER_NOTIFICATION_URL="discord://token@channel slack://watchtower@token-a/token-b/token-c" \
      -e WATCHTOWER_NOTIFICATION_TEMPLATE="
      {{- if .Report -}}
        {{- with .Report -}}
      {{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed
            {{- range .Updated}}
      - {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
            {{- end -}}
            {{- range .Fresh}}
      - {{.Name}} ({{.ImageName}}): {{.State}}
          {{- end -}}
          {{- range .Skipped}}
      - {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
          {{- end -}}
          {{- range .Failed}}
      - {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
          {{- end -}}
        {{- end -}}
      {{- else -}}
        {{range .Entries -}}{{.Message}}{{\"\n\"}}{{- end -}}
      {{- end -}}
      " \
      containrrr/watchtower
    ```

=== "docker-compose"

    ``` yaml
    version: "3"
    services:
      watchtower:
        image: containrrr/watchtower
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
        env:
          WATCHTOWER_NOTIFICATION_REPORT: "true"
          WATCHTOWER_NOTIFICATION_URL: >
            discord://token@channel
            slack://watchtower@token-a/token-b/token-c
          WATCHTOWER_NOTIFICATION_TEMPLATE: |
            {{- if .Report -}}
              {{- with .Report -}}
            {{len .Scanned}} Scanned, {{len .Updated}} Updated, {{len .Failed}} Failed
                  {{- range .Updated}}
            - {{.Name}} ({{.ImageName}}): {{.CurrentImageID.ShortID}} updated to {{.LatestImageID.ShortID}}
                  {{- end -}}
                  {{- range .Fresh}}
            - {{.Name}} ({{.ImageName}}): {{.State}}
                {{- end -}}
                {{- range .Skipped}}
            - {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
                {{- end -}}
                {{- range .Failed}}
            - {{.Name}} ({{.ImageName}}): {{.State}}: {{.Error}}
                {{- end -}}
              {{- end -}}
            {{- else -}}
              {{range .Entries -}}{{.Message}}{{"\n"}}{{- end -}}
            {{- end -}}
    ```

## Legacy notifications

For backwards compatibility, the notifications can also be configured using legacy notification options. These will automatically be converted to shoutrrr URLs when used.  
The types of notifications to send are set by passing a comma-separated list of values to the `--notifications` option
(or corresponding environment variable `WATCHTOWER_NOTIFICATIONS`), which has the following valid values:

-   `email` to send notifications via e-mail
-   `slack` to send notifications through a Slack webhook
-   `msteams` to send notifications via MSTeams webhook
-   `gotify` to send notifications via Gotify

### `notify-upgrade`
If watchtower is started with `notify-upgrade` as it's first argument, it will generate a .env file with your current legacy notification options converted to shoutrrr URLs.

=== "docker run"

    ```bash
    $ docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -e WATCHTOWER_NOTIFICATIONS=slack \
    -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
    containrrr/watchtower \
    notify-upgrade
    ```

=== "docker-compose.yml"

    ```yaml
    version: "3"
    services:
      watchtower:
        image: containrrr/watchtower
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
        env:
          WATCHTOWER_NOTIFICATIONS: slack
          WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL: https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy
        command: notify-upgrade
    ```


You can then copy this file from the container (a message with the full command to do so will be logged) and use it with your current setup:

=== "docker run"

    ```bash
    $ docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    --env-file watchtower-notifications.env \
    containrrr/watchtower
    ```

=== "docker-compose.yml"

    ```yaml
    version: "3"
    services:
      watchtower:
        image: containrrr/watchtower
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
        env_file:
          - watchtower-notifications.env
    ```

### Email

To receive notifications by email, the following command-line options, or their corresponding environment variables, can be set:

-   `--notification-email-from` (env. `WATCHTOWER_NOTIFICATION_EMAIL_FROM`): The e-mail address from which notifications will be sent.
-   `--notification-email-to` (env. `WATCHTOWER_NOTIFICATION_EMAIL_TO`): The e-mail address to which notifications will be sent.
-   `--notification-email-server` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER`): The SMTP server to send e-mails through.
-   `--notification-email-server-tls-skip-verify` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY`): Do not verify the TLS certificate of the mail server. This should be used only for testing.
-   `--notification-email-server-port` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT`): The port used to connect to the SMTP server to send e-mails through. Defaults to `25`.
-   `--notification-email-server-user` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER`): The username to authenticate with the SMTP server with.
-   `--notification-email-server-password` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD`): The password to authenticate with the SMTP server with. Can also reference a file, in which case the contents of the file are used.
-   `--notification-email-delay` (env. `WATCHTOWER_NOTIFICATION_EMAIL_DELAY`): Delay before sending notifications expressed in seconds.
-   `--notification-email-subjecttag` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SUBJECTTAG`): Prefix to include in the subject tag. Useful when running multiple watchtowers. **NOTE:** This will affect all notification types.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=email \
  -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=fromaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_TO=toaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=smtp.gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT=587 \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER=fromaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD=app_password \
  -e WATCHTOWER_NOTIFICATION_EMAIL_DELAY=2 \
  containrrr/watchtower
```

The previous example assumes, that you already have an SMTP server up and running you can connect to. If you don't or you want to bring up watchtower with your own simple SMTP relay the following `docker-compose.yml` might be a good start for you.

The following example assumes, that your domain is called `your-domain.com` and that you are going to use a certificate valid for `smtp.your-domain.com`. This hostname has to be used as `WATCHTOWER_NOTIFICATION_EMAIL_SERVER` otherwise the TLS connection is going to fail with `Failed to send notification email` or `connect: connection refused`. We also have to add a network for this setup in order to add an alias to it. If you also want to enable DKIM or other features on the SMTP server, you will find more information at [freinet/postfix-relay](https://hub.docker.com/r/freinet/postfix-relay).

Example including an SMTP relay:

```yaml
version: '3.8'
services:
  watchtower:
    image: containrrr/watchtower:latest
    container_name: watchtower
    environment:
      WATCHTOWER_MONITOR_ONLY: 'true'
      WATCHTOWER_NOTIFICATIONS: email
      WATCHTOWER_NOTIFICATION_EMAIL_FROM: from-address@your-domain.com
      WATCHTOWER_NOTIFICATION_EMAIL_TO: to-address@your-domain.com
      # you have to use a network alias here, if you use your own certificate
      WATCHTOWER_NOTIFICATION_EMAIL_SERVER: smtp.your-domain.com
      WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT: 25
      WATCHTOWER_NOTIFICATION_EMAIL_DELAY: 2
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    networks:
      - watchtower
    depends_on:
      - postfix

  # SMTP needed to send out status emails
  postfix:
    image: freinet/postfix-relay:latest
    expose:
      - 25
    environment:
      MAILNAME: somename.your-domain.com
      TLS_KEY: '/etc/ssl/domains/your-domain.com/your-domain.com.key'
      TLS_CRT: '/etc/ssl/domains/your-domain.com/your-domain.com.crt'
      TLS_CA: '/etc/ssl/domains/your-domain.com/intermediate.crt'
    volumes:
      - /etc/ssl/domains/your-domain.com/:/etc/ssl/domains/your-domain.com/:ro
    networks:
      watchtower:
        # this alias is really important to make your certificate work
        aliases:
          - smtp.your-domain.com
networks:
  watchtower:
    external: false
```

### Slack

To receive notifications in Slack, add `slack` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the Slack webhook URL using the `--notification-slack-hook-url` option or the `WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL` environment variable. This option can also reference a file, in which case the contents of the file are used.

By default, watchtower will send messages under the name `watchtower`, you can customize this string through the `--notification-slack-identifier` option or the `WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER` environment variable.

Other, optional, variables include:

-   `--notification-slack-channel` (env. `WATCHTOWER_NOTIFICATION_SLACK_CHANNEL`): A string which overrides the webhook's default channel. Example: #my-custom-channel.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=slack \
  -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
  -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
  -e WATCHTOWER_NOTIFICATION_SLACK_CHANNEL=#my-custom-channel \
  containrrr/watchtower
```

### Microsoft Teams

To receive notifications in MSTeams channel, add `msteams` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the MSTeams webhook URL using the `--notification-msteams-hook` option or the `WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL` environment variable. This option can also reference a file, in which case the contents of the file are used.

MSTeams notifier could send keys/values filled by `log.WithField` or `log.WithFields` as MSTeams message facts. To enable this feature add `--notification-msteams-data` flag or set `WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA=true` environment variable.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=msteams \
  -e WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL="https://outlook.office.com/webhook/xxxxxxxx@xxxxxxx/IncomingWebhook/yyyyyyyy/zzzzzzzzzz" \
  -e WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA=true \
  containrrr/watchtower
```

### Gotify

To push a notification to your Gotify instance, register a Gotify app and specify the Gotify URL and app token:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=gotify \
  -e WATCHTOWER_NOTIFICATION_GOTIFY_URL="https://my.gotify.tld/" \
  -e WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN="SuperSecretToken" \
  containrrr/watchtower
```

`-e WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN` or `--notification-gotify-token` can also reference a file, in which case the contents of the file are used.

If you want to disable TLS verification for the Gotify instance, you can use either `-e WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY=true` or `--notification-gotify-tls-skip-verify`.

