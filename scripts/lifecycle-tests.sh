#!/usr/bin/env bash

set -e

IMAGE=server
CONTAINER=server
LINKED_IMAGE=linked
LINKED_CONTAINER=linked
WATCHTOWER_INTERVAL=2

function remove_container {
	docker kill $1 >> /dev/null || true && docker rm -v $1 >> /dev/null || true
}

function cleanup {
  # Do cleanup on exit or error
  echo "Final cleanup"
  sleep 2
  remove_container $CONTAINER
  remove_container $LINKED_CONTAINER
  pkill -9 -f watchtower >> /dev/null || true
}
trap cleanup EXIT

DEFAULT_WATCHTOWER="$(dirname "${BASH_SOURCE[0]}")/../watchtower"
WATCHTOWER=$1
WATCHTOWER=${WATCHTOWER:-$DEFAULT_WATCHTOWER}
echo "watchtower path is $WATCHTOWER"

##################################################################################
##### PREPARATION ################################################################
##################################################################################

#  Create Dockerfile template
DOCKERFILE=$(cat << EOF
FROM node:alpine

LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="cat /opt/test/value.txt"
LABEL com.centurylinklabs.watchtower.lifecycle.post-update="echo image > /opt/test/value.txt"

ENV IMAGE_TIMESTAMP=TIMESTAMP

WORKDIR /opt/test
ENTRYPOINT ["/usr/local/bin/node", "/opt/test/server.js"]

EXPOSE 8888

RUN mkdir -p /opt/test && echo "default" > /opt/test/value.txt
COPY server.js /opt/test/server.js
EOF
)

# Create temporary directory to build docker image
TMP_DIR="/tmp/watchtower-commands-test"
mkdir -p $TMP_DIR

# Create simple http server
cat > $TMP_DIR/server.js << EOF
const http = require("http");
const fs = require("fs");

http.createServer(function(request, response) {
	const fileContent = fs.readFileSync("/opt/test/value.txt");
	response.writeHead(200, {"Content-Type": "text/plain"});
	response.write(fileContent);
	response.end();
}).listen(8888, () => { console.log('server is listening on 8888'); });
EOF

function builddocker {
	TIMESTAMP=$(date +%s)
	echo "Building image $TIMESTAMP"
	echo "${DOCKERFILE/TIMESTAMP/$TIMESTAMP}" > $TMP_DIR/Dockerfile
	docker build $TMP_DIR -t $IMAGE >> /dev/null
}

# Start watchtower
echo "Starting watchtower"
$WATCHTOWER -i $WATCHTOWER_INTERVAL --no-pull --stop-timeout 2s --enable-lifecycle-hooks $CONTAINER $LINKED_CONTAINER &
sleep 3

echo "#################################################################"
echo "##### TEST CASE 1: Execute commands from base image"
echo "#################################################################"

# Build base image
builddocker

# Run container
docker run -d -p 0.0.0.0:8888:8888 --name $CONTAINER $IMAGE:latest >> /dev/null
sleep 1
echo "Container $CONTAINER is running"

# Test default value
RESP=$(curl -s http://localhost:8888)
if [ $RESP != "default" ]; then
	echo "Default value of container response is invalid" 1>&2
	exit 1
fi

# Build updated image to trigger watchtower update
builddocker

WAIT_AMOUNT=$(($WATCHTOWER_INTERVAL * 3))
echo "Wait for $WAIT_AMOUNT seconds"
sleep $WAIT_AMOUNT

# Test value after post-update-command
RESP=$(curl -s http://localhost:8888)
if [[ $RESP != "image" ]]; then
	echo "Value of container response is invalid. Expected: image. Actual: $RESP"
	exit 1
fi

remove_container $CONTAINER

echo "#################################################################"
echo "##### TEST CASE 2: Execute commands from container and base image"
echo "#################################################################"

# Build base image
builddocker

# Run container
docker run -d -p 0.0.0.0:8888:8888 \
	--label=com.centurylinklabs.watchtower.lifecycle.post-update="echo container > /opt/test/value.txt" \
	--name $CONTAINER $IMAGE:latest >> /dev/null
sleep 1
echo "Container $CONTAINER is running"

# Test default value
RESP=$(curl -s http://localhost:8888)
if [ $RESP != "default" ]; then
	echo "Default value of container response is invalid" 1>&2
	exit 1
fi

# Build updated image to trigger watchtower update
builddocker

WAIT_AMOUNT=$(($WATCHTOWER_INTERVAL * 3))
echo "Wait for $WAIT_AMOUNT seconds"
sleep $WAIT_AMOUNT

# Test value after post-update-command
RESP=$(curl -s http://localhost:8888)
if [[ $RESP != "container" ]]; then
	echo "Value of container response is invalid. Expected: container. Actual: $RESP"
	exit 1
fi

remove_container $CONTAINER

echo "#################################################################"
echo "##### TEST CASE 3: Execute commands with a linked container"
echo "#################################################################"

# Tag the current image to keep a version for the linked container
docker tag $IMAGE:latest $LINKED_IMAGE:latest

# Build base image
builddocker

# Run container
docker run -d -p 0.0.0.0:8888:8888 \
	--label=com.centurylinklabs.watchtower.lifecycle.post-update="echo container > /opt/test/value.txt" \
	--name $CONTAINER $IMAGE:latest >> /dev/null
docker run -d -p 0.0.0.0:8989:8888 \
	--label=com.centurylinklabs.watchtower.lifecycle.post-update="echo container > /opt/test/value.txt" \
	--link $CONTAINER \
	--name $LINKED_CONTAINER $LINKED_IMAGE:latest >> /dev/null
sleep 1
echo "Container $CONTAINER and $LINKED_CONTAINER are running"

# Test default value
RESP=$(curl -s http://localhost:8888)
if [ $RESP != "default" ]; then
	echo "Default value of container response is invalid" 1>&2
	exit 1
fi

# Test default value for linked container
RESP=$(curl -s http://localhost:8989)
if [ $RESP != "default" ]; then
	echo "Default value of linked container response is invalid" 1>&2
	exit 1
fi

# Build updated image to trigger watchtower update
builddocker

WAIT_AMOUNT=$(($WATCHTOWER_INTERVAL * 3))
echo "Wait for $WAIT_AMOUNT seconds"
sleep $WAIT_AMOUNT

# Test value after post-update-command
RESP=$(curl -s http://localhost:8888)
if [[ $RESP != "container" ]]; then
	echo "Value of container response is invalid. Expected: container. Actual: $RESP"
	exit 1
fi

# Test that linked container did not execute pre/post-update-command
RESP=$(curl -s http://localhost:8989)
if [[ $RESP != "default" ]]; then
	echo "Value of linked container response is invalid. Expected: default. Actual: $RESP"
	exit 1
fi