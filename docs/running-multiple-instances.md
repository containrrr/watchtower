By default, Watchtower will clean up other instances and won't allow multiple instances running on the same Docker host or swarm. It is possible to override this behavior by defining a [scope](https://containrrr.github.io/watchtower/arguments/#filter_by_scope) to each running instance. 

Notice that:
-   Multiple instances can't run with the same scope;
-   An instance without a scope will clean up other running instances, even if they have a defined scope;

To define an instance monitoring scope, use the `--scope` argument or the `WATCHTOWER_SCOPE` environment variable on startup and set the _com.centurylinklabs.watchtower.scope_ label with the same value for the containers you want to include in this instance's scope (including the instance itself).

For example, in a Docker Compose config file:

```yaml
version: '3'

services:
  app-monitored-by-watchtower:
    image: myapps/monitored-by-watchtower
    labels:
      - "com.centurylinklabs.watchtower.scope=myscope"

  watchtower:
    image: containrrr/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --interval 30 --scope myscope
    labels:
      - "com.centurylinklabs.watchtower.scope=myscope"
```
