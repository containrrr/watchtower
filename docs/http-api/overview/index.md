# API

## Overview

Watchtower has an optional HTTP API server that is enabled by configuring the  [`http-api-endpoints`](../../configuration/http-api/index.md#http_api_endpoints) and [`http-api-token`](../../configuration/http-api/index.md#http_api_token) configuration options and [publishing](https://docs.docker.com/engine/network/port-publishing/){target="_blank" rel="noopener noreferrer"} the HTTP API port (default: 8080).

This HTTP API server extends Watchtower's functionality by providing HTTP endpoints that allows for programmatic access to functionality, such as checking the current status of monitored containers and requesting Watchtower to perform container updates.
Parameters, such as container and image name filters, can be used to further fine-tune HTTP requests.

!!! Warning "Deprecation of HTTP API Endpoint Configuration Options"

    The following options have been deprecated in favor of the unified [`http-api-endpoints`](../../configuration/http-api/index.md#http_api_endpoints) configuration option and will be removed with the v2.x.x release of Watchtower:

    - [`http-api-update`](../../configuration/http-api/index.md#http_api_update)
    - [`http-api-containers`](../../configuration/http-api/index.md#http_api_containers)
    - [`http-api-metrics`](../../configuration/http-api/index.md#http_api_metrics)

## Endpoints

The following endpoints can be enabled by using the[`http-api-endpoints`](../../configuration/http-api/index.md#http_api_endpoints) configuration option and the respective configuration value.

|                           **Name**                           | **Configuration Value** | **Method** |       **Endpoint**       |                                  **Auth**                                   |                                                                                **Parameters**                                                                                |                                              **Description**                                              |
|:------------------------------------------------------------:|:-----------------------:|:----------:|:------------------------:|:---------------------------------------------------------------------------:|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|:---------------------------------------------------------------------------------------------------------:|
|            [Update](../endpoints/update/index.md)            |        `update`         |   `POST`   |       `/v1/update`       |      [API token](../../configuration/http-api/index.md#http_api_token)      | [`image`](../endpoints/update/index.md#image_name), [`container`](../endpoints/update/index.md#container_name), [`async`](../endpoints/update/index.md#asynchronous_updates) |                   Triggers container updates and returns JSON results of the operation                    |
|             [Check](../endpoints/check/index.md)             |         `check`         |   `POST`   |       `/v1/check`        |      [API token](../../configuration/http-api/index.md#http_api_token)      |                                 [`image`](../endpoints/check/index.md#image_name), [`container`](../endpoints/check/index.md#container_name)                                 |                     Checks containers for available updates (registry digest or Git ref)                    |
|        [Containers](../endpoints/containers/index.md)        |      `containers`       |   `GET`    |     `/v1/containers`     |      [API token](../../configuration/http-api/index.md#http_api_token)      |                              [`name`](../endpoints/containers/index.md#container_name), [`image`](../endpoints/containers/index.md#image_name)                               |                     Lists watched containers and their current running image digests                      |
| [Container Details](../endpoints/container-details/index.md) |      `containers`       |   `GET`    | `/v1/containers/details` |      [API token](../../configuration/http-api/index.md#http_api_token)      |                       [`name`](../endpoints/container-details/index.md#container_name), [`image`](../endpoints/container-details/index.md#image_name)                        | Returns detailed information about each watched container including running state and configuration flags |
|           [History](../endpoints/history/index.md)           |        `history`        |   `GET`    |      `/v1/history`       |      [API token](../../configuration/http-api/index.md#http_api_token)      |                [`since`](../endpoints/history/index.md#since), [`until`](../endpoints/history/index.md#until), [`limit`](../endpoints/history/index.md#limit)                |            Returns historical scan results from the in-memory ring buffer (up to 500 entries)             |
|            [Images](../endpoints/images/index.md)            |        `images`         |   `GET`    |       `/v1/images`       |      [API token](../../configuration/http-api/index.md#http_api_token)      |                                       [`name`](../endpoints/images/index.md#image_name), [`id`](../endpoints/images/index.md#image_id)                                       |                   Lists tracked images with their current digests and container counts                    |
|            [Config](../endpoints/config/index.md)            |        `config`         |   `GET`    |       `/v1/config`       |      [API token](../../configuration/http-api/index.md#http_api_token)      |                                                                                                                                                                              |                           Returns the active Watchtower configuration settings                            |
|            [Events](../endpoints/events/index.md)            |        `events`         |   `GET`    |       `/v1/events`       | [Events token](../../configuration/http-api/index.md#http_api_events_token) |                                                                                                                                                                              |                        Streams real-time operational events via Server-Sent Events                        |
|            [Status](../endpoints/status/index.md)            |        `metrics`        |   `GET`    |       `/v1/status`       |      [API token](../../configuration/http-api/index.md#http_api_token)      |                                                                                                                                                                              |                                Returns the summary of the most recent scan                                |
|           [Metrics](../endpoints/metrics/index.md)           |        `metrics`        |   `GET`    |      `/v1/metrics`       |      [API token](../../configuration/http-api/index.md#http_api_token)      |                                                                                                                                                                              |                     Exposes Prometheus-compatible metrics for monitoring and alerting                     |
|           [Swagger](../endpoints/swagger/index.md)           |        `swagger`        |   `GET`    |       `/swagger/*`       |                                    None                                     |                                                                                                                                                                              |                               Interactive API documentation via Swagger UI                                |
|           [Liveness](../endpoints/health/index.md)           |        `health`         |   `GET`    |         `/livez`         |                                    None                                     |                                                                                                                                                                              |                                Returns `200 OK` when the server is running                                |
|          [Readiness](../endpoints/health/index.md)           |        `health`         |   `GET`    |        `/readyz`         |                                    None                                     |                                                                                                                                                                              |                     Returns `200 OK` when Docker client is connected, `503` otherwise                     |
|           [Startup](../endpoints/health/index.md)            |        `health`         |   `GET`    |       `/startupz`        |                                    None                                     |                                                                                                                                                                              |                               Returns `200 OK` once the server has started                                |

!!! Note
    - Endpoints enforce HTTP method restrictions using method-based routing.
    - Requests with unsupported methods will receive a `405 Method Not Allowed` response.
    - Watchtower will not start if [`http-api-endpoints`](../../configuration/http-api/index.md#http_api_endpoints) has incorrect values or is combined with `all` (which enables all endpoints).

## Examples

=== "Enable Check Endpoint"

    === "Docker Compose"

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                environment:
                    - WATCHTOWER_HTTP_API_ENDPOINTS=check
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                ports:
                    - 8080:8080
                restart: unless-stopped
        ```

    === "Docker CLI"

        ```bash
        docker run -d \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=check \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

=== "Enable Update Endpoint"

    === "Docker Compose"
        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                environment:
                    - WATCHTOWER_HTTP_API_ENDPOINTS=update
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                ports:
                    - 8080:8080
                restart: unless-stopped
        ```

    === "Docker CLI"

        ```bash
        docker run -d \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=update \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

=== "Enable Swagger Endpoint"

    === "Docker Compose"

        ```yaml
        services:
            watchtower:
                image: nickfedor/watchtower:latest
                volumes:
                    - /var/run/docker.sock:/var/run/docker.sock
                environment:
                    - WATCHTOWER_HTTP_API_ENDPOINTS=swagger
                    - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                ports:
                    - 8080:8080
                restart: unless-stopped
        ```

    === "Docker CLI"

        ```bash
        docker run -d \
            --name watchtower \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -e WATCHTOWER_HTTP_API_ENDPOINTS=swagger \
            -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
            -p 8080:8080 \
            --restart unless-stopped \
            nickfedor/watchtower
        ```

## Security considerations

Watchtower was originally designed to be a stateless application with direct access to the Docker socket.
By having direct access to the Docker socket, the application effectively runs with root-level privileges.
It is worth taking a moment to review and familiarize yourself with the [Docker Engine security documentation](https://docs.docker.com/engine/security/){target="_blank" rel="noopener noreferrer"} and explore other security tools such as [Docker socket proxies](../../advanced-features/secure-connections/index.md#docker_socket_proxies).

- !!! Warning "The HTTP API should never be directly exposed to the Internet!"
- !!! Warning "Using the HTTP API without TLS encryption is insecure and not recommended!"
- !!! Warning "Only enable the endpoints that you need!"

Authentication, TLS, trusted proxies, CORS, and rate limiting are covered under [Authentication](../configuration/authentication/index.md), [TLS](../configuration/tls/index.md), and the [HTTP API configuration reference](../../configuration/http-api/index.md).
