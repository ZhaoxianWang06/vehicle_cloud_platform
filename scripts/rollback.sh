#!/usr/bin/env bash
set -euo pipefail
kubectl -n vehicle-cloud rollout undo deployment/telemetry-api
kubectl -n vehicle-cloud rollout undo deployment/control-api
kubectl -n vehicle-cloud rollout status deployment/telemetry-api --timeout=180s
kubectl -n vehicle-cloud rollout status deployment/control-api --timeout=180s
