#!/usr/bin/env sh
set -eu

QUERY="${1:-кроссовки}"
USER_ID="${2:-user-1}"

docker compose run --rm demo-producer /app/app -once -query "$QUERY" -user "$USER_ID"
