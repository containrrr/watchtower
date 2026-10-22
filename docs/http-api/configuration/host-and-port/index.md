# Configuration

## Overview

Watchtower defaults to listening on all IPv4 interfaces on port 8080.
This behavior can be changed by configuring the [HTTP API Host](../../../configuration/http-api/index.md#http_api_host) and [HTTP API Port](../../../configuration/http-api/index.md#http_api_port) configuration options

## Configuration Options

### HTTP API Host

Use the [HTTP API Host](../../../configuration/http-api/index.md#http_api_host) configuration option to bind to a specific host interface.

- This must be a valid IP address (IPv4 or IPv6).
- An IPv6 address binds that address. The unspecified IPv6 address `::` listens on all IPv6 interfaces, and also on all IPv4 interfaces on systems that support IPv4-mapped IPv6 addresses, such as Linux, macOS and Windows.
- If not specified, Watchtower listens on all IPv4 interfaces on the port specified by [HTTP API Port](../../../configuration/http-api/index.md#http_api_port).

!!! Warning "Default bind risk"
    When the host is unset, the HTTP API listens on all IPv4 network interfaces (`0.0.0.0`).
    This exposes the API to every reachable network.
    Set [`http-api-host`](../../../configuration/http-api/index.md#http_api_host) to a loopback or firewall-restricted interface unless the API is protected by an outer network policy (reverse proxy, VPN, etc.).

### HTTP API Port

The port can be changed using the [HTTP API Port](../../../configuration/http-api/index.md#http_api_port) configuration option.

If Watchtower is being run via a Docker container, then the `host:container` port mapping can be updated accordingly (e.g. `8080:8080` -> `9000:8080`).

## Examples

- Listen on all IPv4 interfaces on port 8080 (default):

  ```bash
  --http-api-port=8080
  ```

- Listen on localhost only on port 8080:

  ```bash
  --http-api-host=127.0.0.1 --http-api-port=8080
  ```

- Listen on the IPv6 localhost only on port 8080:

  ```bash
  --http-api-host=::1 --http-api-port=8080
  ```

- Listen on all IPv6 interfaces, and on all IPv4 interfaces where supported, on port 8080:

  ```bash
  --http-api-host=:: --http-api-port=8080
  ```

- Listen on a specific IP and port:

  ```bash
  --http-api-host=192.168.1.100 --http-api-port=9090
  ```
