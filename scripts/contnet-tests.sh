#!/usr/bin/env bash

set -e

function exit_env_err() {
  >&2 echo "Required environment variable not set: $1"
  exit 1
}

if [ -z "$VPN_SERVICE_PROVIDER" ]; then exit_env_err "VPN_SERVICE_PROVIDER"; fi
if [ -z "$OPENVPN_USER" ]; then exit_env_err "OPENVPN_USER"; fi
if [ -z "$OPENVPN_PASSWORD" ]; then exit_env_err "OPENVPN_PASSWORD"; fi
# if [ -z "$SERVER_COUNTRIES" ]; then exit_env_err "SERVER_COUNTRIES"; fi


export SERVER_COUNTRIES=${SERVER_COUNTRIES:"Sweden"}
REPO_ROOT="$(git rev-parse --show-toplevel)"
COMPOSE_FILE="$REPO_ROOT/dockerfiles/container-networking/docker-compose.yml"
DEFAULT_WATCHTOWER="$REPO_ROOT/watchtower"
WATCHTOWER="$*"
WATCHTOWER=${WATCHTOWER:-$DEFAULT_WATCHTOWER}
echo "repo root path is $REPO_ROOT"
echo "watchtower path is $WATCHTOWER"
echo "compose file path is $COMPOSE_FILE"

echo; echo "=== Forcing network container producer update..."

echo "Pull previous version of gluetun..."
docker pull qmcgaw/gluetun:v3.34.3
echo "Fake new version of gluetun by retagging v3.34.4 as v3.35.0..."
docker tag qmcgaw/gluetun:v3.34.3  qmcgaw/gluetun:v3.35.0

echo; echo "=== Creating containers..."

docker compose -p "wt-contnet" -f "$COMPOSE_FILE" up -d

echo; echo "=== Running watchtower"
$WATCHTOWER --run-once

echo; echo "=== Removing containers..."

docker compose -p "wt-contnet" -f "$COMPOSE_FILE" down
