#!/usr/bin/env sh
set -eu

URL="${URL:-http://localhost:8080/top?limit=10}"
DURATION="${DURATION:-30s}"
CONNECTIONS="${CONNECTIONS:-100}"

if command -v hey >/dev/null 2>&1; then
  hey -z "$DURATION" -c "$CONNECTIONS" "$URL"
  exit 0
fi

if command -v wrk >/dev/null 2>&1; then
  wrk -t4 -c"$CONNECTIONS" -d"$DURATION" "$URL"
  exit 0
fi

echo "hey/wrk not found. Running simple curl smoke load."
i=0
while [ "$i" -lt 1000 ]; do
  curl -fsS "$URL" >/dev/null
  i=$((i + 1))
done

echo "done"
