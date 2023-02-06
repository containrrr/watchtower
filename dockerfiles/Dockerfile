FROM --platform=$BUILDPLATFORM alpine:3.17.1 as alpine

RUN apk add --no-cache \
    ca-certificates \
    tzdata

FROM scratch
LABEL "com.centurylinklabs.watchtower"="true"

COPY --from=alpine \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine \
    /usr/share/zoneinfo \
    /usr/share/zoneinfo

EXPOSE 8080

COPY watchtower /
ENTRYPOINT ["/watchtower"]
