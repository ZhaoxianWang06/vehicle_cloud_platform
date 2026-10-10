# 本地运行

需要 Docker Desktop 和 Go 1.22。Go 只在本机跑测试和模拟器时需要，镜像构建在容器里完成。构建镜像时如果模块下载超时，先设置 `$env:GOPROXY = "https://goproxy.cn,direct"` 和 `$env:GOSUMDB = "sum.golang.google.cn"`。如果 Docker Hub 的 `golang` / `alpine` 拉不下来，再设置 `$env:GO_BASE` 和 `$env:RUNTIME_BASE` 指向可用的镜像仓库。Compose 会把这些变量传进构建。

```powershell
docker compose up -d --build
powershell -File scripts/demo.ps1
```

Linux 或 Git Bash 用 `scripts/demo.sh`。

## 实验口令

| 对象 | 用户 | 口令 |
| --- | --- | --- |
| MySQL root | root | vehicle-root-pass |
| 业务库 vehicle_cloud | vehicle | vehicle-lab-pass |
| mysqld exporter | exporter | exporter-lab-pass |
| Grafana | admin | admin |
| Redis | 无 | 无 |

备份：

```powershell
powershell -File scripts/mysql-backup.ps1
```

## 日志和告警

应用和 Nginx 打 JSON 到标准输出。Promtail 通过 Docker socket 采集，Grafana 里选 Loki 数据源，或直接看仪表盘底部的日志面板。

```logql
{container=~"telemetry-api|control-api|nginx"}
```

Prometheus 规则在 `deploy/observability/prometheus/alerts.yml`。车况入库失败率、上报 P99、车况/车控抓取失败、MySQL、Redis、Nginx 各有一条。Alertmanager 目前只接收不外发，接通知时改 `deploy/observability/alertmanager/alertmanager.yml`。

改完 Prometheus 配置后：

```powershell
curl.exe -X POST http://127.0.0.1:9090/-/reload
```

## 排障

`docker compose ps` 里某个服务一直 restarting 时，先看 `docker compose logs <服务名>`。

MySQL 初始化只在卷是空的时候跑。改过 `deploy/mysql/init` 之后要 `docker compose down -v` 再启动。如果也在用本地集群，把同一份 SQL 同步到 `deploy/k8s/overlays/local/001_schema.sql`。

Windows 上 Promtail 如果因为 Docker socket 起不来，业务接口和 Prometheus 不受影响，只是 Loki 没有容器日志。

车况 `readyz` 在 MySQL 不可用时失败，Redis 不可用时仍然就绪。车控两个都要通。这个差别是故意的，说明见 `docs/architecture.md`。
