#!/usr/bin/env bash
set -euo pipefail
stamp=$(date +%Y%m%d-%H%M%S)
out="vehicle-cloud-${stamp}.sql"
docker compose exec -T mysql sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --databases vehicle_cloud' > "$out"
echo "backup written to $out"
