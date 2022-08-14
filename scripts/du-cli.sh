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
        echo "Unknown registry action \"$2\""
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
      rm)
        remove-repo-images "$3"
        ;;
      *)
        echo "Unknown image action \"$2\""
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
      create)
        create-container "${@:3:2}"
        ;;
      create-stale)
        if [ -z "$3" ]; then
          echo "NAME missing"
          exit 1
        fi
        if ! registry-exists; then
          echo "Registry container missing! Creating..."
          start-registry || exit 1
        fi
        image_name="images/$3"
        container_name=$3
        $0 image rev "$image_name" || exit 1
        $0 container create "$container_name" "$image_name" || exit 1
        $0 image rev "$image_name" || exit 1
        ;;
      *)
        echo "Unknown container action \"$2\""
        ;;
    esac
    ;;
  *)
    echo "Unknown keyword \"$1\""
    ;;
esac