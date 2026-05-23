#!/usr/bin/env sh
set -eu

curl -fsS http://localhost:8080/healthz >/dev/null
./scripts/publish-event.sh кроссовки user-smoke >/dev/null
sleep 1
curl -fsS "http://localhost:8080/top?limit=5" | grep -q "кроссовки"
curl -fsS http://localhost:8080/metrics | grep -q "search_events_received_total"

echo "smoke ok"
