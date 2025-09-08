<p style="text-align: center; margin-left: 1.6rem;">
  <img alt="Logotype depicting a lighthouse" src="./images/logo-450px.png" width="450" />
</p>
<h1 align="center">
  Watchtower
</h1>

<p align="center">
  A container-based solution for automating Docker container base image updates.
  <br/><br/>
  <a href="https://codecov.io/gh/beatkind/watchtower">
    <img alt="Codecov" src="https://codecov.io/gh/beatkind/watchtower/branch/main/graph/badge.svg">
  </a>
  <a href="https://godoc.org/github.com/beatkind/watchtower">
    <img alt="GoDoc" src="https://godoc.org/github.com/beatkind/watchtower?status.svg" />
  </a>
  <a href="https://goreportcard.com/report/github.com/beatkind/watchtower">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/beatkind/watchtower" />
  </a>
  <a href="https://github.com/beatkind/watchtower/releases">
    <img alt="latest version" src="https://img.shields.io/github/tag/beatkind/watchtower.svg" />
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img alt="Apache-2.0 License" src="https://img.shields.io/github/license/beatkind/watchtower.svg" />
  </a>
  <a href="https://hub.docker.com/r/beatkind/watchtower">
    <img alt="Pulls from DockerHub" src="https://img.shields.io/docker/pulls/beatkind/watchtower.svg" />
  </a>
</p>

# Overview

!!! note "Watchtower fork"
    This is a fork of the really nice project from [containrrr](https://github.com/containrrr) called [watchtower](https://github.com/containrrr/watchtower).
    I am not the original author of this project. I just forked it to make some changes to it and keep it up-to-date as properly as I can.
    Contributions, tips and hints are welcome. Just open an issue or a pull request. Please be aware that I am by no means a professional developer. I am just a Platform Engineer.

## Quick Start

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker
Hub or your own image registry. Watchtower will pull down your new image, gracefully shut down your existing container
and restart it with the same options that were used when it was deployed initially. Run the watchtower container with
the following command:

=== "docker run"

    ```bash
    $ docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    beatkind/watchtower
    ```

=== "docker-compose.yml"

    ```yaml
    services:
      watchtower:
        image: beatkind/watchtower
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```
