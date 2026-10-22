# Remote Hosts

By default, Watchtower is set-up to monitor the local Docker daemon (the same daemon running the Watchtower container itself).
However, it is possible to configure Watchtower to monitor a remote Docker endpoint.
When starting the Watchtower container you can specify a remote Docker endpoint with either the `--host` flag or the `DOCKER_HOST` environment variable:

```bash
docker run -d \
  --name watchtower \
  nickfedor/watchtower --host "tcp://10.0.1.2:2375"
```

or

```bash
docker run -d \
  --name watchtower \
  -e DOCKER_HOST="tcp://10.0.1.2:2375" \
  nickfedor/watchtower
```

Note in both of the examples above that it is unnecessary to mount the _/var/run/docker.sock_ into the Watchtower container.
