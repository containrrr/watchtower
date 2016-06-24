FROM centurylink/ca-certs

COPY watchtower /
ENTRYPOINT ["/watchtower"]
