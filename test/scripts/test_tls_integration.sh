#!/bin/bash

set -e

CERT_DIR="/tmp/watchtower-testing"
IMAGE_TAG="watchtower:test"
DIND_NAME="watchtower-dind"
WATCHTOWER_NAME="watchtower-test"

# Cleanup function
cleanup() {
	echo "Cleaning up containers..."
	docker stop "$DIND_NAME" 2>/dev/null || true
	docker rm "$DIND_NAME" 2>/dev/null || true
	docker rm "$WATCHTOWER_NAME" 2>/dev/null || true
}

# Trap to cleanup on exit
trap cleanup EXIT

# Function to generate self-signed certificates
generate_certs() {
	echo "Generating self-signed certificates in $CERT_DIR..."
	mkdir -p "$CERT_DIR"

	# Generate CA
	openssl genrsa -out "$CERT_DIR/ca-key.pem" 4096
	openssl req -new -x509 -days 365 -key "$CERT_DIR/ca-key.pem" -sha256 -out "$CERT_DIR/ca.pem" -subj "/C=US/ST=State/L=City/O=Org/CN=ca"

	# Generate server key
	openssl genrsa -out "$CERT_DIR/server-key.pem" 4096
	openssl req -subj "/CN=localhost" -new -key "$CERT_DIR/server-key.pem" -out "$CERT_DIR/server.csr"
	echo "subjectAltName = IP:127.0.0.1,DNS:localhost" >"$CERT_DIR/extfile.cnf"
	openssl x509 -req -days 365 -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial -out "$CERT_DIR/server-cert.pem" -extfile "$CERT_DIR/extfile.cnf"

	# Generate client key
	openssl genrsa -out "$CERT_DIR/client-key.pem" 4096
	openssl req -subj '/CN=client' -new -key "$CERT_DIR/client-key.pem" -out "$CERT_DIR/client.csr"
	echo "extendedKeyUsage = clientAuth" >"$CERT_DIR/extfile-client.cnf"
	openssl x509 -req -days 365 -in "$CERT_DIR/client.csr" -CA "$CERT_DIR/ca.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial -out "$CERT_DIR/client-cert.pem" -extfile "$CERT_DIR/extfile-client.cnf"

	rm "$CERT_DIR/server.csr" "$CERT_DIR/client.csr" "$CERT_DIR/extfile.cnf" "$CERT_DIR/extfile-client.cnf"

	# Copy client certs to expected names
	cp "$CERT_DIR/client-cert.pem" "$CERT_DIR/cert.pem"
	cp "$CERT_DIR/client-key.pem" "$CERT_DIR/key.pem"
}

# Cleanup any existing containers
cleanup

# Check if certs exist
if [ ! -f "$CERT_DIR/ca.pem" ]; then
	generate_certs
else
	echo "Certificates already exist in $CERT_DIR, skipping generation."
fi

# Start DinD with TLS
echo "Starting Docker-in-Docker with TLS..."
docker run --privileged --name "$DIND_NAME" -d -p 2376:2376 \
	-v "$CERT_DIR:/certs" \
	docker:dind \
	--tlsverify \
	--tlscacert=/certs/ca.pem \
	--tlscert=/certs/server-cert.pem \
	--tlskey=/certs/server-key.pem

echo "DinD logs:"
docker logs "$DIND_NAME"

# Wait for DinD to be ready
echo "Waiting for DinD to be ready..."
for i in {1..30}; do
	if docker --tlsverify \
		--tlscacert="$CERT_DIR/ca.pem" \
		--tlscert="$CERT_DIR/cert.pem" \
		--tlskey="$CERT_DIR/key.pem" \
		-H "tcp://localhost:2376" info >/dev/null 2>&1; then
		echo "DinD is ready after ${i}s"
		break
	fi
	sleep 1
done

# Build Watchtower image
echo "Building Watchtower image..."
docker build -f build/docker/Dockerfile.self-local -t "$IMAGE_TAG" .

# Run Watchtower and capture logs
echo "Running Watchtower..."
LOGS=$(docker run --rm --name "$WATCHTOWER_NAME" --network host \
	-e DOCKER_HOST=tcp://localhost:2376 \
	-e DOCKER_CERT_PATH="$CERT_DIR" \
	-v "$CERT_DIR:$CERT_DIR" \
	-v /var/run/docker.sock:/var/run/docker.sock \
	"$IMAGE_TAG" --tlsverify --debug --run-once 2>&1)

echo "Watchtower logs:"
echo "$LOGS"

# Check logs
if echo "$LOGS" | grep -q "Client sent an HTTP request to an HTTPS server"; then
	echo "FAIL: Found HTTP to HTTPS error in logs."
	RESULT="FAIL"
elif echo "$LOGS" | grep -q "Initialized Docker client"; then
	echo "PASS: Successful connection detected."
	RESULT="PASS"
else
	echo "FAIL: No successful connection message found."
	RESULT="FAIL"
fi

echo "Test result: $RESULT"
if [ "$RESULT" = "FAIL" ]; then
	exit 1
fi
