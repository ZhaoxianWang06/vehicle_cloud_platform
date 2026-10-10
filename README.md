# 车云平台

车辆通过 T-Box 上报车况，云端保存历史、刷新实时状态，并下发车控指令。这个仓库把这条链路做成可以操作的运维工程：本地用 Docker Compose 跑通，集群用 Kubernetes 部署，云上用 Terraform 建 VPC、EKS、RDS 和 Redis，节点初始化用 Ansible。

实验口令只写在 Compose 和本地清单里。不要把它们带到生产，也不要把 Terraform 生成的数据库口令提交进 Git。

## 先跑通业务闭环

在仓库根目录：

```powershell
docker compose up -d --build
powershell -File scripts/demo.ps1
```

脚本会上报一条车况，确认实时状态来自 Redis，下发 `door_lock`，模拟车端回执，并检查车况指标。成功时输出 `demo ok`。

然后可以看这些地址：

| 地址 | 用途 |
| --- | --- |
| http://127.0.0.1:18080 | Nginx 接入。用 18080，避开本机上常见的 8080 |
| http://127.0.0.1:3000 | Grafana，`admin` / `admin`，仪表盘在 Vehicle Cloud 目录 |
| http://127.0.0.1:9090 | Prometheus |
| http://127.0.0.1:9093 | Alertmanager |
| http://127.0.0.1:18081/metrics | 车况指标，仅本地调试端口 |
| http://127.0.0.1:18082/metrics | 车控指标，仅本地调试端口 |

压测用模拟车队，默认 10 辆车、每辆 200ms 一条，低于 Nginx 的上报限流：

```powershell
go run ./tools/tbox-simulator -url http://127.0.0.1:18080 -vehicles 20 -duration 15s
```

把 `-vehicles` 提到几百、把 `-interval` 降到 10ms，就能在 Nginx 日志里看到 `429`。车况限流是 200 请求/秒，车控是 50 请求/秒，两条通道分开，上报高峰不会先把车控打满。

停掉并清掉实验数据：

```powershell
docker compose down
docker compose down -v
```

`-v` 会删掉 MySQL 和 Redis 的卷。初始化 SQL 只在数据卷为空时执行。

## 请求长什么样

上报：

```http
POST /api/v1/telemetry
{"vin":"LSVAA4182E2123456","speed_kmh":62.5,"battery_pct":78.2,"lng":121.47,"lat":31.23,"odometer_km":15230.4}
```

读取实时状态和历史：

```http
GET /api/v1/vehicles/LSVAA4182E2123456
GET /api/v1/vehicles/LSVAA4182E2123456/history?limit=50
```

下发车控。`idempotency_key` 相同的重试返回原来的指令，不会再写一条：

```http
POST /api/v1/commands
{"vin":"LSVAA4182E2123456","command":"door_lock","idempotency_key":"demo-door-lock-01"}
```

支持的指令：`door_lock`、`door_unlock`、`ac_on`、`ac_off`、`horn`、`find_car`。

车端回执：

```http
POST /api/v1/commands/{id}/ack
{"success":true,"message":"locked"}
```

指令状态：`pending`（已入库，下行队列还没写上）、`dispatched`（已进入 Redis Stream `cmd:stream:{vin}`）、`acked`、`failed`。

## 目录和技能

| 路径 | 对应能力 |
| --- | --- |
| `apps/telemetry-api` | 车况上报。MySQL 存历史，Redis 存最新状态 |
| `apps/control-api` | 车控。MySQL 做幂等，Redis Stream 做下行 |
| `tools/tbox-simulator` | 车队并发上报 |
| `deploy/docker` | Nginx 与 MySQL 初始化 |
| `docker-compose.yml` | 本地整栈 |
| `deploy/k8s` | Deployment、HPA、PDB、Ingress、NetworkPolicy |
| `deploy/observability` | Prometheus、Alertmanager、Grafana、Loki |
| `infra/terraform` | VPC、EKS、RDS MySQL、ElastiCache Redis |
| `ansible` | 节点基线、containerd、kubeadm |
| `scripts` | 业务演示、备份、Terraform workspace、回滚 |
| `.gitlab-ci.yml` | 测试、构建镜像、手动部署 |

更细的说明在 `docs/architecture.md`、`docs/local.md`、`docs/cluster.md`。

## 测试

```powershell
go test ./pkg/platform/ ./apps/telemetry-api/... ./apps/control-api/... ./tools/tbox-simulator/
```

单元测试不连接数据库。车况接口在缓存失败时仍返回成功，车控接口在队列失败时返回 `202` 且 `queued=false`，这两条都有测试。如果模块下载超时，先执行 `$env:GOPROXY = "https://goproxy.cn,direct"`。
