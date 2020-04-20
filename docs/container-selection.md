By default, watchtower will watch all containers. However, sometimes only some containers should be updated.

If you need to exclude some containers, set the _com.centurylinklabs.watchtower.enable_ label to `false`.

```docker
LABEL com.centurylinklabs.watchtower.enable="false"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=false someimage
```

If you need to [include only containers with the enable label](https://containrrr.github.io/watchtower/arguments/#filter_by_enable_label), pass the `--label-enable` flag or the  `WATCTOWER_LABEL_ENABLE` environment variable on startup and set the _com.centurylinklabs.watchtower.enable_ label with a value of `true` for the containers you want to watch.

```docker
LABEL com.centurylinklabs.watchtower.enable="true"
```

Or, it can be specified as part of the `docker run` command line:

```bash
docker run -d --label=com.centurylinklabs.watchtower.enable=true someimage
```
