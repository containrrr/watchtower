# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
FROM golang:1.27.3-alpine@sha256:d530cf60723abf1ffae0cd16bb546b379ec0550d2f3815e5b16ed69dbc00fba4
ARG GOOS=linux

# gcc and musl-dev provide the cgo toolchain that the race detector requires.
RUN apk add --no-cache \
    gcc \
    git \
    musl-dev \
    nano \
    openssh

ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000
ENV GO111MODULE=on


# Setup user
RUN adduser $USERNAME -s /bin/sh -D -u $USER_UID $USER_GID && \
    mkdir -p /etc/sudoers.d && \
    echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME && \
    chmod 0440 /etc/sudoers.d/$USERNAME

# Install golangci-lint
RUN wget -O- https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin/

USER $USERNAME

RUN go install -v github.com/go-task/task/v3/cmd/task@v3.54.0
RUN go install -v golang.org/x/tools/gopls@latest
RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
RUN go install -v honnef.co/go/tools/cmd/staticcheck@latest
