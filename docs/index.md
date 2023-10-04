<p style="text-align: center; margin-left: 1.6rem;">
  <img alt="Logotype depicting a lighthouse" src="./images/logo-450px.png" width="450" />
</p>
<h1 align="center">
  Watchtower
</h1>

<p align="center">
  A container-based solution for automating Docker container base image updates.
  <br/><br/>
  <a href="https://circleci.com/gh/nicholas-fedor/watchtower">
    <img alt="Circle CI" src="https://circleci.com/gh/nicholas-fedor/watchtower.svg?style=shield" />
  </a>
  <a href="https://codecov.io/gh/nicholas-fedor/watchtower">
    <img alt="Codecov" src="https://codecov.io/gh/nicholas-fedor/watchtower/branch/main/graph/badge.svg">
  </a>
  <a href="https://godoc.org/github.com/nicholas-fedor/watchtower">
    <img alt="GoDoc" src="https://godoc.org/github.com/nicholas-fedor/watchtower?status.svg" />
  </a>
  <a href="https://goreportcard.com/report/github.com/nicholas-fedor/watchtower">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/nicholas-fedor/watchtower" />
  </a>
  <a href="https://github.com/nicholas-fedor/watchtower/releases">
    <img alt="latest version" src="https://img.shields.io/github/tag/nicholas-fedor/watchtower.svg" />
  </a>
  <a href="https://www.apache.org/licenses/LICENSE-2.0">
    <img alt="Apache-2.0 License" src="https://img.shields.io/github/license/nicholas-fedor/watchtower.svg" />
  </a>
  <a href="https://www.codacy.com/gh/nicholas-fedor/watchtower/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=nicholas-fedor/watchtower&amp;utm_campaign=Badge_Grade">
    <img alt="Codacy Badge" src="https://app.codacy.com/project/badge/Grade/1c48cfb7646d4009aa8c6f71287670b8"/>
  </a>
  <a href="https://github.com/nicholas-fedor/watchtower/#contributors">
    <img alt="All Contributors" src="https://img.shields.io/github/all-contributors/nicholas-fedor/watchtower" />
  </a>
  <a href="https://hub.docker.com/r/nickfedor/watchtower">
    <img alt="Pulls from DockerHub" src="https://img.shields.io/docker/pulls/nickfedor/watchtower.svg" />
  </a>
</p>

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
    nickfedor/watchtower
    ```

=== "docker-compose.yml"

    ```yaml
    version: "3"
    services:
      watchtower:
        image: nickfedor/watchtower
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock
    ```
