By default, Watchtower will clean up other instances and won't allow multiple instances monitoring the same containers. It is possible to override this behavior by defining a [scopeUID](https://containrrr.github.io/watchtower/arguments/#filter_by_scope) to each running instance. 

Notice that:
- Multiple instances can run with the same scope;
- An instance without a scope will clean up other running instances, even if they have a defined scope;

To define an instance monitoring scope, use the `--scope-uid` argument or the `WATCHTOWER_SCOPE_UID` environment variable on startup and set the _com.centurylinklabs.watchtower.scope-uid_ label with the same value for the containers you want to include in this instance's scope.

For example, in a Docker Compose config file:

```json
version: '3'

services:
  app-monitored-by-watchtower:
    image: myapps/monitored-by-watchtower
    labels:
      - "com.centurylinklabs.watchtower.enable=true"
      - "com.centurylinklabs.watchtower.scope-uid=myscope"

  watchtower:
    image: containrrr/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    command: --debug --interval 30
    environment:
      - WATCHTOWER_SCOPE_UID=myscope
    labels:
      - "com.centurylinklabs.watchtower.enable=false"
    ports:
      - 8080:8080
```