#!/usr/bin/env bash

# Simulates a container that will always be updated, checking whether it shuts down it's dependencies correctly.

docker rm -f parent || true
docker rm -f depending || true

CHANGE=redis:latest
KEEP=tutum/hello-world

docker tag tutum/hello-world:latest redis:latest

docker run -d --name parent $CHANGE
docker run -d --name depending --link parent $KEEP

go run . --run-once --debug $@
