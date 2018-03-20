#!/bin/bash

PROGNAME=$(basename $0)
VERSION_BUILD=$1

function error_exit
{
    echo "${PROGNAME}: ${1:-"Unknown Error"}" 1>&2
    exit 1
}

if [ "$1" = "" ]; then
  error_exit "Please provide version as first argument."
fi

SEMVER=${VERSION_BUILD#*v}
VERSION=`echo $SEMVER | awk '{split($0,a,"."); print a[1]}'`
BUILD=`echo $SEMVER | awk '{split($0,a,"."); print a[2]}'`
PATCH=`echo $SEMVER | awk '{split($0,a,"."); print a[3]}'`

if [ "${VERSION}" = "" ]; then
  echo "Please provide a semantic version."
  exit 1
fi

if [ "${BUILD}" = "" ]; then
  BUILD='0'
fi

if [ "${PATCH}" = "" ]; then
  PATCH='0'
fi

push_docker() {
  echo "  -> push $1 $2"
  docker tag $1 $2 || exit 1
  docker push $2 || exit 1
}

push_all() {
    IMAGE_NAME_VERSION=${1}${VERSION}.${BUILD}.${PATCH}
    echo "Pulling $IMAGE_NAME_VERSION..."
    docker pull ${IMAGE_NAME_VERSION} || exit 1
    echo "Pushing $IMAGE_NAME_VERSION..."
    push_docker ${IMAGE_NAME_VERSION} ${1}${VERSION}.${BUILD}
    push_docker ${IMAGE_NAME_VERSION} ${1}${VERSION}
    push_docker ${IMAGE_NAME_VERSION} ${1}latest
}

IMAGE_NAME=v2tec/watchtower
push_all ${IMAGE_NAME}:
push_all ${IMAGE_NAME}:armhf-
push_all ${IMAGE_NAME}:arm64v8-
