## Executing commands before and after updating

> **DO NOTE**: These are shell commands executed with `sh`, and therefore require the
> container to provide the `sh` executable.

It is possible to execute _pre/post\-check_ and _pre/post\-update_ commands
**inside** every container updated by watchtower.

- The _pre-check_ command is executed before checking the container for updates.
- The _pre-update_ command is executed before stopping the container when an update is about to start.
- The _post-update_ command is executed after restarting the updated container
- The _post-check_ command is executed last after a updated container is started or no update was needed.

This feature is disabled by default. To enable it, you need to set the option
`--enable-lifecycle-hooks` on the command line, or set the environment variable
`WATCHTOWER_LIFECYCLE_HOOKS` to `true`.

### Specifying update commands

The commands are specified using docker container labels, the following are currently available:

- `com.centurylinklabs.watchtower.lifecycle.pre-check` - _pre-check_
- `com.centurylinklabs.watchtower.lifecycle.pre-update-command` - _pre-update_
- `com.centurylinklabs.watchtower.lifecycle.post-update` - _post-update_
- `com.centurylinklabs.watchtower.lifecycle.post-check` - _post-check_

These labels can be declared as instructions in a Dockerfile (with some example .sh files):

```docker
LABEL com.centurylinklabs.watchtower.lifecycle.pre-check="/sync.sh"
LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh"
LABEL com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh"
LABEL com.centurylinklabs.watchtower.lifecycle.post-check="/send-heartbeat.sh"
```

Or be specified as part of the `docker run` command line:

```bash
docker run -d \
  --label=com.centurylinklabs.watchtower.lifecycle.pre-check="/sync.sh" \
  --label=com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh" \
  --label=com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh" \
  someimage
  --label=com.centurylinklabs.watchtower.lifecycle.post-check="/send-heartbeat.sh" \
```

### Execution failure

The failure of a command to execute, identified by an exit code different than
0, will not prevent watchtower from updating the container. Only an error
log statement containing the exit code will be reported.
