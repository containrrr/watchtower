##
# Build
##
FROM golang:alpine as build-env

RUN apk add --no-cache openssh-client git curl

RUN curl https://glide.sh/get | sh

WORKDIR /go/src/github.com/kopfkrieg/watchtower
COPY . .

# RUN set -x && \
#     go get github.com/golang/dep/cmd/dep && \
#     dep ensure -v
RUN glide install

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o watchtower .
# RUN go build -o watchtower .

##
# Watchtower
##
FROM alpine
LABEL "com.centurylinklabs.watchtower"="true"

RUN apk add --no-cache \
    ca-certificates \
    tzdata

COPY --from=build-env /go/src/github.com/kopfkrieg/watchtower/watchtower /
ENTRYPOINT ["/watchtower"]
