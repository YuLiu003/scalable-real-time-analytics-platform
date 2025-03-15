#!/bin/bash
set -e

CERTS_DIR="./certs"
mkdir -p $CERTS_DIR

# Generate a key pair for the Kafka broker
openssl req -new -newkey rsa:4096 -days 365 -nodes -x509 \
    -subj "/CN=kafka-service/OU=Analytics/O=Company/L=City/ST=State/C=US" \
    -keyout $CERTS_DIR/kafka.key -out $CERTS_DIR/kafka.crt

# Generate a client certificate
openssl req -new -newkey rsa:4096 -days 365 -nodes -x509 \
    -subj "/CN=kafka-client/OU=Analytics/O=Company/L=City/ST=State/C=US" \
    -keyout $CERTS_DIR/client.key -out $CERTS_DIR/client.crt

# Create keystore
openssl pkcs12 -export -in $CERTS_DIR/kafka.crt -inkey $CERTS_DIR/kafka.key \
    -name kafka -out $CERTS_DIR/kafka.p12 -password pass:kafka-password

echo "Certificates generated successfully in $CERTS_DIR"
echo "Add these certificates to Kubernetes secrets before deploying"