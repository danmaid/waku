#!/usr/bin/env bash
set -e

TLS_DIR=/etc/httpd/tls
CRT=$TLS_DIR/localhost.crt
KEY=$TLS_DIR/localhost.key

if [ ! -f "$CRT" ] || [ ! -s "$CRT" ]; then
  echo "Generating self-signed TLS certificate for dev…"

  mkdir -p "$TLS_DIR"

  openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -days 365 \
    -keyout "$KEY" \
    -out "$CRT" \
    -subj "/CN=localhost"

  chmod 600 "$KEY"
fi
