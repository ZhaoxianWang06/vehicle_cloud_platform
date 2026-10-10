$ErrorActionPreference = "Stop"
$base = "http://127.0.0.1:18080"
$vin = "LSVAA4182E2123456"

for ($i = 0; $i -lt 40; $i++) {
    try {
        $health = Invoke-WebRequest -Uri "$base/healthz" -UseBasicParsing -TimeoutSec 2
        if ($health.StatusCode -eq 200) { break }
    } catch {
        Start-Sleep -Seconds 3
    }
    if ($i -eq 39) { throw "Nginx 未就绪。先在仓库根目录执行 docker compose up -d --build" }
}

$reportedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$telemetry = @{
    vin          = $vin
    speed_kmh    = 62.5
    battery_pct  = 78.2
    lng          = 121.4737
    lat          = 31.2304
    odometer_km  = 15230.4
    reported_at  = $reportedAt
} | ConvertTo-Json
$ingest = Invoke-RestMethod -Method Post -Uri "$base/api/v1/telemetry" -ContentType "application/json" -Body $telemetry
if ($ingest.code -ne 0) { throw "车况上报失败" }

$latest = Invoke-RestMethod -Uri "$base/api/v1/vehicles/$vin"
if ($latest.data.source -ne "redis") { throw "实时状态没有命中 Redis，实际来源 $($latest.data.source)" }

$command = @{
    vin              = $vin
    command          = "door_lock"
    idempotency_key  = "demo-door-lock-01"
} | ConvertTo-Json
$created = Invoke-RestMethod -Method Post -Uri "$base/api/v1/commands" -ContentType "application/json" -Body $command
$id = $created.data.id
if (-not $id) { throw "车控没有返回指令 id" }

$ack = @{ success = $true; message = "locked" } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$base/api/v1/commands/$id/ack" -ContentType "application/json" -Body $ack | Out-Null
$got = Invoke-RestMethod -Uri "$base/api/v1/commands/$id"
if ($got.data.status -ne "acked") { throw "回执后状态是 $($got.data.status)" }

$metrics = (Invoke-WebRequest -Uri "http://127.0.0.1:18081/metrics" -UseBasicParsing).Content
if ($metrics -notmatch "vehicle_telemetry_ingest_total") { throw "车况指标不存在" }

Write-Host "demo ok vin=$vin command=$id source=$($latest.data.source)"
