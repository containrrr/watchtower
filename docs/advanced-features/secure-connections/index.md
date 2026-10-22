# Securely Connecting Watchtower to Docker

## Overview

Watchtower supports secure TLS connections to Docker hosts through its usage of [Docker's Go SDK](https://docs.docker.com/reference/api/engine/sdk/){target="_blank" rel="noopener noreferrer"} to create a Docker client.

It is highly recommended to review Docker's documentation:

- <https://docs.docker.com/engine/daemon/remote-access/>{target="_blank" rel="noopener noreferrer"}
- <https://docs.docker.com/engine/security/protect-access>{target="_blank" rel="noopener noreferrer"}
- <https://docs.docker.com/reference/cli/dockerd/#daemon-socket-option>{target="_blank" rel="noopener noreferrer"}

!!! Note
    Docker has [retired](https://docs.docker.com/retired/#docker-machine){target="_blank" rel="noopener noreferrer"} the [Docker Machine](https://github.com/docker-archive-public/docker.machine){target="_blank" rel="noopener noreferrer"} project that was previously noted in Watchtower's documentation.

## Configuration

### TLS Verification

```text
            Argument: --tlsverify
Environment Variable: DOCKER_TLS_VERIFY
                Type: Boolean
             Default: false
```

!!! Warning "`http://` and `unix://` schemes are incompatible with TLS verification."

!!! Note "When TLS verification is enabled"
    - The use of `tcp://` or `https://` schemes for the Docker host URL is required.
    - `tcp://` is converted to `https://` when TLS verification is enabled.

### TLS Certificate Path

```text
            Argument: --cert-path
Environment Variable: DOCKER_CERT_PATH
                Type: String
             Default: /etc/ssl/docker
```

!!! Notes
    - This specifies the directory where Watchtower's Docker client should find the certificate files within the Watchtower container.
    - Docker expects the following filenames:
        - `ca.pem`
        - `cert.pem`
        - `key.pem`

### Docker Host URL

```text
            Argument: --host
Environment Variable: DOCKER_HOST
                Type: String
             Default: unix:///var/run/docker.sock
```

!!! Warning "Specifying multiple connections, such as both the local socket (i.e. `/var/run/docker.sock`) and a remote host is not supported."

!!! Notes
    - This is required for connections to any Docker host.
    - The use of `tcp://` or `https://` schemes is required when using a TLS connection.
    - `tcp://` is internally converted to `https://` when TLS verification is enabled.

### Docker API Version

```text
            Argument: --api-version
Environment Variable: DOCKER_API_VERSION
                Type: String
             Default: <Auto-negotiated>
```

!!! Notes
    - This provides the ability to manually specify the Docker API version.
    - The default version autonegotiation should be sufficient for normal use cases.

## Examples

!!! Note
    Replace `remote-host` with your actual Docker host address and `/path/to/certs` with the path to your certificate directory.

=== "Docker CLI"

    ```bash
    docker run -d \
      --name watchtower \
      -v /path/to/certs:/etc/ssl/docker:ro \
      nickfedor/watchtower --host tcp://remote-host:2376 --cert-path /etc/ssl/docker --tlsverify
    ```

    | Parameter                              | Description                                                                            |
    |----------------------------------------|----------------------------------------------------------------------------------------|
    | `--name watchtower`                    | Assigns the name "watchtower" to the container for easy identification and management. |
    | `-v /path/to/certs:/etc/ssl/docker:ro` | Mounts the local certificate directory to the container's SSL directory as read-only.  |
    | `nickfedor/watchtower`                 | Specifies the Docker image to run, which is the Watchtower container image.            |
    | `--host tcp://remote-host:2376`        | Sets the Docker host to connect to via TCP on port 2376.                               |
    | `--cert-path /etc/ssl/docker`          | Defines the path inside the container where TLS certificates are located.              |
    | `--tlsverify`                          | Enables TLS verification for secure connections to the Docker host.                    |

    !!! Tip
        If using `-e` flags to pass environment variables, then remember to place them before the `nickfedor/watchtower` image reference.

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower
            environment:
                - DOCKER_HOST=tcp://remote-host:2376
                - DOCKER_CERT_PATH=/etc/ssl/docker
                - DOCKER_TLS_VERIFY=1
            volumes:
                - /path/to/certs:/etc/ssl/docker:ro
            restart: unless-stopped
    ```

    | Parameter                              | Description                                                                           |
    |----------------------------------------|---------------------------------------------------------------------------------------|
    | `image: nickfedor/watchtower`          | Specifies the Docker image for the Watchtower service.                                |
    | `- DOCKER_HOST=tcp://remote-host:2376` | Sets the Docker host connection URL.                                      |
    | `- DOCKER_CERT_PATH=/etc/ssl/docker`   | Specifies the path to the directory containing TLS certificates.                      |
    | `- DOCKER_TLS_VERIFY=1`                | Enables TLS verification for secure connections.                                      |
    | `/path/to/certs:/etc/ssl/docker:ro`    | Mounts the local certificate directory to the container's SSL directory as read-only. |
    | `restart: unless-stopped`              | Configures the container to restart automatically unless it was manually stopped.     |

## Basic Tutorial

!!! Note "This is not a comprehensive guide and is merely a simple tutorial to illustrate a basic test deployment."
    The following tutorial is intended to provide a basic walkthrough for manually setting up Watchtower to perform container updates on a Docker host that has enabled access to the Docker daemon using TLS.

    This largely follow's [Docker's guide](https://docs.docker.com/engine/security/protect-access/#use-tls-https-to-protect-the-docker-daemon-socket){target="_blank" rel="noopener noreferrer"} for setting up TLS on the Docker host.

!!! Warning
    Configuring Docker to accept network connections has critical security implications that can leave you vulnerable to unauthorized access.
    You are **highly** encouraged to perform your own due diligence to mitigate these risks.

### Tutorial Overview

In order for Watchtower to connect via TLS to a Docker daemon, the Docker daemon must be setup to accept remote connections using TLS.

Setting up TLS for Docker involves several key steps:

  1. **Certificate Generation:**

    - Create a Certificate Authority (CA), server certificate, and client certificates using OpenSSL or similar tools.

    !!! Note "Server certificates must include the Docker host's IP or DNS name in the Subject Alternative Name (SAN) field."

  2. **Daemon Configuration:**

    Start the Docker daemon with TLS options:

    - `--tlsverify`: Enable TLS verification
    - `--tlscacert`: Path to CA certificate
    - `--tlscert`: Path to server certificate
    - `--tlskey`: Path to server private key

  3. **Client Setup:**

    - Prepare client certificates (`cert.pem` and `key.pem`) for authentication.

  4. **Environment Variables:**

    - Set `DOCKER_HOST` to the secure endpoint (e.g., `tcp://host:2376`)
    - Set `DOCKER_CERT_PATH` to the directory containing client certificates
    - Set `DOCKER_TLS_VERIFY=1` to enable verification

For detailed instructions, refer to the [Docker documentation on protecting the Docker daemon socket](https://docs.docker.com/engine/security/protect-access/#use-tls-https-to-protect-the-docker-daemon-socket){target="_blank" rel="noopener noreferrer"}.

### Certificate Generation

Generate self-signed certificates for testing (replace with proper certificates for production):

#### Create a CA Key and Certificate

1. Generate a 4096-bit RSA private key for the Certificate Authority and save it to `ca-key.pem`:

    ```bash
    openssl genrsa -aes256 -out ca-key.pem 4096
    ```

    !!! Warning "Make sure to take note of the passphrase"

2. Create a self-signed X.509 certificate for the Certificate Authority (valid for 365 days) using the private key and save it to `ca.pem`:

    ```bash
    openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem -subj "/C=US/ST=State/L=City/O=Org/CN=ca"
    ```

#### Create a Server Key and Certificate

1. Generate a 4096-bit RSA private key for the server certificate and save it to `server-key.pem`:

    ```bash
    openssl genrsa -out server-key.pem 4096
    ```

2. Generate a certificate signing request (CSR) for the server with common name "localhost" and save it to `server.csr`:

    ```bash
    openssl req -subj "/CN=$HOST" -sha256 -new -key server-key.pem -out server.csr
    ```

3. Sign the server CSR with the CA certificate, creating a server certificate valid for 365 days with the specified extensions, and save it to `server-cert.pem`:

    ```bash
    echo "subjectAltName = DNS:$HOST,IP:10.10.10.20,IP:127.0.0.1" > extfile-server.cnf
    ```

    !!! Tip "`IP:10.10.10.20` is an example IP. Replace with your host's actual IP."

    !!! Tip "`$HOST` typically resolves to the hostname. Change this as necessary (i.e. `localhost` for testing) "

    ```bash
    echo "extendedKeyUsage = serverAuth" >> extfile-server.cnf
    ```

    !!! Tip "This sets the Docker daemon key's extended usage attributes to be used only for server authentication."

    ```bash
    openssl x509 -req -days 365 -sha256 -in server.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extfile extfile-server.cnf
    ```

#### Create a Client Key and Certificate

1. Generate a 4096-bit RSA private key for the client certificate and save it to `key.pem`:

    ```bash
    openssl genrsa -out key.pem 4096
    ```

2. Generate a certificate signing request (CSR) for the client with common name "client" and save it to `client.csr`:

    ```bash
    openssl req -subj '/CN=client' -new -key key.pem -out client.csr
    ```

3. Sign the client CSR with the CA certificate, creating a client certificate valid for 365 days with the client authentication extension, and save it to `cert.pem`:

    ```bash
    echo "extendedKeyUsage = clientAuth" > extfile-client.cnf
    ```

    !!! Tip "This makes the key suitable for client authentication."

    ```bash
    openssl x509 -req -days 365 -sha256 -in client.csr -CA ca.pem -CAkey ca-key.pem -CAcreateserial -out cert.pem -extfile extfile-client.cnf
    ```

### Key and Certificate Management

1. Remove both the certificate signing requests (`client.csr` and `server.csr`) and extensions config files (`extfile-server.cnf` and `extfile-client.cnf`) after generating the server (`server-cert.pem`) and client `cert.pem` certificates:

    ```bash
    rm -v client.csr server.csr extfile-server.cnf extfile-client.cnf
    ```

2. Update the file permissions of the `ca-key.pem`, `server-key.pem`, and `key.pem` secret keys:

    ```bash
    sudo chmod -v 0400 ca-key.pem server-key.pem key.pem
    ```

3. Remove `ca.pem`, `server-cert.pem`, and `cert.pem` file write access:

    ```bash
    sudo chmod -v 0444 ca.pem server-cert.pem cert.pem
    ```

4. Create the `/etc/docker/certs` directory if it doesn't exist:

    ```bash
    sudo mkdir -p /etc/docker/certs
    ```

5. Copy the server-specific files:

    ```bash
    sudo cp ca.pem ca-key.pem server-cert.pem server-key.pem /etc/docker/certs/
    ```

6. (Optional): Copy the client-specific files:

    ```bash
    sudo cp cert.pem key.pem /etc/docker/certs/
    ```

7. Ensure root ownership:

    ```bash
    sudo chown root:root /etc/docker/certs/*
    ```

8. Set directory-level access:

    ```bash
    sudo chmod 0755 /etc/docker/certs/
    ```

9. Verify the files:

    ```bash
    ls -la /etc/docker/certs/
    ```

    !!! Tip "Output should show files with `-r--------` for keys and `-r--r--r--` for certs."

10. (Optional) Remove the original files (in the original directory, not `/etc/docker/certs`):

    ```bash
    rm -v {ca,ca-key,server-cert,server-key,cert,key}.pem
    ```

!!! Notes
    - The server files (`ca.pem`, `server-key.pem`, `server-cert.pem`) stay on the daemon host.
    - The client's files (`ca.pem`, `key.pem`, and `cert.pem`) can be moved the client's Docker directory (e.g. `~/.docker/`).

### Setup the Docker Daemon with TLS

!!! Note "This assumes Docker is already installed on the host system."

1. Edit or create `/etc/docker/daemon.json` to enable TLS and TCP listening:

    ```json
    {
      "tls": true,
      "tlscacert": "/etc/docker/certs/ca.pem",
      "tlscert": "/etc/docker/certs/server-cert.pem",
      "tlskey": "/etc/docker/certs/server-key.pem",
      "hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2376"]
    }
    ```

    !!! Notes
        - `"tls": true` enables TLS verification.
        - `"hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2376"]` adds TCP listening on port 2376 and retains the Unix socket for local access.
        - Alternatively, use dockerd flags in `/etc/systemd/system/docker.service.d/override.conf` for overrides without editing `daemon.json`.

2. Restart the daemon:

    ```bash
    sudo systemctl restart docker
    ```

3. Verify the daemon is listening:

    ```bash
    sudo netstat -tlnp | grep 2376
    ```

### Watchtower Configuration

=== "Docker Compose"

    ```yaml
    services:
        watchtower:
            image: nickfedor/watchtower
            environment:
                - DOCKER_HOST=tcp://remote-host:2376
                - DOCKER_CERT_PATH=/etc/ssl/docker
                - DOCKER_TLS_VERIFY=1
            volumes:
                - /path/to/certs:/etc/ssl/docker:ro
            restart: unless-stopped
    ```

=== "Docker CLI"

    ```bash
    docker run -d \
      --name watchtower \
      -v /path/to/certs:/etc/ssl/docker:ro \
      nickfedor/watchtower --host tcp://remote-host:2376 --cert-path /etc/ssl/docker --tlsverify
    ```

!!! Note "`remote-host` is used, but can be replaced with `localhost` for local testing."

## Troubleshooting

### Insecure Scheme with TLS Verification

When [TLS verification](#tls_verification) is enabled and the [Docker host URL](#docker_host_url) uses `http://`,  Watchtower logs the following warning:

!!! Warning "TLS verification is enabled but DOCKER_HOST uses insecure scheme 'http://'. Consider using 'https://' or disable TLS verification."

Possible Solutions:

- If using a secure connection, then use `https://`.
- If using `http://`, then disable TLS verification.

### Local Socket with TLS Verification

When [TLS verification](#tls_verification) is enabled and the [Docker host URL](#docker_host_url) is not configured or uses `unix://`,  Watchtower logs the following warning:

!!! Warning "TLS verification is enabled but DOCKER_HOST uses local socket 'unix://'. TLS is not applicable for local sockets; consider disabling TLS verification."

Possible Solutions:

- If the [Docker host URL](#docker_host_url) is not configured, then the default `unix:///var/run/docker.sock` is used.
- If using a local socket (i.e. `unix://`), then disable [TLS verification](#tls_verification).

### Missing TLS Certificates

In order for Watchtower's Docker client to connect to the Docker daemon via TLS, the certificates must be provided to the Watchtower container.

If the certificates are available via the host filesystem of the Watchtower container's Docker host, then this can be accomplished using [bind mounts](https://docs.docker.com/engine/storage/bind-mounts/){target="_blank" rel="noopener noreferrer"}. Other options, such as building a custom Watchtower image with the certificates, are outside the scope of this documentation.

Refer to the documentation for the [TLS Certificate Path](#tls_certificate_path) and the [examples](#examples).

### Other Common Mistakes

- Using `tcp://` without `--tlsverify`: This disables TLS, potentially allowing insecure connections.
- Mismatched certificate paths: Ensure `DOCKER_CERT_PATH` points to the correct directory containing `cert.pem` and `key.pem`.
- Expired or invalid certificates: Check certificate validity and SAN fields matching the host.
- Firewall blocking TLS port: Ensure port 2376 is open for remote connections.

## Certificate Management

This documentation is not intended to be a guide on TLS/mTLS certificate deployment or management.
Manual certificate management might be acceptable for smaller deployments; however, there are solutions for automating certificate management.
If you are exposing your Docker daemon to external network connections, then both proper TLS setup and management is a highly recommended.

Here are just a few available solutions:

- [Step-CA](https://github.com/smallstep/certificates){target="_blank" rel="noopener noreferrer"}: Smallstep's private, self-hostable certificate authority. [[Tutorial](https://smallstep.com/docs/tutorials/docker-tls-certificate-authority/){target="_blank" rel="noopener noreferrer"}]
- [Vault](https://www.vaultproject.io/){target="_blank" rel="noopener noreferrer"}: HashiCorp's secret management tool with PKI secrets engine for certificate generation
- [CFSSL](https://github.com/cloudflare/cfssl){target="_blank" rel="noopener noreferrer"}: Cloudflare's PKI toolkit for certificate management

## Docker Socket Proxies

!!! Warning "You are **highly** encouraged to perform your own due diligence before using any software that interacts directly with the Docker socket."

Docker socket proxies provide a security layer between applications and the Docker daemon by filtering API calls and preventing unrestricted access to the Docker socket.

While this documentation focuses on TLS-based connections, socket proxies represent another approach for securing Docker daemon access in environments where full socket exposure is undesirable.

Refer to the documentation for the [Docker Host URL](#docker_host_url) to connect Watchtower to a Docker socket proxy.

The following projects are examples of Docker socket proxies:

- <https://github.com/Tecnativa/docker-socket-proxy>{target="_blank" rel="noopener noreferrer"}
- <https://github.com/11notes/docker-socket-proxy>{target="_blank" rel="noopener noreferrer"}
- <https://github.com/linuxserver/docker-socket-proxy>{target="_blank" rel="noopener noreferrer"}
- <https://github.com/wollomatic/socket-proxy>{target="_blank" rel="noopener noreferrer"}
