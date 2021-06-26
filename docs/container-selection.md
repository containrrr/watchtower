By default, watchtower will watch all containers. However, sometimes only some containers should be updated.

There are two options:

-   **Fully exclude**: You can choose to exclude containers entirely from being watched by watchtower.
-   **Monitor only**: In this mode, watchtower checks for container updates, sends notifications and invokes the [pre-check/post-check hooks](https://containrrr.dev/watchtower/lifecycle-hooks/) on the containers but does **not** perform the update.

## Full Exclude 

If you need to exclude some containers, set the _com.centurylinklabs.watchtower.enable_ label to `false`.

```docker
LABEL com.centurylinklabs.watchtower.enable="false"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=false someimage
```

If you need to [include only containers with the enable label](https://containrrr.github.io/watchtower/arguments/#filter_by_enable_label), pass the `--label-enable` flag or the `WATCHTOWER_LABEL_ENABLE` environment variable on startup and set the _com.centurylinklabs.watchtower.enable_ label with a value of `true` for the containers you want to watch.

```docker
LABEL com.centurylinklabs.watchtower.enable="true"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=true someimage
```

If you wish to create a monitoring scope, you will need to [run multiple instances and set a scope for each of them](https://containrrr.github.io/watchtower/running-multiple-instances).

Watchtower filters running containers by testing them against each configured criteria. A container is monitored if all criteria are met. For example:
-   If a container's name is on the monitoring name list (not empty `--name` argument) but it is not enabled (_centurylinklabs.watchtower.enable=false_), it won't be monitored;
-   If a container's name is not on the monitoring name list (not empty `--name` argument), even if it is enabled (_centurylinklabs.watchtower.enable=true_ and `--label-enable` flag is set), it won't be monitored;

## Monitor Only

Individual containers can be marked to only be monitored (without being updated).

To do so, set the *com.centurylinklabs.watchtower.monitor-only* label to `true` on that container.

```docker
LABEL com.centurylinklabs.watchtower.monitor-only="true"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.monitor-only=true someimage
```

When the label is specified on a container, watchtower treats that container exactly as if [`WATCHTOWER_MONITOR_ONLY`](https://containrrr.dev/watchtower/arguments/#without_updating_containers) was set, but the effect is limited to the individual container. 
