FROM ubuntu:14.04

COPY watchtower /
ENTRYPOINT ["/watchtower"]
