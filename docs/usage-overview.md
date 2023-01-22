Watchtower is itself packaged as a Docker container so installation is as simple as pulling the `containrrr/watchtower` image. If you are using ARM based architecture, pull the appropriate `containrrr/watchtower:armhf-<tag>` image from the [containrrr Docker Hub](https://hub.docker.com/r/containrrr/watchtower/tags/).

Since the watchtower code needs to interact with the Docker API in order to monitor the running containers, you need to mount _/var/run/docker.sock_ into the container with the `-v` flag when you run it.

Run the `watchtower` container with the following command:

```bash
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  containrrr/watchtower
```

If pulling images from private Docker registries, supply registry authentication credentials with the environment variables `REPO_USER` and `REPO_PASS`
or by mounting the host's docker config file into the container (at the root of the container filesystem `/`).

Passing environment variables:

```bash
docker run -d \
  --name watchtower \
  -e REPO_USER=username \
  -e REPO_PASS=password \
  -v /var/run/docker.sock:/var/run/docker.sock \
  containrrr/watchtower container_to_watch --debug
```

Also check out [this Stack Overflow answer](https://stackoverflow.com/a/30494145/7872793) for more options on how to pass environment variables.

Alternatively if you 2FA authentication setup on Docker Hub then passing username and password will be insufficient.  Instead you can run `docker login` to store your credentials in `$HOME/.docker/config.json` and then mount this config file to make it available to the Watchtower container:

```bash
docker run -d \
  --name watchtower \
  -v $HOME/.docker/config.json:/config.json \
  -v /var/run/docker.sock:/var/run/docker.sock \
  containrrr/watchtower container_to_watch --debug
```

!!! note "Changes to config.json while running"
    If you mount `config.json` in the manner above, changes from the host system will (generally) not be propagated to the
    running container. Mounting files into the Docker daemon uses bind mounts, which are based on inodes. Most
    applications (including `docker login` and `vim`) will not directly edit the file, but instead make a copy and replace
    the original file, which results in a new inode which in turn _breaks_ the bind mount.  
    **As a workaround**, you can create a symlink to your `config.json` file and then mount the symlink in the container. 
    The symlinked file will always have the same inode, which keeps the bind mount intact and will ensure changes
    to the original file are propagated to the running container (regardless of the inode of the source file!).

If you mount the config file as described above, be sure to also prepend the URL for the registry when starting up your
watched image (you can omit the https://). Here is a complete docker-compose.yml file that starts up a docker container
from a private repo on the GitHub Registry and monitors it with watchtower. Note the command argument changing the interval
to 30s rather than the default 24 hours.

```yaml
version: "3"
services:
  cavo:
    image: ghcr.io/<org>/<image>:<tag>
    ports:
      - "443:3443"
      - "80:3080"
  watchtower:
    image: containrrr/watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /root/.docker/config.json:/config.json
    command: --interval 30
```
