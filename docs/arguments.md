By default, watchtower will monitor all containers running within the Docker daemon to which it is pointed (in most cases this
will be the local Docker daemon, but you can override it with the `--host` option described in the next section). However, you
can restrict watchtower to monitoring a subset of the running containers by specifying the container names as arguments when
launching watchtower.

```bash
$ docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    containrrr/watchtower \
    nginx redis
```

In the example above, watchtower will only monitor the containers named "nginx" and "redis" for updates -- all of the other
running containers will be ignored. If you do not want watchtower to run as a daemon you can pass the `--run-once` flag and remove
the watchtower container after its execution.

```bash
$ docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    containrrr/watchtower \
    --run-once \
    nginx redis
```

In the example above, watchtower will execute an upgrade attempt on the containers named "nginx" and "redis". Using this mode will enable debugging output showing all actions performed, as usage is intended for interactive users. Once the attempt is completed, the container will exit and remove itself due to the `--rm` flag.

When no arguments are specified, watchtower will monitor all running containers.

## Secrets/Files

Some arguments can also reference a file, in which case the contents of the file are used as the value.
This can be used to avoid putting secrets in the configuration file or command line.

The following arguments are currently supported (including their corresponding `WATCHTOWER_` environment variables):
 - `notification-url`
 - `notification-email-server-password`
 - `notification-slack-hook-url`
 - `notification-msteams-hook`
 - `notification-gotify-token`
 - `http-api-token`

### Example docker-compose usage
```yaml
secrets:
  access_token:
    file: access_token

services:
  watchtower:
    secrets:
      - access_token
    environment:
      - WATCHTOWER_HTTP_API_TOKEN=/run/secrets/access_token
```

## Help
Shows documentation about the supported flags.

```text
            Argument: --help
Environment Variable: N/A
                Type: N/A
             Default: N/A
```

## Time Zone
Sets the time zone to be used by WatchTower's logs and the optional Cron scheduling argument (--schedule). If this environment variable is not set, Watchtower will use the default time zone: UTC.
To find out the right value, see [this list](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones), find your location and use the value in _TZ Database Name_, e.g _Europe/Rome_. The timezone can alternatively be set by volume mounting your hosts /etc/localtime file. `-v /etc/localtime:/etc/localtime:ro`

```text
            Argument: N/A
Environment Variable: TZ
                Type: String
             Default: "UTC"
```

## Cleanup
Removes old images after updating. When this flag is specified, watchtower will remove the old image after restarting a container with a new image. Use this option to prevent the accumulation of orphaned images on your system as containers are updated.

```text
            Argument: --cleanup
Environment Variable: WATCHTOWER_CLEANUP
                Type: Boolean
             Default: false
```

## Remove anonymous volumes
Removes anonymous volumes after updating. When this flag is specified, watchtower will remove all anonymous volumes from the container before restarting with a new image. Named volumes will not be removed!

```text
            Argument: --remove-volumes
Environment Variable: WATCHTOWER_REMOVE_VOLUMES
                Type: Boolean
             Default: false
```

## Debug
Enable debug mode with verbose logging.

!!! note "Notes"  
    Alias for `--log-level debug`. See [Maximum log level](#maximum-log-level).  
    Does _not_ take an argument when used as an argument. Using `--debug true` will **not** work.

```text
            Argument: --debug, -d
Environment Variable: WATCHTOWER_DEBUG
                Type: Boolean
             Default: false
```

## Trace
Enable trace mode with very verbose logging. Caution: exposes credentials!

!!! note "Notes"  
    Alias for `--log-level trace`. See [Maximum log level](#maximum-log-level).  
    Does _not_ take an argument when used as an argument. Using `--trace true` will **not** work.

```text
            Argument: --trace
Environment Variable: WATCHTOWER_TRACE
                Type: Boolean
             Default: false
```

## Maximum log level

The maximum log level that will be written to STDERR (shown in `docker log` when used in a container).

```text
            Argument: --log-level
Environment Variable: WATCHTOWER_LOG_LEVEL
     Possible values: panic, fatal, error, warn, info, debug or trace
             Default: info
```

## Logging format

Sets what logging format to use for console output.

```text
            Argument: --log-format, -l
Environment Variable: WATCHTOWER_LOG_FORMAT
     Possible values: Auto, LogFmt, Pretty or JSON
             Default: Auto
```

## ANSI colors
Disable ANSI color escape codes in log output.

```text
            Argument: --no-color
Environment Variable: NO_COLOR
                Type: Boolean
             Default: false
```

## Docker host
Docker daemon socket to connect to. Can be pointed at a remote Docker host by specifying a TCP endpoint as "tcp://hostname:port".

```text
            Argument: --host, -H
Environment Variable: DOCKER_HOST
                Type: String
             Default: "unix:///var/run/docker.sock"
```

## Docker API version
The API version to use by the Docker client for connecting to the Docker daemon. The minimum supported version is 1.24.

```text
            Argument: --api-version, -a
Environment Variable: DOCKER_API_VERSION
                Type: String
             Default: "1.24"
```

## Include restarting
Will also include restarting containers.

```text
            Argument: --include-restarting
Environment Variable: WATCHTOWER_INCLUDE_RESTARTING
                Type: Boolean
             Default: false
```

## Include stopped
Will also include created and exited containers.

```text
            Argument: --include-stopped, -S
Environment Variable: WATCHTOWER_INCLUDE_STOPPED
                Type: Boolean
             Default: false
```

## Revive stopped
Start any stopped containers that have had their image updated. This argument is only usable with the `--include-stopped` argument.

```text
            Argument: --revive-stopped
Environment Variable: WATCHTOWER_REVIVE_STOPPED
                Type: Boolean
             Default: false
```

## Poll interval
Poll interval (in seconds). This value controls how frequently watchtower will poll for new images. Either `--schedule` or a poll interval can be defined, but not both.

```text
            Argument: --interval, -i
Environment Variable: WATCHTOWER_POLL_INTERVAL
                Type: Integer
             Default: 86400 (24 hours)
```

## Filter by enable label
Monitor and update containers that have a `com.centurylinklabs.watchtower.enable` label set to true.

```text
            Argument: --label-enable
Environment Variable: WATCHTOWER_LABEL_ENABLE
                Type: Boolean
             Default: false
```

## Filter by disable label
__Do not__ Monitor and update containers that have `com.centurylinklabs.watchtower.enable` label set to false and 
no `--label-enable` argument is passed. Note that only one or the other (targeting by enable label) can be 
used at the same time to target containers.

## Filter by disabling specific container names
Monitor and update containers whose names are not in a given set of names.

This can be used to exclude specific containers, when setting labels is not an option.
The listed containers will be excluded even if they have the enable filter set to true.

```text
            Argument: --disable-containers, -x
Environment Variable: WATCHTOWER_DISABLE_CONTAINERS
                Type: Comma- or space-separated string list
             Default: ""
```

## Without updating containers
Will only monitor for new images, send notifications and invoke
the [pre-check/post-check hooks](https://containrrr.dev/watchtower/lifecycle-hooks/), but will __not__ update the
containers.

!!! note
    Due to Docker API limitations the latest image will still be pulled from the registry.
    The HEAD digest checks allows watchtower to skip pulling when there are no changes, but to know _what_ has changed it
    will still do a pull whenever the repository digest doesn't match the local image digest.

```text
            Argument: --monitor-only
Environment Variable: WATCHTOWER_MONITOR_ONLY
                Type: Boolean
             Default: false
```

Note that monitor-only can also be specified on a per-container basis with the `com.centurylinklabs.watchtower.monitor-only` label set on those containers.

See [With label taking precedence over arguments](#With-label-taking-precedence-over-arguments) for behavior when both argument and label are set

## With label taking precedence over arguments

By default, arguments will take precedence over labels. This means that if you set `WATCHTOWER_MONITOR_ONLY` to true or use `--monitor-only`, a container with `com.centurylinklabs.watchtower.monitor-only` set to false will not be updated. If you set `WATCHTOWER_LABEL_TAKE_PRECEDENCE` to true or use `--label-take-precedence`, then the container will also be updated. This also apply to the no pull option. if you set `WATCHTOWER_NO_PULL` to true or use `--no-pull`, a container with `com.centurylinklabs.watchtower.no-pull` set to false will not pull the new image. If you set `WATCHTOWER_LABEL_TAKE_PRECEDENCE` to true or use `--label-take-precedence`, then the container will pull image

```text
            Argument: --label-take-precedence
Environment Variable: WATCHTOWER_LABEL_TAKE_PRECEDENCE
                Type: Boolean
             Default: false
```

## Without restarting containers
Do not restart containers after updating. This option can be useful when the start of the containers
is managed by an external system such as systemd.
```text
            Argument: --no-restart
Environment Variable: WATCHTOWER_NO_RESTART
                Type: Boolean
             Default: false
```

## Without pulling new images
Do not pull new images. When this flag is specified, watchtower will not attempt to pull
new images from the registry. Instead it will only monitor the local image cache for changes.
Use this option if you are building new images directly on the Docker host without pushing
them to a registry.

```text
            Argument: --no-pull
Environment Variable: WATCHTOWER_NO_PULL
                Type: Boolean
             Default: false
```

Note that no-pull can also be specified on a per-container basis with the
`com.centurylinklabs.watchtower.no-pull` label set on those containers.

See [With label taking precedence over arguments](#With-label-taking-precedence-over-arguments) for behavior when both argument and label are set

## Without sending a startup message
Do not send a message after watchtower started. Otherwise there will be an info-level notification.

```text
            Argument: --no-startup-message
Environment Variable: WATCHTOWER_NO_STARTUP_MESSAGE
                Type: Boolean
             Default: false
```

## Run once
Run an update attempt against a container name list one time immediately and exit.

```text
            Argument: --run-once, -R
Environment Variable: WATCHTOWER_RUN_ONCE
                Type: Boolean
             Default: false
```

## HTTP API Mode
Runs Watchtower in HTTP API mode, only allowing image updates to be triggered by an HTTP request. 
For details see [HTTP API](https://containrrr.dev/watchtower/http-api-mode).

```text
            Argument: --http-api-update
Environment Variable: WATCHTOWER_HTTP_API_UPDATE
                Type: Boolean
             Default: false
```

## HTTP API Token
Sets an authentication token to HTTP API requests.
Can also reference a file, in which case the contents of the file are used.

```text
            Argument: --http-api-token
Environment Variable: WATCHTOWER_HTTP_API_TOKEN
                Type: String
             Default: -
```

## HTTP API periodic polls
Keep running periodic updates if the HTTP API mode is enabled, otherwise the HTTP API would prevent periodic polls.  

```text
            Argument: --http-api-periodic-polls
Environment Variable: WATCHTOWER_HTTP_API_PERIODIC_POLLS
                Type: Boolean
             Default: false
```

## Filter by scope
Update containers that have a `com.centurylinklabs.watchtower.scope` label set with the same value as the given argument. 
This enables [running multiple instances](https://containrrr.dev/watchtower/running-multiple-instances).

!!! note "Filter by lack of scope"
    If you want other instances of watchtower to ignore the scoped containers, set this argument to `none`.
    When omitted, watchtower will update all containers regardless of scope.


```text
            Argument: --scope
Environment Variable: WATCHTOWER_SCOPE
                Type: String
             Default: -
``` 

## HTTP API Metrics
Enables a metrics endpoint, exposing prometheus metrics via HTTP. See [Metrics](metrics.md) for details.  

```text
            Argument: --http-api-metrics
Environment Variable: WATCHTOWER_HTTP_API_METRICS
                Type: Boolean
             Default: false
```

## Scheduling
[Cron expression](https://pkg.go.dev/github.com/robfig/cron@v1.2.0?tab=doc#hdr-CRON_Expression_Format) in 6 fields (rather than the traditional 5) which defines when and how often to check for new images. Either `--interval` or the schedule expression
can be defined, but not both. An example: `--schedule "0 0 4 * * *"`

```text
            Argument: --schedule, -s
Environment Variable: WATCHTOWER_SCHEDULE
                Type: String
             Default: -
```

## Rolling restart
Restart one image at time instead of stopping and starting all at once.  Useful in conjunction with lifecycle hooks
to implement zero-downtime deploy.

```text
            Argument: --rolling-restart
Environment Variable: WATCHTOWER_ROLLING_RESTART
                Type: Boolean
             Default: false
```

## Wait until timeout
Timeout before the container is forcefully stopped. When set, this option will change the default (`10s`) wait time to the given value. An example: `--stop-timeout 30s` will set the timeout to 30 seconds.

```text
            Argument: --stop-timeout
Environment Variable: WATCHTOWER_TIMEOUT
                Type: Duration
             Default: 10s
```

## TLS Verification

Use TLS when connecting to the Docker socket and verify the server's certificate. See below for options used to
configure notifications.

```text
            Argument: --tlsverify
Environment Variable: DOCKER_TLS_VERIFY
                Type: Boolean
             Default: false
```

## HEAD failure warnings

When to warn about HEAD pull requests failing. Auto means that it will warn when the registry is known to handle the
requests and may rate limit pull requests (mainly docker.io).

```text
            Argument: --warn-on-head-failure
Environment Variable: WATCHTOWER_WARN_ON_HEAD_FAILURE
     Possible values: always, auto, never
             Default: auto
```

## Health check

Returns a success exit code to enable usage with docker `HEALTHCHECK`. This check is naive and only returns checks whether there is another process running inside the container, as it is the only known form of failure state for watchtowers container.

!!! note "Only for HEALTHCHECK use"
    Never put this on the main container executable command line as it is only meant to be run from docker HEALTHCHECK.

```text
            Argument: --health-check
```

## Programatic Output (porcelain)

Writes the session results to STDOUT using a stable, machine-readable format (indicated by the argument VERSION).  
  
Alias for:

```text
		--notification-url logger://
		--notification-log-stdout
		--notification-report
		--notification-template porcelain.VERSION.summary-no-log

            Argument: --porcelain, -P
Environment Variable: WATCHTOWER_PORCELAIN
     Possible values: v1
             Default: -
```
