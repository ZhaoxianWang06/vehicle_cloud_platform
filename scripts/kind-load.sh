#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"
docker build -f apps/telemetry-api/Dockerfile -t vehicle-cloud/telemetry-api:local .
docker build -f apps/control-api/Dockerfile -t vehicle-cloud/control-api:local .
kind load docker-image vehicle-cloud/telemetry-api:local --name vehicle-cloud
kind load docker-image vehicle-cloud/control-api:local --name vehicle-cloud
kubectl apply -k deploy/k8s/overlays/local
kubectl -n vehicle-cloud rollout status deployment/telemetry-api --timeout=180s
kubectl -n vehicle-cloud rollout status deployment/control-api --timeout=180s
