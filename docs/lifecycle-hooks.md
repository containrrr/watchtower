
## Executing commands before and after updating

> **DO NOTE**: Both commands are shell commands executed with `sh`, and therefore require the 
> container to provide the `sh` executable.

It is possible to execute a *pre-update* command and a *post-update* command 
**inside** every container updated by watchtower. The *pre-update* command is 
executed before stopping the container, and the *post-update* command is 
executed after restarting the container.

This feature is disabled by default. To enable it, you need to set the option
`--enable-lifecycle-hooks` on the command line, or set the environment variable
`WATCHTOWER_LIFECYCLE_HOOKS` to true.

 

### Specifying update commands

The commands are specified using docker container labels, with 
`com.centurylinklabs.watchtower.pre-update-command` for the *pre-update* 
command and `com.centurylinklabs.watchtower.lifecycle.post-update` for the
*post-update* command.

These labels can be declared as instructions in a Dockerfile:

```docker
LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh"
LABEL com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh"
```

Or be specified as part of the `docker run` command line:

```bash
docker run -d \
  --label=com.centurylinklabs.watchtower.lifecycle.pre-update="/dump-data.sh" \
  --label=com.centurylinklabs.watchtower.lifecycle.post-update="/restore-data.sh" \
  someimage
```

### Execution failure

The failure of a command to execute, identified by an exit code different than 
0, will not prevent watchtower from updating the container. Only an error
log statement containing the exit code will be reported.