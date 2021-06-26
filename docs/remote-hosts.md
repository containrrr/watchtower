By default, watchtower is set-up to monitor the local Docker daemon (the same daemon running the watchtower container itself). However, it is possible to configure watchtower to monitor a remote Docker endpoint. When starting the watchtower container you can specify a remote Docker endpoint with either the `--host` flag or the `DOCKER_HOST` environment variable:

```bash
docker run -d \
  --name watchtower \
  containrrr/watchtower --host "tcp://10.0.1.2:2375"
```

or

```bash
docker run -d \
  --name watchtower \
  -e DOCKER_HOST="tcp://10.0.1.2:2375" \
  containrrr/watchtower
```

Note in both of the examples above that it is unnecessary to mount the _/var/run/docker.sock_ into the watchtower container.
