# Logging and Output

## Debug

Enables debug mode with verbose logging.

```text
            Argument: --debug, -d
Environment Variable: WATCHTOWER_DEBUG
                Type: Boolean
             Default: false
```

!!! Note
    Equivalent to `--log-level debug`.
    As an argument, it does not accept a value (e.g., `--debug true` is invalid).

    See [Maximum Log Level](#maximum_log_level).

## Trace

Enables trace mode with highly verbose logging, including sensitive information like credentials.

```text
            Argument: --trace
Environment Variable: WATCHTOWER_TRACE
                Type: Boolean
             Default: false
```

!!! Note
    Equivalent to `--log-level trace`.
    As an argument, does not accept a value (e.g., `--trace true` is invalid).

    See [Maximum Log Level](#maximum_log_level).

!!! Warning
    Use with caution due to credential exposure.

## Maximum Log Level

Sets the maximum log level output to STDERR, visible in `docker logs` when running Watchtower in a container.

```text
            Argument: --log-level
Environment Variable: WATCHTOWER_LOG_LEVEL
     Possible Values: panic, fatal, error, warn, info, debug, trace
             Default: info
```

## Logging Format

Specifies the format for console log output.

```text
            Argument: --log-format, -l
Environment Variable: WATCHTOWER_LOG_FORMAT
     Possible Values: Auto, LogFmt, Pretty, JSON
             Default: Auto
```

## Disable ANSI Colors

Disables ANSI color escape codes in log output for plain text logs.

```text
            Argument: --no-color
Environment Variable: NO_COLOR
                Type: Boolean
             Default: false
```

## Programmatic Output (Porcelain)

Outputs session results in a machine-readable format.

```text
            Argument: --porcelain, -P
Environment Variable: WATCHTOWER_PORCELAIN
     Possible Values: v1, json
             Default: None
```

!!! Note
    Equivalent to:
    ```text
    --notification-url logger://
    --notification-log-stdout
    --notification-report
    ```

Porcelain only fills in the notification-template, notification-report, and notification-log-stdout configuration options you have not set. A value you set yourself, with a flag or an environment variable, is kept.

For `v1`, the template is `porcelain.v1.summary-no-log`, producing one container per line.

For `json`, the template is `porcelain.json`, producing a JSON document:

```json
{
  "containers": [
    {
      "name": "nginx",
      "image": "nginx:latest",
      "image_id": "abc123",
      "latest_image_id": "def456",
      "state": "Updated",
      "update_available": true
    }
  ]
}
```

Fields:

- `name` — container name
- `image` — image name with tag
- `image_id` — current short image ID
- `latest_image_id` — latest short image ID
- `state` — container state (`Updated`, `Fresh`, `Failed`, `Skipped`, `Restarted`, `Stale`)
- `update_available` — whether an update is available
- `error` — error message if any (omitted when empty)
