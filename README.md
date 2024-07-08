<div align="center">
  <img src="./logo.png" width="450" />
  
  # Watchtower
  
  A process for automating Docker container base image updates.
  <br/><br/>
  
  [![codecov](https://codecov.io/gh/beatkind/watchtower/branch/main/graph/badge.svg)](https://codecov.io/gh/beatkind/watchtower)
  [![GoDoc](https://godoc.org/github.com/beatkind/watchtower?status.svg)](https://godoc.org/github.com/beatkind/watchtower)
  [![Go Report Card](https://goreportcard.com/badge/github.com/beatkind/watchtower)](https://goreportcard.com/report/github.com/beatkind/watchtower)
  [![latest version](https://img.shields.io/github/tag/beatkind/watchtower.svg)](https://github.com/beatkind/watchtower/releases)
  [![Apache-2.0 License](https://img.shields.io/github/license/beatkind/watchtower.svg)](https://www.apache.org/licenses/LICENSE-2.0)
  [![Pulls from DockerHub](https://img.shields.io/docker/pulls/beatkind/watchtower.svg)](https://hub.docker.com/r/beatkind/watchtower)

</div>

## Quick Start

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. 

Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially. Run the watchtower container with the following command:

```
$ docker run --detach \
    --name watchtower \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    beatkind/watchtower
```

Watchtower is intended to be used in homelabs, media centers, local dev environments, and similar. We do **not** recommend using Watchtower in a commercial or production environment. If that is you, you should be looking into using Kubernetes. If that feels like too big a step for you, please look into solutions like [MicroK8s](https://microk8s.io/) and [k3s](https://k3s.io/) that take away a lot of the toil of running a Kubernetes cluster.

## Documentation
The full documentation is available at https://beatkind.dev/watchtower.
