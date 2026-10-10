#!/usr/bin/env bash
set -euo pipefail
base="${BASE_URL:-http://127.0.0.1:18080}"
vin="LSVAA4182E2123456"

for _ in $(seq 1 40); do
  if curl -fsS "$base/healthz" >/dev/null; then
    break
  fi
  sleep 3
done
curl -fsS "$base/healthz" >/dev/null

curl -fsS -X POST "$base/api/v1/telemetry" \
  -H 'content-type: application/json' \
  -d "{\"vin\":\"$vin\",\"speed_kmh\":62.5,\"battery_pct\":78.2,\"lng\":121.4737,\"lat\":31.2304,\"odometer_km\":15230.4}" >/dev/null

latest=$(curl -fsS "$base/api/v1/vehicles/$vin")
printf '%s' "$latest" | grep -q '"source":"redis"'

created=$(curl -fsS -X POST "$base/api/v1/commands" \
  -H 'content-type: application/json' \
  -d "{\"vin\":\"$vin\",\"command\":\"door_lock\",\"idempotency_key\":\"demo-door-lock-01\"}")
id=$(printf '%s' "$created" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
test -n "$id"

curl -fsS -X POST "$base/api/v1/commands/$id/ack" \
  -H 'content-type: application/json' \
  -d '{"success":true,"message":"locked"}' >/dev/null
got=$(curl -fsS "$base/api/v1/commands/$id")
printf '%s' "$got" | grep -q '"status":"acked"'

curl -fsS http://127.0.0.1:18081/metrics | grep -q vehicle_telemetry_ingest_total
echo "demo ok vin=$vin command=$id"
