#
# Builder
#

FROM golang:alpine as builder

RUN apk add --no-cache \
    alpine-sdk \
    ca-certificates \
    git \
    tzdata

WORKDIR /usr/local/src
COPY . .

RUN \
  GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' . && \
  GO111MODULE=on go test ./... -v


#
# watchtower
#

FROM scratch

LABEL "com.centurylinklabs.watchtower"="true"

# copy files from other container
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /usr/local/src/watchtower /watchtower

ENTRYPOINT ["/watchtower"]
