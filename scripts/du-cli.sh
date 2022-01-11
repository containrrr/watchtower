#!/usr/bin/env bash

SCRIPT_ROOT=$(dirname "$(readlink -m "$(type -p "$0")")")
source "$SCRIPT_ROOT/docker-util.sh"

case $1 in
  registry | reg | r)
    case $2 in
      start)
        start-registry
        ;;
      stop)
        stop-registry
        ;;
      host)
        registry-host
        ;;
      *)
        echo "Unknown keyword \"$2\""
        ;;
    esac
    ;;
  image | img | i)
    case $2 in
      rev)
        create-dummy-image "${@:3:2}"
        ;;
      latest)
        latest-image-rev "$3"
        ;;
      *)
        echo "Unknown keyword \"$2\""
        ;;
    esac
    ;;
  container | cnt | c)
    case $2 in
      query)
        query-rev "$3"
        ;;
      rm)
        try-remove-container "$3"
        ;;
      id)
        container-id "$3"
        ;;
      started)
        container-started "$3"
        ;;
      *)
        echo "Unknown keyword \"$2\""
        ;;
    esac
    ;;
  *)
    echo "Unknown keyword \"$1\""
    ;;
esac