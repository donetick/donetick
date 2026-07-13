#!/bin/sh
set -eu
set -o pipefail

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$(id -u)" = "0" ]; then
    addgroup -g "$PGID" -S donetick 2>/dev/null || true
    adduser -u "$PUID" -G donetick -S -H -D donetick 2>/dev/null || true

    mkdir -p /donetick-data /config
    chown -R "$PUID:$PGID" /donetick-data /config

    exec su-exec donetick "$@"
else
    exec "$@"
fi
