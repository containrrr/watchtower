#!/usr/bin/env bash
# This file is meant to be sourced into other scripts and contain some utility functions for docker e2e testing


CONTAINER_PREFIX=${CONTAINER_PREFIX:-du}

function get-port() {
    Container=$1
    Port=$2

    if [ -z "$Container" ];  then
      echo "CONTAINER missing" 1>&2
      return 1
    fi

    if [ -z "$Port" ];  then
      echo "PORT missing" 1>&2
      return 1
    fi

    Query=".[].NetworkSettings.Ports[\"$Port/tcp\"] | .[0].HostPort"
    docker container inspect "$Container" | jq -r "$Query"
}

function start-registry() {
  local Name="$CONTAINER_PREFIX-registry"
  echo -en "Starting \e[94m$Name\e[0m container... "
  local Port="${1:-5000}"
  docker run -d -p 5000:"$Port" --restart=unless-stopped --name "$Name" registry:2
}

function stop-registry() {
  try-remove-container "$CONTAINER_PREFIX-registry"
}

function registry-host() {
  echo "localhost:$(get-port "$CONTAINER_PREFIX"-registry 5000)"
}

function try-remove-container() {
  echo -en "Looking for container \e[95m$1\e[0m... "
  local Found
  Found=$(container-id "$1")
  if [ -n "$Found" ]; then
    echo "$Found"
    echo -n "  Stopping... "
    docker stop "$1"
    echo -n "  Removing... "
    docker rm "$1"
  else
    echo "Not found"
  fi
}

function create-dummy-image() {
    if [ -z "$1" ];  then
      echo "TAG missing"
      return 1
    fi
    local Tag="$1"
    local Repo
    Repo="$(registry-host)"
    local Revision=${2:-$(("$(date +%s)" - "$(date --date='2021-10-21' +%s)"))}

    echo -e "Creating new image \e[95m$Tag\e[0m revision: \e[94m$Revision\e[0m"

    local BuildDir="/tmp/docker-dummy-$Tag-$Revision"

    mkdir -p "$BuildDir"

    cat > "$BuildDir/Dockerfile" << END
FROM alpine

RUN echo "Tag: $Tag"
RUN echo "Revision: $Revision"
ENTRYPOINT ["nc", "-lk", "-v", "-l", "-p", "9090", "-e", "echo", "-e", "HTTP/1.1 200 OK\n\n$Tag $Revision"]
END

   docker build -t "$Repo/$Tag:latest" -t "$Repo/$Tag:r$Revision" "$BuildDir"

   echo -e "Pushing images...\e[93m"
   docker push -q "$Repo/$Tag:latest"
   docker push -q "$Repo/$Tag:r$Revision"
   echo -en "\e[0m"

   rm -r "$BuildDir"
}

function query-rev() {
  local Name=$1
  if [ -z "$Name" ];  then
    echo "NAME missing"
    return 1
  fi
  curl -s "localhost:$(get-port "$Name" 9090)"
}

function latest-image-rev() {
  local Tag=$1
  if [ -z "$Tag" ];  then
    echo "TAG missing"
    return 1
  fi
  local ID
  ID=$(docker image ls "$(registry-host)"/"$Tag":latest -q)
  docker image inspect "$ID" | jq -r '.[].RepoTags | .[]' | grep  -v latest
}

function container-id() {
  local Name=$1
  if [ -z "$Name" ];  then
    echo "NAME missing"
    return 1
  fi
  docker container ls -f name="$Name" -q
}

function container-started() {
  local Name=$1
  if [ -z "$Name" ];  then
    echo "NAME missing"
    return 1
  fi
  docker container inspect "$Name" | jq -r .[].State.StartedAt
}