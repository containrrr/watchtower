## Executing commands before and after updating

!!! note 
    These are shell commands executed with `sh`, and therefore require the container to provide the `sh`
    executable.

> **DO NOTE**: If the container is not running then lifecycle hooks can not run and therefore 
> the update is executed without running any lifecycle hooks.

It is possible to execute _pre/post\-check_ and _pre/post\-update_ commands
**inside** every container updated by watchtower.

-   The _pre-check_ command is executed for each container prior to every update cycle.
-   The _pre-update_ command is executed before stopping the container when an update is about to start.
-   The _post-update_ command is executed after restarting the updated container
-   The _post-check_ command is executed for each container post every update cycle.

This feature is disabled by default. To enable it, you need to set the option
`--enable-lifecycle-hooks` on the command line, or set the environment variable
`WATCHTOWER_LIFECYCLE_HOOKS` to `true`.

### Specifying update commands

The commands are specified using docker container labels, the following are currently available:

| Type        | Docker Container Label                                 |
| ----------- | ------------------------------------------------------ | 
| Pre Check   | `com.centurylinklabs.watchtower.lifecycle.pre-check`   |
| Pre Update  | `com.centurylinklabs.watchtower.lifecycle.pre-update`  | 
| Post Update | `com.centurylinklabs.watchtower.lifecycle.post-update` |
| Post Check  | `com.centurylinklabs.watchtower.lifecycle.post-check`  |

These labels can be declared as instructions in a Dockerfile (with some example .sh files) or be specified as part of
the `docker run` command line:

=== "Dockerfile"
    ```docker 
    LABEL com.centurylinklabs.watchtower.lifecycle.pre-check="/sync.sh"
    LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh"
    LABEL com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh"
    LABEL com.centurylinklabs.watchtower.lifecycle.post-check="/send-heartbeat.sh"
    ```

=== "docker run"
    ```bash 
    docker run -d \
    --label=com.centurylinklabs.watchtower.lifecycle.pre-check="/sync.sh" \
    --label=com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh" \
    --label=com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh" \
    someimage --label=com.centurylinklabs.watchtower.lifecycle.post-check="/send-heartbeat.sh" \
    ```

### Timeouts
The timeout for all lifecycle commands is 60 seconds. After that, a timeout will
occur, forcing Watchtower to continue the update loop.

#### Pre- or Post-update timeouts

For the `pre-update` or `post-update` lifecycle command, it is possible to override this timeout to
allow the script to finish before forcefully killing it. This is done by adding the
label `com.centurylinklabs.watchtower.lifecycle.pre-update-timeout` or post-update-timeout respectively followed by
the timeout expressed in minutes.

If the label value is explicitly set to `0`, the timeout will be disabled.  

### Execution failure

The failure of a command to execute, identified by an exit code different than
0 or 75 (EX_TEMPFAIL), will not prevent watchtower from updating the container. Only an error
log statement containing the exit code will be reported.
