#!/bin/bash
set -e

echo "Generating self-signed TLS certificates for development..."

mkdir -p certs

openssl req -x509 -newkey rsa:4096 -sha256 -days 365 \
    -nodes \
    -keyout certs/key.pem \
    -out certs/cert.pem \
    -subj "/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

chmod 600 certs/key.pem
chmod 644 certs/cert.pem

echo ""
echo "✓ Generated development certificates:"
echo "  certs/cert.pem"
echo "  certs/key.pem"
echo ""
echo "Enable TLS in development:"
echo "  export PAYMENTS_SERVER_TLS_ENABLED=true"
echo "  export PAYMENTS_SERVER_TLS_CERT_FILE=./certs/cert.pem"
echo "  export PAYMENTS_SERVER_TLS_KEY_FILE=./certs/key.pem"
