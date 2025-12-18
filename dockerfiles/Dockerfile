FROM --platform=$BUILDPLATFORM alpine:3.19.0 as alpine

RUN apk add --no-cache \
    ca-certificates \
    tzdata

FROM scratch
LABEL "com.centurylinklabs.watchtower"="true"
LABEL "org.opencontainers.image.url"="https://containrrr.dev/watchtower/" \
      "org.opencontainers.image.documentation"="https://containrrr.dev/watchtower/" \
      "org.opencontainers.image.source"="https://github.com/containrrr/watchtower" \
      "org.opencontainers.image.licenses"="Apache-2.0" \
# unsure about capitalization
      "org.opencontainers.image.title"="watchtower" \
      "org.opencontainers.image.description"="A process for automating Docker container base image updates." \
# The version might be forgotten. The version could be extraced into a `ARG` before the initial `FROM` or removed.
      "org.opencontainers.image.base.name"="alpine:3.19.0"

COPY --from=alpine \
    /etc/ssl/certs/ca-certificates.crt \
    /etc/ssl/certs/ca-certificates.crt
COPY --from=alpine \
    /usr/share/zoneinfo \
    /usr/share/zoneinfo

EXPOSE 8080

COPY watchtower /

HEALTHCHECK CMD [ "/watchtower", "--health-check"]

ENTRYPOINT ["/watchtower"]
