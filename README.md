# Watchtower

[![Circle CI](https://circleci.com/gh/v2tec/watchtower.svg?style=shield)](https://circleci.com/gh/v2tec/watchtower)
[![GoDoc](https://godoc.org/github.com/v2tec/watchtower?status.svg)](https://godoc.org/github.com/v2tec/watchtower)
[![](https://images.microbadger.com/badges/image/v2tec/watchtower.svg)](https://microbadger.com/images/v2tec/watchtower "Get your own image badge on microbadger.com")
[![Go Report Card](https://goreportcard.com/badge/github.com/v2tec/watchtower)](https://goreportcard.com/report/github.com/v2tec/watchtower)

A process for watching your Docker containers and automatically restarting them whenever their base image is refreshed.

## Overview

Watchtower is an application that will monitor your running Docker containers and watch for changes to the images that those containers were originally started from. If watchtower detects that an image has changed, it will automatically restart the container using the new image.

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially.

For example, let's say you were running watchtower along with an instance of *centurylink/wetty-cli* image:

```bash
$ docker ps
CONTAINER ID   IMAGE                   STATUS          PORTS                    NAMES
967848166a45   centurylink/wetty-cli   Up 10 minutes   0.0.0.0:8080->3000/tcp   wetty
6cc4d2a9d1a5   v2tec/watchtower        Up 15 minutes                            watchtower
```

Every few minutes watchtower will pull the latest *centurylink/wetty-cli* image and compare it to the one that was used to run the "wetty" container. If it sees that the image has changed it will stop/remove the "wetty" container and then restart it using the new image and the same `docker run` options that were used to start the container initially (in this case, that would include the `-p 8080:3000` port mapping).

## Usage

Watchtower is itself packaged as a Docker container so installation is as simple as pulling the `v2tec/watchtower` image.

Since the watchtower code needs to interact with the Docker API in order to monitor the running containers, you need to mount */var/run/docker.sock* into the container with the -v flag when you run it.

Run the `watchtower` container with the following command:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  v2tec/watchtower
```

If pulling images from private Docker registries, supply registry authentication credentials with the environment variables `REPO_USER` and `REPO_PASS`
or by mounting the host's docker config file into the container (at the root of the container filesystem `/`).

```bash
docker run -d \
  --name watchtower \
  -v /home/<user>/.docker/config.json:/config.json \
  -v /var/run/docker.sock:/var/run/docker.sock \
  v2tec/watchtower container_to_watch --debug
```

If you mount the config file as described below, be sure to also prepend the url for the registry when starting up your watched image (you can omit the https://). Here is a complete docker-compose.yml file that starts up a docker container from a private repo at dockerhub and monitors it with watchtower. Note the command argument changing the interval to 30s rather than the default 5 minutes.

```json
version: "3"
services:
  cavo:
    image: index.docker.io/<org>/<image>:<tag>
    ports:
      - "443:3443"
      - "80:3080"
  watchtower:
    image: v2tec/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /root/.docker/config.json:/config.json
    command: --interval 30
```

### Arguments

By default, watchtower will monitor all containers running within the Docker daemon to which it is pointed (in most cases this will be the local Docker daemon, but you can override it with the `--host` option described in the next section). However, you can restrict watchtower to monitoring a subset of the running containers by specifying the container names as arguments when launching watchtower.

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  v2tec/watchtower nginx redis
```

In the example above, watchtower will only monitor the containers named "nginx" and "redis" for updates -- all of the other running containers will be ignored.

When no arguments are specified, watchtower will monitor all running containers.

### Options

Any of the options described below can be passed to the watchtower process by setting them after the image name in the `docker run` string:

```bash
docker run --rm v2tec/watchtower --help
```

* `--host, -h` Docker daemon socket to connect to. Defaults to "unix:///var/run/docker.sock" but can be pointed at a remote Docker host by specifying a TCP endpoint as "tcp://hostname:port". The host value can also be provided by setting the `DOCKER_HOST` environment variable.
* `--interval, -i` Poll interval (in seconds). This value controls how frequently watchtower will poll for new images. Defaults to 300 seconds (5 minutes).
* `--schedule, -s` [Cron expression](https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format) in 6 fields (rather than the traditional 5) which defines when and how often to check for new images. Either `--interval` or the schedule expression could be defined, but not both. An example: `--schedule "0 0 4 * * *" ` 
* `--no-pull` Do not pull new images. When this flag is specified, watchtower will not attempt to pull new images from the registry. Instead it will only monitor the local image cache for changes. Use this option if you are building new images directly on the Docker host without pushing them to a registry.
* `--stop-timeout` Timeout before the container is forcefully stopped. When set, this option will change the default (`10s`) wait time to the given value. An example: `--stop-timeout 30s` will set the timeout to 30 seconds.
* `--label-enable` Watch containers where the `com.centurylinklabs.watchtower.enable` label is set to true.
* `--cleanup` Remove old images after updating. When this flag is specified, watchtower will remove the old image after restarting a container with a new image. Use this option to prevent the accumulation of orphaned images on your system as containers are updated.
* `--tlsverify` Use TLS when connecting to the Docker socket and verify the server's certificate.
* `--debug` Enable debug mode. When this option is specified you'll see more verbose logging in the watchtower log file.
* `--help` Show documentation about the supported flags.

See below for options used to configure notifications.

## Linked Containers

Watchtower will detect if there are links between any of the running containers and ensure that things are stopped/started in a way that won't break any of the links. If an update is detected for one of the dependencies in a group of linked containers, watchtower will stop and start all of the containers in the correct order so that the application comes back up correctly.

For example, imagine you were running a *mysql* container and a *wordpress* container which had been linked to the *mysql* container. If watchtower were to detect that the *mysql* container required an update, it would first shut down the linked *wordpress* container followed by the *mysql* container. When restarting the containers it would handle *mysql* first and then *wordpress* to ensure that the link continued to work.

## Stopping Containers

When watchtower detects that a running container needs to be updated it will stop the container by sending it a SIGTERM signal.
If your container should be shutdown with a different signal you can communicate this to watchtower by setting a label named *com.centurylinklabs.watchtower.stop-signal* with the value of the desired signal.

This label can be coded directly into your image by using the `LABEL` instruction in your Dockerfile:

```docker
LABEL com.centurylinklabs.watchtower.stop-signal="SIGHUP"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.stop-signal=SIGHUP someimage
```

## Selectively Watching Containers

By default, watchtower will watch all containers. However, sometimes only some containers should be updated.

If you need to exclude some containers, set the *com.centurylinklabs.watchtower.enable* label to `false`.

```docker
LABEL com.centurylinklabs.watchtower.enable="false"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=false someimage
```

If you need to only include only some containers, pass the --label-enable flag on startup and set the *com.centurylinklabs.watchtower.enable* label with a value of true for the containers you want to watch.

```docker
LABEL com.centurylinklabs.watchtower.enable="true"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=true someimage
```

## Remote Hosts

By default, watchtower is set-up to monitor the local Docker daemon (the same daemon running the watchtower container itself). However, it is possible to configure watchtower to monitor a remote Docker endpoint. When starting the watchtower container you can specify a remote Docker endpoint with either the `--host` flag or the `DOCKER_HOST` environment variable:

```bash
docker run -d \
  --name watchtower \
  v2tec/watchtower --host "tcp://10.0.1.2:2375"
```

or

```bash
docker run -d \
  --name watchtower \
  -e DOCKER_HOST="tcp://10.0.1.2:2375" \
  v2tec/watchtower
```

Note in both of the examples above that it is unnecessary to mount the */var/run/docker.sock* into the watchtower container.

### Secure Connections

Watchtower is also capable of connecting to Docker endpoints which are protected by SSL/TLS. If you've used *docker-machine* to provision your remote Docker host, you simply need to volume mount the certificates generated by *docker-machine* into the watchtower container and optionally specify `--tlsverify` flag.

The *docker-machine* certificates for a particular host can be located by executing the `docker-machine env` command for the desired host (note the values for the `DOCKER_HOST` and `DOCKER_CERT_PATH` environment variables that are returned from this command). The directory containing the certificates for the remote host needs to be mounted into the watchtower container at */etc/ssl/docker*.

With the certificates mounted into the watchtower container you need to specify the `--tlsverify` flag to enable verification of the certificate:

```bash
docker run -d \
  --name watchtower \
  -e DOCKER_HOST=$DOCKER_HOST \
  -v $DOCKER_CERT_PATH:/etc/ssl/docker \
  v2tec/watchtower --tlsverify
```

## Updating Watchtower

If watchtower is monitoring the same Docker daemon under which the watchtower container itself is running (i.e. if you volume-mounted */var/run/docker.sock* into the watchtower container) then it has the ability to update itself. If a new version of the *v2tec/watchtower* image is pushed to the Docker Hub, your watchtower will pull down the new image and restart itself automatically.

## Notifications

Watchtower can send notifications when containers are updated. Notifications are sent via hooks in the logging system, [logrus](http://github.com/sirupsen/logrus).
The types of notifications to send are passed via the comma-separated option `--notifications` (or corresponding environment variable `WATCHTOWER_NOTIFICATIONS`), which has the following valid values:

* `email` to send notifications via e-mail
* `slack` to send notifications through a Slack webhook
* `msteams` to send notifications via MSTeams webhook

### Settings

* `--notifications-level` (env. `WATCHTOWER_NOTIFICATIONS_LEVEL`): Controls the log level which is used for the notifications. If omitted, the default log level is `info`. Possible values are: `panic`, `fatal`, `error`, `warn`, `info` or `debug`.

### Notifications via E-Mail

To receive notifications by email, the following command-line options, or their corresponding environment variables, can be set:

* `--notification-email-from` (env. `WATCHTOWER_NOTIFICATION_EMAIL_FROM`): The e-mail address from which notifications will be sent.
* `--notification-email-to` (env. `WATCHTOWER_NOTIFICATION_EMAIL_TO`): The e-mail address to which notifications will be sent.
* `--notification-email-server` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER`): The SMTP server to send e-mails through.
* `--notification-email-server-tls-skip-verify` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_TLS_SKIP_VERIFY`): Do not verify the TLS certificate of the mail server. This should be used only for testing.
* `--notification-email-server-port` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PORT`): The port used to connect to the SMTP server to send e-mails through. Defaults to `25`.
* `--notification-email-server-user` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_USER`): The username to authenticate with the SMTP server with.
* `--notification-email-server-password` (env. `WATCHTOWER_NOTIFICATION_EMAIL_SERVER_PASSWORD`): The password to authenticate with the SMTP server with.

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
  v2tec/watchtower
```

### Notifications through Slack webhook

To receive notifications in Slack, add `slack` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the Slack webhook url using the `--notification-slack-hook-url` option or the `WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL` environment variable.

By default, watchtower will send messages under the name `watchtower`, you can customize this string through the `--notification-slack-identifier` option or the `WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER` environment variable.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=slack \
  -e WATCHTOWER_NOTIFICATION_SLACK_HOOK_URL="https://hooks.slack.com/services/xxx/yyyyyyyyyyyyyyy" \
  -e WATCHTOWER_NOTIFICATION_SLACK_IDENTIFIER=watchtower-server-1 \
  v2tec/watchtower
```

### Notifications via MSTeams incoming webhook

To receive notifications in MSTeams channel, add `msteams` to the `--notifications` option or the `WATCHTOWER_NOTIFICATIONS` environment variable.

Additionally, you should set the MSTeams webhook url using the `--notification-msteams-hook` option or the `WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL` environment variable.

MSTeams notifier could send keys/values filled by ```log.WithField``` or ```log.WithFields``` as MSTeams message facts. To enable this feature add `--notification-msteams-data` flag or set `WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA=true` environment variable.

Example:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e WATCHTOWER_NOTIFICATIONS=msteams \
  -e WATCHTOWER_NOTIFICATION_MSTEAMS_HOOK_URL="https://outlook.office.com/webhook/xxxxxxxx@xxxxxxx/IncomingWebhook/yyyyyyyy/zzzzzzzzzz" \
  -e WATCHTOWER_NOTIFICATION_MSTEAMS_USE_LOG_DATA=true \
  v2tec/watchtower
```
