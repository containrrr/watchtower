# TLS

## Overview

Watchtower's HTTP API supports TLS (HTTPS) to encrypt traffic between clients and the API server.

By default, the HTTP API uses unencrypted HTTP.
Enabling TLS ensures that all API requests, including authenticated ones, are protected against eavesdropping and man-in-the-middle attacks.

!!! Warning "Using the HTTP API without TLS encryption is insecure and not recommended!"
    See the [HTTP API overview](../../overview/index.md#security_considerations) for important security guidance.

## Configuration

TLS is enabled by providing both of the following configuration options:

- [HTTP API TLS Certificate](../../../configuration/http-api/index.md#http_api_tls_certificate): Path (inside the container) to the TLS certificate file (PEM format).
- [HTTP API TLS Key](../../../configuration/http-api/index.md#http_api_tls_key): Path (inside the container) to the TLS private key file (PEM format).

Both options must be set together.
The server will use HTTPS when both are provided.

The certificate and key files must be accessible inside the Watchtower container, typically via a bind mount or Docker secret.

## Walkthrough: Enabling TLS for the HTTP API

### Step 1: Obtain a Certificate and Private Key

You need a valid X.509 certificate and matching private key.

#### Production Use

Use certificates from a trusted certificate authority such as:

- [Let's Encrypt](https://letsencrypt.org/){target="_blank" rel="noopener noreferrer"}
- Your organization's internal CA
- Cloud provider certificate services

#### Testing / Internal Use

You can generate a self-signed certificate for testing:

- Generate a private key:

    ```bash
    openssl genrsa -out watchtower.key 2048
    ```

- Generate a self-signed certificate (valid 365 days):

    ````bash
    openssl req -new -x509 -sha256 -key watchtower.key -out watchtower.crt -days 365 \
    -subj "/C=US/ST=State/L=City/O=Organization/CN=watchtower.example.com"
    ````

!!! Warning "Self-signed certificates will cause browser and client warnings."
    For production or any environment where clients cannot easily trust the certificate, use certificates signed by a trusted CA.

### Step 2: Prepare Files for the Container

Place `watchtower.crt` and `watchtower.key` in a directory on the host (e.g., `/opt/watchtower/certs/`).

Ensure proper permissions:

```bash
chmod 644 watchtower.crt
chmod 600 watchtower.key
```

### Step 3: Mount and Configure Watchtower

Mount the certificate directory into the container, enable the metrics API (providing the `/v1/metrics` endpoint referenced in Step 4), and configure the TLS certificate and key paths.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower:latest
            volumes:
                - /var/run/docker.sock:/var/run/docker.sock
                - /opt/watchtower/certs:/certs:ro
            environment:
                - WATCHTOWER_HTTP_API_TOKEN=your-secure-token
                # Enables the /v1/metrics endpoint (used in Step 4)
                - WATCHTOWER_HTTP_API_ENDPOINTS=metrics
                - WATCHTOWER_HTTP_API_TLS_CERT=/certs/watchtower.crt
                - WATCHTOWER_HTTP_API_TLS_KEY=/certs/watchtower.key
            ports:
                - "8080:8080"
            restart: unless-stopped
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
        --name watchtower \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /opt/watchtower/certs:/certs:ro \
        -e WATCHTOWER_HTTP_API_TOKEN=your-secure-token \
        -e WATCHTOWER_HTTP_API_ENDPOINTS=metrics \
        -e WATCHTOWER_HTTP_API_TLS_CERT=/certs/watchtower.crt \
        -e WATCHTOWER_HTTP_API_TLS_KEY=/certs/watchtower.key \
        -p 8080:8080 \
        --restart unless-stopped \
        nickfedor/watchtower
    ```

### Step 4: Connect Using HTTPS

Clients must use `https://` when connecting:

```bash
curl -H "Authorization: Bearer your-secure-token" https://localhost:8080/v1/metrics
```

For self-signed certificates, you may need to:

- Add the certificate to your system's trust store, or
- Use the `--insecure` / `-k` flag with tools like `curl` (not recommended for production)

Example with self-signed certificate:

```bash
curl -k -H "Authorization: Bearer your-secure-token" https://localhost:8080/v1/metrics
```

## Important Considerations

- The listening [HTTP API Port](../../../configuration/http-api/index.md#http_api_port) remains the same; only the protocol changes to HTTPS.
- All endpoints (including unauthenticated health probes) are served over HTTPS when TLS is enabled.
- [Authentication](../authentication/index.md) is still required for protected endpoints.
- Ensure certificate files are mounted read-only (`:ro`) where possible.
- Certificate rotation requires restarting the Watchtower container.
- Hostname / SANs in the certificate must match how clients connect (especially important for production certificates).

## Related Documentation

- [HTTP API Overview](../../overview/index.md)
- [HTTP API Host and Port](../host-and-port/index.md)
- [HTTP API Authentication](../authentication/index.md)
- [HTTP API Configuration](../../../configuration/http-api/index.md)
- [Secure Connections (Docker daemon TLS)](../../../advanced-features/secure-connections/index.md)
