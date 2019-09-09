
# Notifications

Watchtower can send notifications when containers are updated. Notifications are sent via hooks in the logging system, [logrus](http://github.com/sirupsen/logrus).
The types of notifications to send are passed via the comma-separated option `--notifications` (or corresponding environment variable `WATCHTOWER_NOTIFICATIONS`), which has the following valid values:

- `email` to send notifications via e-mail
- `slack` to send notifications through a Slack webhook
- `msteams` to send notifications via MSTeams webhook
- `gotify` to send notifications via Gotify
- `hangouts` to send notifications via Hangouts Chat webhook

## Settings

- `--notifications-level` (env. `WATCHTOWER_NOTIFICATIONS_LEVEL`): Controls the log level which is used for the notifications. If omitted, the default log level is `info`. Possible values are: `panic`, `fatal`, `error`, `warn`, `info` or `debug`.

## Available services

### Email

To receive notifications by email, the following command-line options, or their corresponding environment variables, can be set:

- `--notification-email-from` (env. `WATCHTOWER_NOTIFICATION_EMAIL_FROM`): The e-mail address from which notifications will be sent.
- `--notification-email-to` (env. `WATCHTOWER_NOTIFICATION_EMAIL_TO`): The e-mail address to which notifications will be sent.
- `--notification-email-server` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER`): The SMTP server to send e-mails through.
- `--notification-email-server-tls-skip-verify` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY`): Do not verify the TLS certificate of the mail server. This should be used only for testing.
- `--notification-email-server-port` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT`): The port used to connect to the SMTP server to send e-mails through. Defaults to `25`.
- `--notification-email-server-user` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER`): The username to authenticate with the SMTP server with.
- `--notification-email-server-password` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD`): The password to authenticate with the SMTP server with.
- `--notification-email-delay` (env. `WATCHTOWER_NOTIFICATION_EMAIL_DELAY`): Delay before sending notifications expressed in seconds.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=email \
  -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=fromaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_TO=toaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=smtp.gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER=fromaddress@gmail.com \
  -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD=app_password \
  -e WATCHTOWER_NOTIFICATION_EMAIL_DELAY=2 \
  containrrr/watchtower
```

### Slack
If watchtower is monitoring the same Docker daemon under which the watchtower container itself is running (i.e. if you volume-mounted _/var/run/docker.sock_ into the watchtower container) then it has the ability to update itself. If a new version of the _containrrr/watchtower_ image is pushed to the Docker Hub, your watchtower will pull down the new image and restart itself automatically.

To receive notifications in Slack, add `slack` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the Slack webhook URL using the `--notification-slack-hook-url` option or the `WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL` environment variable.

By default, watchtower will send messages under the name `watchtower`, you can customize this string through the `--notification-slack-identifier` option or the `WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER` environment variable.

Other, optional, variables include:

- `--notification-slack-channel` (env. `WATCHTOWER_NOTIFICATION_SLACK_CHANNEL`): A string which overrides the webhook's default channel. Example: #my-custom-channel.
- `--notification-slack-icon-emoji` (env. `WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI`): An [emoji code](https://www.webpagefx.com/tools/emoji-cheat-sheet/) string to use in place of the default icon.
- `--notification-slack-icon-url` (env. `WATCHTOWER_NOTIFICATION_SLACK_ICON_URL`): An icon image URL string to use in place of the default icon.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=slack \
  -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
  -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
  -e WATCHTOWER_NOTIFICATION_SLACK_CHANNEL=#my-custom-channel \
  -e WATCHTOWER_NOTIFICATION_SLACK_ICON_EMOJI=:whale: \
  -e WATCHTOWER_NOTIFICATION_SLACK_ICON_URL=<icon url> \
  containrrr/watchtower
```

### Microsoft Teams

To receive notifications in MSTeams channel, add `msteams` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the MSTeams webhook URL using the `--notification-msteams-hook` option or the `WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL` environment variable.

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

### Hangouts Chat

To push a notification to a Hangouts Chat channel, create a new [channel webhoook](https://developers.google.com/hangouts/chat/how-tos/webhooks) and set the webhook URL:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=hangouts \
  -e WATCHTOWER_NOTIFICATION_HANGOUTS_CHAT_WEBHOOK_URL="https://chat.googleapis.com/v1/spaces/XXXXXXX/messages?key=YYYYYYYYY&token=ZZZZZZZZZZ" \
  containrrr/watchtower
```
