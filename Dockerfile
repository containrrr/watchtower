#
# Alpine image to get some needed data
#
FROM alpine:latest as alpine
RUN apk add --no-cache \
    ca-certificates \
    tzdata

#
# Image
#
FROM scratch
LABEL "com.centurylinklabs.watchtower"="true"

# copy files from other containers
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine /usr/share/zoneinfo /usr/share/zoneinfo

COPY watchtower /
ENTRYPOINT ["/watchtower"]