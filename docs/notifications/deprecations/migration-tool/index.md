# Migration Tool

## Overview

Watchtower includes a `notify-upgrade` command to help convert legacy notification configurations to Shoutrrr URLs for use with the [`NOTIFICATION URL`](../../../configuration/notifications/index.md#notification_url).

It parses legacy Watchtower notification configurations and outputs Shoutrrr URLs to a temporary file inside the Watchtower container.

## Usage

The `notify-upgrade` command converts legacy Watchtower notification configurations into Shoutrrr URLs and writes them to a temporary file inside the container.

### Running the Command

The `notify-upgrade` can be run either against a Watchtower container that is already running or as a single-run instance that allows you to convert a Docker Compose configuration or Docker CLI flag / environment variable configuration.

#### Existing Deployment

If Watchtower is already deployed and running with your legacy notification configuration, you can execute `notify-upgrade` directly inside the container:

```bash
docker exec watchtower /watchtower notify-upgrade
```

This works because the Watchtower binary is installed at `/watchtower` and `notify-upgrade` is a built-in subcommand.
The common mistake is running `docker exec watchtower notify-upgrade`, which fails because `notify-upgrade` is not a standalone executable in `$PATH`.

!!! Note
    Running `notify-upgrade` inside an existing container does **not** stop or restart the main Watchtower process.
    `docker exec` runs a separate, short-lived process.
    The only side effect is that the `notify-upgrade` process will stay alive for up to 5 minutes while waiting for you to retrieve the temporary file, after which it cleans up and exits.

#### Single-Use Run

If you have not yet deployed Watchtower or if you want to run `notify-upgrade` as a one-off command without starting the persistent Watchtower update loop, then run a temporary container configured with `notify-upgrade` as the command.

=== "Email (SMTP)"

    === "Docker Compose"
        ```yaml
        services:
          watchtower:
            image: nickfedor/watchtower:latest
            command: notify-upgrade
            environment:
              WATCHTOWER_NOTIFICATIONS: email
              WATCHTOWER_NOTIFICATION_EMAIL_SERVER: smtp.example.com
              WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT: 587
              WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER: user@example.com
              WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD: secret
              WATCHTOWER_NOTIFICATION_EMAIL_FROM: sender@example.com
              WATCHTOWER_NOTIFICATION_EMAIL_TO: recipient@example.com
            volumes:
              - /var/run/docker.sock:/var/run/docker.sock
        ```
    === "Docker CLI (Env Vars)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -e WATCHTOWER_NOTIFICATIONS=email \
          -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=smtp.example.com \
          -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT=587 \
          -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER=user@example.com \
          -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD=secret \
          -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=sender@example.com \
          -e WATCHTOWER_NOTIFICATION_EMAIL_TO=recipient@example.com \
          nickfedor/watchtower \
          notify-upgrade
        ```
    === "Docker CLI (Flags)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          nickfedor/watchtower \
          notify-upgrade \
          --notifications email \
          --notification-email-server smtp.example.com \
          --notification-email-server-port 587 \
          --notification-email-server-user user@example.com \
          --notification-email-server-password secret \
          --notification-email-from sender@example.com \
          --notification-email-to recipient@example.com
        ```

=== "Gotify"

    === "Docker Compose"
        ```yaml
        services:
          watchtower:
            image: nickfedor/watchtower:latest
            command: notify-upgrade
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
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -e WATCHTOWER_NOTIFICATIONS=gotify \
          -e WATCHTOWER_NOTIFICATION_GOTIFY_URL="https://my.gotify.tld/" \
          -e WATCHTOWER_NOTIFICATION_GOTIFY_TOKEN="SuperSecretToken" \
          -e WATCHTOWER_NOTIFICATION_GOTIFY_TLS_SKIP_VERIFY=true \
          nickfedor/watchtower \
          notify-upgrade
        ```
    === "Docker CLI (Flags)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          nickfedor/watchtower \
          notify-upgrade \
          --notifications gotify \
          --notification-gotify-url "https://my.gotify.tld/" \
          --notification-gotify-token "SuperSecretToken" \
          --notification-gotify-tls-skip-verify
        ```

=== "Slack"

    === "Docker Compose"
        ```yaml
        services:
          watchtower:
            image: nickfedor/watchtower:latest
            command: notify-upgrade
            environment:
              WATCHTOWER_NOTIFICATIONS: slack
              WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL: "https://hooks.slack.com/services/AAA/BBB/CCC"
              WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER: watchtower-server-1
            volumes:
              - /var/run/docker.sock:/var/run/docker.sock
        ```
    === "Docker CLI (Env Vars)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -e WATCHTOWER_NOTIFICATIONS=slack \
          -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/AAA/BBB/CCC" \
          -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
          nickfedor/watchtower \
          notify-upgrade
        ```
    === "Docker CLI (Flags)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          nickfedor/watchtower \
          notify-upgrade \
          --notifications slack \
          --notification-slack-hook-url "https://hooks.slack.com/services/AAA/BBB/CCC" \
          --notification-slack-identifier watchtower-server-1 \
        ```

=== "Microsoft Teams"

    === "Docker Compose"
        ```yaml
        services:
          watchtower:
            image: nickfedor/watchtower:latest
            command: notify-upgrade
            environment:
              WATCHTOWER_NOTIFICATIONS: msteams
              WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL: "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
            volumes:
              - /var/run/docker.sock:/var/run/docker.sock
        ```
    === "Docker CLI (Env Vars)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -e WATCHTOWER_NOTIFICATIONS=msteams \
          -e WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL="https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX" \
          nickfedor/watchtower \
          notify-upgrade
        ```
    === "Docker CLI (Flags)"
        ```bash
        docker run --rm \
          --name watchtower-notify-upgrade \
          -v /var/run/docker.sock:/var/run/docker.sock \
          nickfedor/watchtower \
          notify-upgrade \
          --notifications msteams \
          --notification-msteams-hook "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
        ```

!!! Important
    Replace the example environment variables with your own legacy notification configuration.
    The `notify-upgrade` command inspects the container's current configuration, so it must be started with your legacy notification settings.

### Retrieving the Generated File

After running `notify-upgrade`, the converted Shoutrrr URL is written to a temporary file in the container's working directory, with a filename like `watchtower-notif-urls-XXXXXX`.

The command logs the exact container path and prints a copy command, for example:

```text
INFO To get the environment file, use: docker cp abc123:watchtower-notif-urls-123456789 ./watchtower-notifications.env
```

Copy it to your local machine with:

```bash
docker cp <CONTAINER>:<FILE_PATH> ./watchtower-notifications.env
```

The temporary file is automatically removed after **5 minutes** or when the container stops, so copy it promptly after generation.
If you are running `notify-upgrade` as a one-off command (`docker run --rm ...`), the process will wait up to 5 minutes before exiting.

## Migration Walkthrough

Select the tab that matches your legacy notification service.

=== "Email (SMTP)"

    1. Run the following Docker Compose configuration / Docker CLI command:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATIONS: email
                        WATCHTOWER_NOTIFICATION_EMAIL_SERVER: smtp.example.com
                        WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT: 587
                        WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER: user@example.com
                        WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD: secret
                        WATCHTOWER_NOTIFICATION_EMAIL_FROM: sender@example.com
                        WATCHTOWER_NOTIFICATION_EMAIL_TO: recipient@example.com
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
            --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATIONS=email \
              -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER=smtp.example.com \
              -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT=587 \
              -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER=user@example.com \
              -e WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD=secret \
              -e WATCHTOWER_NOTIFICATION_EMAIL_FROM=sender@example.com \
              -e WATCHTOWER_NOTIFICATION_EMAIL_TO=recipient@example.com \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notifications email \
              --notification-email-server smtp.example.com \
              --notification-email-server-port 587 \
              --notification-email-server-user user@example.com \
              --notification-email-server-password secret \
              --notification-email-from sender@example.com \
              --notification-email-to recipient@example.com
            ```

    2. With the container running, execute the `notify-upgrade` command:

        ```bash
        docker exec watchtower /watchtower notify-upgrade
        ```

        Then copy the resulting file out of the container:

        ```bash
        docker cp <CONTAINER>:<FILE_PATH> ./watchtower-notifications.env
        ```

    3. The following converted Shoutrrr URL should be produced:

        ```text
        smtp://user@example.com:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com&encryption=ExplicitTLS&usestarttls=yes
        ```

    4. Replace the deprecated configuration with the coverted Shoutrrr URL:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATION_URL: smtp://user@example.com:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com&encryption=ExplicitTLS&usestarttls=yes
                        WATCHTOWER_NOTIFICATIONS_DELAY: "10"
                        WATCHTOWER_NOTIFICATION_TITLE_TAG: Watchtower
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e "WATCHTOWER_NOTIFICATION_URL=smtp://user@example.com:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com&encryption=ExplicitTLS&usestarttls=yes" \
              -e WATCHTOWER_NOTIFICATIONS_DELAY=10 \
              -e WATCHTOWER_NOTIFICATION_TITLE_TAG=Watchtower \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notification-url "smtp://user@example.com:secret@smtp.example.com:587/?fromaddress=sender@example.com&toaddresses=recipient@example.com&encryption=ExplicitTLS&usestarttls=yes" \
              --notifications-delay 10 \
              --notification-title-tag Watchtower
            ```

    !!! Note
        - Avoid using unrecognized flags like `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_SSL`, as they are ignored and may cause confusion.
        - Use the `encryption` and `usestarttls` URL parameters in the `smtp://` URL to control TLS behavior rather than deprecated flags.

=== "Gotify"

    1. Run the following Docker Compose configuration / Docker CLI command:

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

    2. With the container running, execute the `notify-upgrade` command:

        ```bash
        docker exec watchtower /watchtower notify-upgrade
        ```

        Then copy the resulting file out of the container:

        ```bash
        docker cp <CONTAINER>:<FILE_PATH> ./watchtower-notifications.env
        ```

    3. The following converted Shoutrrr URL should be produced:

        ```text
        gotify://my.gotify.tld/SuperSecretToken?title=
        ```

    4. Replace the deprecated configuration with the coverted Shoutrrr URL:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATION_URL: "gotify://my.gotify.tld/SuperSecretToken?title="
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATION_URL="gotify://my.gotify.tld/SuperSecretToken?title=" \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notification-url "gotify://my.gotify.tld/SuperSecretToken?title="
            ```

=== "Slack"

    1. Run the following Docker Compose configuration / Docker CLI command:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATIONS: slack
                        WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL: "https://hooks.slack.com/services/AAA/BBB/CCC"
                        WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER: watchtower-server-1
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATIONS=slack \
          -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/AAA/BBB/CCC" \
          -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notifications slack \
              --notification-slack-hook-url "https://hooks.slack.com/services/AAA/BBB/CCC" \
          --notification-slack-identifier watchtower-server-1 \
            ```

    2. With the container running, execute the `notify-upgrade` command:

        ```bash
        docker exec watchtower /watchtower notify-upgrade
        ```

        Then copy the resulting file out of the container:

        ```bash
        docker cp <CONTAINER>:<FILE_PATH> ./watchtower-notifications.env
        ```

    3. The following converted Shoutrrr URL should be produced:

        ```text
        slack://hook:AAA-BBB-CCC@webhook?botname=watchtower&color=%23406170
        ```

    4. Replace the deprecated configuration with the coverted Shoutrrr URL:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATION_URL: "slack://hook:AAA-BBB-CCC@webhook?botname=watchtower&color=%23406170"
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATION_URL="slack://hook:AAA-BBB-CCC@webhook?botname=watchtower&color=%23406170" \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notification-url "slack://hook:AAA-BBB-CCC@webhook?botname=watchtower&color=%23406170"
            ```

=== "Microsoft Teams"

    1. Run the following Docker Compose configuration / Docker CLI command:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATIONS: msteams
                        WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL: "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATIONS=msteams \
              -e WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL="https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX" \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notifications msteams \
              --notification-msteams-hook "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
            ```

    2. With the container running, execute the `notify-upgrade` command:

        ```bash
        docker exec watchtower /watchtower notify-upgrade
        ```

        Then copy the resulting file out of the container:

        ```bash
        docker cp <CONTAINER>:<FILE_PATH> ./watchtower-notifications.env
        ```

    3. The following converted Shoutrrr URL should be produced:

        ```text
        teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX&color=%23406170
        ```

    4. Replace the deprecated configuration with the coverted Shoutrrr URL:

        === "Docker Compose"
            ```yaml
            services:
                watchtower:
                    image: nickfedor/watchtower:latest
                    environment:
                        WATCHTOWER_NOTIFICATION_URL: "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX&color=%23406170"
                    volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
            ```
        === "Docker CLI (Env Vars)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              -e WATCHTOWER_NOTIFICATION_URL="teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX&color=%23406170" \
              nickfedor/watchtower
            ```
        === "Docker CLI (Flags)"
            ```bash
            docker run -d \
              --name watchtower \
              -v /var/run/docker.sock:/var/run/docker.sock \
              nickfedor/watchtower \
              --notification-url "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX&color=%23406170"
            ```

## Automated Migration Script

If you have a `docker-compose.yaml` with a `watchtower` service configured for legacy notifications, you can use the script below to run the entire migration process end-to-end. It starts the container, executes `notify-upgrade`, extracts the generated file, prints its contents, and cleans up.

```bash
#!/usr/bin/env bash
set -euo pipefail

SERVICE="${1:-watchtower}"
OUTPUT="${2:-./watchtower-notifications.env}"

CONTAINER=$(docker compose run -d "${SERVICE}" notify-upgrade)

trap 'docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true' EXIT

if ! docker ps -q --filter "id=${CONTAINER}" | grep -q .; then
    echo "notify-upgrade container exited before producing output. Logs:" >&2
    docker logs "${CONTAINER}" >&2 || true

    exit 1
fi

FILE=""
TIMEOUT=300
INTERVAL=2
ELAPSED=0

while [[ ${ELAPSED} -lt ${TIMEOUT} ]]; do
    FILE=$(docker logs "${CONTAINER}" 2>&1 | grep -oE 'watchtower-notif-urls-[^ ]+' | tail -1 || true)

    if [[ -n "${FILE}" ]]; then
        break
    fi

    sleep "${INTERVAL}"

    ELAPSED=$((ELAPSED + INTERVAL))
done

if [[ -z "${FILE}" ]]; then
    echo "Timed out waiting for notify-upgrade output. Logs:" >&2
    docker logs "${CONTAINER}" >/dev/null 2>&1 || true

    exit 1
fi

docker cp "${CONTAINER}:${FILE}" "${OUTPUT}"

echo "=== Generated Shoutrrr URL ==="
cat "${OUTPUT}"
```

!!! Important
    - The service must already be defined in `docker-compose.yaml` with your legacy notification environment variables. The script assumes the service name defaults to `watchtower`.
    - The command will self-cleanup after 5 minutes or when the script exits.
    - `notify-upgrade` keeps a Slack channel only when it is a Slack channel ID, such as `C0123456789`. A channel name such as `#ops` or a user such as `@alice` cannot be carried into the Shoutrrr URL, so the generated URL uses the webhook's own channel and `notify-upgrade` logs a warning. Webhooks created by a Slack app always post to their own channel.
