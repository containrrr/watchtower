Watchtower supports private Docker image registries. In many cases, accessing a private registry
requires a valid username and password (i.e., _credentials_). In order to operate in such an
environment, watchtower needs to know the credentials to access the registry. 

The credentials can be provided to watchtower in a configuration file called `config.json`.
There are two ways to generate this configuration file:

* The configuration file can be created manually.
* Call `docker login <REGISTRY_NAME>` and share the resulting configuration file.

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
(e.g., `my-private-registry.example.org`)

The required `auth` string can be generated as follows:
```bash
echo -n 'username:password' | base64
```

> ### ℹ️ Username and Password for GCloud
>
> For gcloud, we'll use `__json_key` as our username and the content
> of `gcloudauth.json` as the password.

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
version: "3"
[...]
watchtower:
  image: index.docker.io/containrrr/watchtower:latest
  volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - <PATH_TO_HOME_DIR>/.docker/config.json:/config.json
[...]
```

## Credential helpers
Some private Docker registries (the most prominent probably being AWS ECR) use non-standard ways of authentication.
To be able to use this together with watchtower, we need to use a credential helper.

To keep the image size small we've decided to not include any helpers in the watchtower image, instead we'll put the
helper in a separate container and mount it using volumes.

### Example
Example implementation for use with [amazon-ecr-credential-helper](https://github.com/awslabs/amazon-ecr-credential-helper):

```Dockerfile
FROM golang:latest

ENV CGO_ENABLED 0
ENV REPO github.com/awslabs/amazon-ecr-credential-helper/ecr-login/cli/docker-credential-ecr-login

RUN go get -u $REPO

RUN rm /go/bin/docker-credential-ecr-login

RUN go build \
  -o /go/bin/docker-credential-ecr-login \
  /go/src/$REPO

WORKDIR /go/bin/
```

and the docker-compose definition:
```yaml
version: "3"

services:
  watchtower:
    image: index.docker.io/containrrr/watchtower:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - <PATH_TO_HOME_DIR>/.docker/config.json:/config.json
      - helper:/go/bin
    environment:
      - HOME=/
      - PATH=$PATH:/go/bin
      - AWS_REGION=<AWS_REGION>
      - AWS_ACCESS_KEY_ID=<AWS_ACCESS_KEY>
      - AWS_SECRET_ACCESS_KEY=<AWS_SECRET_ACCESS_KEY>
volumes:
  helper: {}
```

and for `<PATH_TO_HOME_DIR>/.docker/config.json`:
```json
  {
    "HttpHeaders" : {
      "User-Agent" : "Docker-Client/19.03.1 (XXXXXX)"
    },
    "credsStore" : "osxkeychain",
    "auths" : {
      "xyzxyzxyz.dkr.ecr.eu-north-1.amazonaws.com" : {},
      "https://index.docker.io/v1/": {}
    },
    "credHelpers": {
      "xyzxyzxyz.dkr.ecr.eu-north-1.amazonaws.com" : "ecr-login",
      "index.docker.io": "osxkeychain"
    }
  }
```

*Note:* `osxkeychain` can be changed to your preferred credentials helper.
