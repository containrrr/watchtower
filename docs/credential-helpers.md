Some private docker registries (the most prominent probably being AWS ECR) use non-standard ways of authentication.
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

*Note:* `osxkeychain` can be changed to your prefered credentials helper.
