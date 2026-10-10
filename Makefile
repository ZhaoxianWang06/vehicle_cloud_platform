.PHONY: test up down logs demo

test:
	go test ./pkg/platform/ ./apps/telemetry-api/... ./apps/control-api/... ./tools/tbox-simulator/

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f telemetry-api control-api nginx

demo:
	powershell -NoProfile -File scripts/demo.ps1
