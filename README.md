# Watchtower
![Watchtower](http://panamax.ca.tier3.io/zodiac/logo-watchtower_thumb.png)

[![Circle CI](https://circleci.com/gh/CenturyLinkLabs/watchtower.svg?style=svg)](https://circleci.com/gh/CenturyLinkLabs/watchtower)&nbsp;
[![GoDoc](https://godoc.org/github.com/CenturyLinkLabs/watchtower?status.svg)](https://godoc.org/github.com/CenturyLinkLabs/watchtower)&nbsp;
[![](https://badge.imagelayers.io/centurylink/watchtower:latest.svg)](https://imagelayers.io/?images=centurylink/watchtower:latest 'Get your own badge on imagelayers.io')

A process for watching your Docker containers and automatically restarting them whenever their base image is refreshed.

## Overview

Watchtower is an application that will monitor your running Docker containers and watch for changes to the images that those containers were originally started from. If watchtower detects that an image has changed, it will automatically restart the container using the new image.

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially.

For example, let's say you were running watchtower along with an instance of *centurylink/wetty-cli* image:

```
$ docker ps
CONTAINER ID   IMAGE                   STATUS          PORTS                    NAMES
967848166a45   centurylink/wetty-cli   Up 10 minutes   0.0.0.0:8080->3000/tcp   wetty
6cc4d2a9d1a5   centurylink/watchtower  Up 15 minutes                            watchtower
```

Every few mintutes watchtower will pull the latest *centurylink/wetty-cli* image and compare it to the one that was used to run the "wetty" container. If it sees that the image has changed it will stop/remove the "wetty" container and then restart it using the new image and the same `docker run` options that were used to start the container initially (in this case, that would include the `-p 8080:3000` port mapping).

## Usage

Watchtower is itself packaged as a Docker container so installation is as simple as pulling the `centurylink/watchtower` image.

Since the watchtower code needs to interact with the Docker API in order to monitor the running containers, you need to mount */var/run/docker.sock* into the container with the -v flag when you run it.

Run the `watchtower` container with the following command:

```
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  centurylink/watchtower
```

For private images:

```
docker run -d \
  --name watchtower \
  -e REPO_USER="username" -e REPO_PASS="pass" -e REPO_EMAIL="email" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  drud/watchtower container_to_watch --debug
```

### Arguments

By default, watchtower will monitor all containers running within the Docker daemon to which it is pointed (in most cases this will be the local Docker daemon, but you can override it with the `--host` option described in the next section). However, you can restrict watchtower to monitoring a subset of the running containers by specifying the container names as arguments when launching watchtower.

```
docker run -d \
  --name watchtower \
  -v /var/run/docker.sock:/var/run/docker.sock \
  centurylink/watchtower nginx redis
```

In the example above, watchtower will only monitor the containers named "nginx" and "redis" for updates -- all of the other running containers will be ignored.

When no arguments are specified, watchtower will monitor all running containers.

### Options

Any of the options described below can be passed to the watchtower process by setting them after the image name in the `docker run` string:

```
docker run --rm centurylink/watchtower --help
```

* `--host, -h` Docker daemon socket to connect to. Defaults to "unix:///var/run/docker.sock" but can be pointed at a remote Docker host by specifying a TCP endpoint as "tcp://hostname:port". The host value can also be provided by setting the `DOCKER_HOST` environment variable.
* `--interval, -i` Poll interval (in seconds). This value controls how frequently watchtower will poll for new images. Defaults to 300 seconds (5 minutes).
* `--no-pull` Do not pull new images. When this flag is specified, watchtower will not attempt to pull new images from the registry. Instead it will only monitor the local image cache for changes. Use this option if you are building new images directly on the Docker host without pushing them to a registry.
* `--cleanup` Remove old images after updating. When this flag is specified, watchtower will remove the old image after restarting a container with a new image. Use this option to prevent the accumulation of orphaned images on your system as containers are updated.
* `--tls` Use TLS when connecting to the Docker socket but do NOT verify the server's certificate. If you are connecting a TCP Docker socket protected by TLS you'll need to use either this flag or the `--tlsverify` flag (described below). The `--tlsverify` flag is preferred as it will cause the server's certificate to be verified before a connection is made.
* `--tlsverify` Use TLS when connecting to the Docker socket and verify the server's certificate. If you are connecting a TCP Docker socket protected by TLS you'll need to use either this flag or the `--tls` flag (describe above).  
* `--tlscacert` Trust only certificates signed by this CA. Used in conjunction with the `--tlsverify` flag to identify the CA certificate which should be used to verify the identity of the server. The value for this flag can be either the fully-qualified path to the *.pem* file containing the CA certificate or a string containing the CA certificate itself. Defaults to "/etc/ssl/docker/ca.pem".
* `--tlscert` Client certificate for TLS authentication. Used in conjunction with the `--tls` or `--tlsverify` flags to identify the certificate to use for client authentication. The value for this flag can be either the fully-qualified path to the *.pem* file containing the client certificate or a string containing the certificate itself. Defaults to "/etc/ssl/docker/cert.pem".
* `--tlskey` Client key for TLS authentication. Used in conjunction with the `--tls` or `--tlsverify` flags to identify the key to use for client authentication. The value for this flag can be either the fully-qualified path to the *.pem* file containing the client key or a string containing the key itself. Defaults to "/etc/ssl/docker/key.pem".
* `--debug` Enable debug mode. When this option is specified you'll see more verbose logging in the watchtower log file.
* `--help` Show documentation about the supported flags.

## Linked Containers

Watchtower will detect if there are links between any of the running containers and ensure that things are stopped/started in a way that won't break any of the links. If an update is detected for one of the dependencies in a group of linked containers, watchtower will stop and start all of the containers in the correct order so that the application comes back up correctly.

For example, imagine you were running a *mysql* container and a *wordpress* container which had been linked to the *mysql* container. If watchtower were to detect that the *mysql* container required an update, it would first shut down the linked *wordpress* container followed by the *mysql* container. When restarting the containers it would handle *mysql* first and then *wordpress* to ensure that the link continued to work.

## Stopping Containers

When watchtower detects that a running container needs to be updated it will stop the container by sending it a SIGTERM signal.
If your container should be shutdown with a different signal you can communicate this to watchtower by setting a label named *com.centurylinklabs.watchtower.stop-signal* with the value of the desired signal.

This label can be coded directly into your image by using the `LABEL` instruction in your Dockerfile:

```
LABEL com.centurylinklabs.watchtower.stop-signal="SIGHUP"
```

Or, it can be specified as part of the `docker run` command line:

```
docker run -d --label=com.centurylinklabs.watchtower.stop-signal=SIGHUP someimage
```

## Remote Hosts

By default, watchtower is set-up to monitor the local Docker daemon (the same daemon running the watchtower container itself). However, it is possible to configure watchtower to monitor a remote Docker endpoint. When starting the watchtower container you can specify a remote Docker endpoint with either the `--host` flag or the `DOCKER_HOST` environment variable:

```
docker run -d \
  --name watchtower \
  centurylink/watchtower --host "tcp://10.0.1.2:2375"
```

or

```
docker run -d \
  --name watchtower \
  -e DOCKER_HOST="tcp://10.0.1.2:2375" \
  centurylink/watchtower
```

Note in both of the examples above that it is unnecessary to mount the */var/run/docker.sock* into the watchtower container.

### Secure Connections

Watchtower is also capable of connecting to Docker endpoints which are protected by SSL/TLS. If you've used *docker-machine* to provision your remote Docker host, you simply need to volume mount the certificates generated by *docker-machine* into the watchtower container and specify either the `--tls` or `--tlsverify` flags.

The *docker-machine* certificates for a particular host can be located by executing the `docker-machine env` command for the desired host (note the values for the `DOCKER_HOST` and `DOCKER_CERT_PATH` environment variables that are returned from this command). The directory containing the certificates for the remote host needs to be mounted into the watchtower container at */etc/ssl/docker*.

With the certificates mounted into the watchtower container you simply need to specify either the `--tls` or `--tlsverify` flags to enable the TLS connection to the remote host:

```
docker run -d \
  --name watchtower \
  -e DOCKER_HOST=$DOCKER_HOST \
  -v $DOCKER_CERT_PATH:/etc/ssl/docker \
  centurylink/watchtower --tlsverify
```

## Updating Watchtower

If watchtower is monitoring the same Docker daemon under which the watchtower container itself is running (i.e. if you volume-mounted */var/run/docker.sock* into the watchtower container) then it has the ability to update itself. If a new version of the *centurylink/watchtower* image is pushed to the Docker Hub, your watchtower will pull down the new image and restart itself automatically.
