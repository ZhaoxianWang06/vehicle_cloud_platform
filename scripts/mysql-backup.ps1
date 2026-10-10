$ErrorActionPreference = "Stop"
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$out = Join-Path (Get-Location) "vehicle-cloud-$stamp.sql"
docker compose exec -T mysql sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction --databases vehicle_cloud' | Out-File -FilePath $out -Encoding utf8
Write-Host "backup written to $out"
