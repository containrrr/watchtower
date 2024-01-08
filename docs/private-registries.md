Watchtower supports private Docker image registries. In many cases, accessing a private registry
requires a valid username and password (i.e., _credentials_). In order to operate in such an
environment, watchtower needs to know the credentials to access the registry. 

The credentials can be provided to watchtower in a configuration file called `config.json`.
There are two ways to generate this configuration file:

*   The configuration file can be created manually.
*   Call `docker login <REGISTRY_NAME>` and share the resulting configuration file.

### Create the configuration file manually
Create a new configuration file with the following syntax and a base64 encoded username and
password `auth` string:

```json
{
    "auths": {
        "<REGISTRY_NAME>": {
            "auth": "XXXXXXX"
        }
    }
}
```

`<REGISTRY_NAME>` needs to be replaced by the name of your private registry
(e.g., `my-private-registry.example.org`).

!!! info "Using private images on Docker Hub"
    To access private repositories on Docker Hub,
    `<REGISTRY_NAME>` should be `https://index.docker.io/v1/`.
    In this special case, the registry domain does not have to be specified
    in `docker run` or `docker-compose`. Like Docker, Watchtower will use the
    Docker Hub registry and its credentials when no registry domain is specified.
    
    <sub>Watchtower will recognize credentials with `<REGISTRY_NAME>` `index.docker.io`,
    but the Docker CLI will not.</sub>

!!! important "Using a private registry on a local host"
    To use a private registry hosted locally, make sure to correctly specify the registry host
    in both `config.json` and the `docker run` command or `docker-compose` file.
    Valid hosts are `localhost[:PORT]`, `HOST:PORT`,
    or any multi-part `domain.name` or IP-address with or without a port.
    
    Examples:
    * `localhost` -> `localhost/myimage`
    * `127.0.0.1` -> `127.0.0.1/myimage:mytag`
    * `host.domain` -> `host.domain/myorganization/myimage`
    * `other-lan-host:80` -> `other-lan-host:80/imagename:latest`

The required `auth` string can be generated as follows:

```bash
echo -n 'username:password' | base64
```

!!! info "Username and Password for GCloud"
    For gcloud, we'll use `_json_key` as our username and the content of `gcloudauth.json` as the password.
    ```
    bash echo -n "_json_key:$(cat gcloudauth.json)" | base64 -w0
    ```

When the watchtower Docker container is started, the created configuration file
(`<PATH>/config.json` in this example) needs to be passed to the container:

```bash
docker run [...] -v <PATH>/config.json:/config.json containrrr/watchtower
```

### Share the Docker configuration file

To pull an image from a private registry, `docker login` needs to be called first, to get access
to the registry. The provided credentials are stored in a configuration file called `<PATH_TO_HOME_DIR>/.docker/config.json`.
This configuration file can be directly used by watchtower. In this case, the creation of an
additional configuration file is not necessary.

When the Docker container is started, pass the configuration file to watchtower:

```bash
docker run [...] -v <PATH_TO_HOME_DIR>/.docker/config.json:/config.json containrrr/watchtower
```

When creating the watchtower container via docker-compose, use the following lines:

```yaml
version: "3.4"
services:
  watchtower:
    image: containrrr/watchtower:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - <PATH_TO_HOME_DIR>/.docker/config.json:/config.json
  ...
```

#### Docker Config path
By default, watchtower will look for the `config.json` file in `/`, but this can be changed by setting the `DOCKER_CONFIG` environment variable to the directory path where your config is located. This is useful for setups where the config.json file is changed while the watchtower instance is running, as the changes will not be picked up for a mounted file if the inode changes.
Example usage:

```yaml
version: "3.4"

services: 
  watchtower:
    image: containrrr/watchtower
    environment:
        DOCKER_CONFIG: /config
    volumes:
      - /etc/watchtower/config/:/config/
      - /var/run/docker.sock:/var/run/docker.sock
```

## Credential helpers
Some private Docker registries (the most prominent probably being AWS ECR) use non-standard ways of authentication.
To be able to use this together with watchtower, we need to use a credential helper.

To keep the image size small we've decided to not include any helpers in the watchtower image, instead we'll put the
helper in a separate container and mount it using volumes.

### Example
Example implementation for use with [amazon-ecr-credential-helper](https://github.com/awslabs/amazon-ecr-credential-helper):

Use the dockerfile below to build the [amazon-ecr-credential-helper](https://github.com/awslabs/amazon-ecr-credential-helper),
in a volume that may be mounted onto your watchtower container.

1.  Create the Dockerfile (contents below):
    ```Dockerfile
    FROM golang:1.20
    
    ENV GO111MODULE off
    ENV CGO_ENABLED 0
    ENV REPO github.com/awslabs/amazon-ecr-credential-helper/ecr-login/cli/docker-credential-ecr-login
    
    RUN go get -u $REPO
    
    RUN rm /go/bin/docker-credential-ecr-login
    
    RUN go build \
     -o /go/bin/docker-credential-ecr-login \
     /go/src/$REPO
    
    WORKDIR /go/bin/
    ```

2.  Use the following commands to build the aws-ecr-dock-cred-helper and store it's output in a volume:
    ```bash
    # Create a volume to store the command (once built)
    docker volume create helper 
    
    # Build the container
    docker build -t aws-ecr-dock-cred-helper .
    
    # Build the command and store it in the new volume in the /go/bin directory.
    docker run  -d --rm --name aws-cred-helper \
      --volume helper:/go/bin aws-ecr-dock-cred-helper
    ```

3.  Create a configuration file for docker, and store it in $HOME/.docker/config.json (replace the <AWS_ACCOUNT_ID>
   placeholders with your AWS Account ID and <AWS_ECR_REGION> with your AWS ECR Region):
    ```json
    {
       "credsStore" : "ecr-login",
       "HttpHeaders" : {
         "User-Agent" : "Docker-Client/19.03.1 (XXXXXX)"
       },
       "auths" : {
         "<AWS_ACCOUNT_ID>.dkr.ecr.<AWS_ECR_REGION>.amazonaws.com" : {}
       },
       "credHelpers": {
         "<AWS_ACCOUNT_ID>.dkr.ecr.<AWS_ECR_REGION>.amazonaws.com" : "ecr-login"
       }
    }
    ```

4.  Create a docker-compose file (as an example) to help launch the container:
    ```yaml
    version: "3.4"
    services:
     # Check for new images and restart things if a new image exists
     # for any of our containers.
     watchtower:
       image: containrrr/watchtower:latest
       volumes:
         - /var/run/docker.sock:/var/run/docker.sock
         - .docker/config.json:/config.json
         - helper:/go/bin
       environment:
         - HOME=/
         - PATH=$PATH:/go/bin
         - AWS_REGION=us-west-1
    volumes:
     helper: 
       external: true
    ```

A few additional notes:

1.  With docker-compose the volume (helper, in this case) MUST be set to `external: true`, otherwise docker-compose 
    will preface it with the directory name.

2.  Note that "credsStore" : "ecr-login" is needed - and in theory if you have that you can remove the 
    credHelpers section

3.  I have this running on an EC2 instance that has credentials assigned to it - so no keys are needed; however, 
    you may need to include the `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables as well.

4.  An alternative to adding the various variables is to create a ~/.aws/config and ~/.aws/credentials files and 
    place the settings there, then mount the ~/.aws directory to / in the container.
