# 集群、云上和流水线

本地 Compose 用容器里的 MySQL 和 Redis。上集群之后有两条路：kind 或自建 kubeadm 使用 `deploy/k8s/overlays/local`；EKS 使用托管的 RDS 和 ElastiCache，清单是 `deploy/k8s/overlays/prod`。

## kind

```bash
kind create cluster --name vehicle-cloud
bash scripts/kind-load.sh
```

脚本构建 `vehicle-cloud/telemetry-api:local` 和 `vehicle-cloud/control-api:local`，装进 kind，再套用 local overlay。Overlay 里带单副本 MySQL 和 Redis，口令和 Compose 相同。

没有 metrics-server 时 HPA 会显示 unknown，Pod 仍然按副本数运行。Ingress 需要 ingress-nginx，否则 Ingress 对象不会有地址，可以用 `kubectl -n vehicle-cloud port-forward svc/telemetry-api 18081:80` 看接口。

ServiceMonitor 在 `deploy/k8s/monitoring/servicemonitors.yaml`，没有 Prometheus Operator 的 CRD 时不要套用它。安装 kube-prometheus-stack 时可以带上 `deploy/observability/kube-prometheus-values.yaml`，里面把 ServiceMonitor 选择器放开。

回滚：

```bash
bash scripts/rollback.sh
```

## Ansible 自建集群

剧本面向 Ubuntu，做三件事：关闭 swap、加载 `br_netfilter`、写 sysctl；安装 containerd 并打开 systemd cgroup；安装并锁定 kubeadm 1.31。Kubernetes 1.24 之后节点运行时是 containerd，Docker 只负责构建镜像。

先改 `ansible/inventory/lab.ini` 里的地址，再在能 SSH 到这些机器的位置执行：

```bash
cd ansible
ansible-playbook playbooks/kubeadm-cluster.yml
ansible-playbook playbooks/addons.yml
```

`kubeadm-cluster.yml` 只在第一台 master 上 `kubeadm init`，worker 使用它打印的 join 命令。Pod 网段是 `10.244.0.0/16`，CNI 是 Flannel。这是单控制面实验，不是堆叠 etcd 的高可用控制面。

`addons.yml` 安装 baremetal 版 ingress-nginx（NodePort），再套用 local overlay。云上负载均衡要用 ingress-nginx 的 cloud 清单，不要沿用这个 baremetal 地址。

版本钉在 `ansible/group_vars/all.yml`。apply 前确认 pkgs.k8s.io 仍提供这个小版本。

已经有 kubeconfig 的机器用：

```bash
ansible-playbook ansible/playbooks/addons-localhost.yml
```

## Terraform

在 `infra/terraform`。一个根模块，三个变量文件：

| 文件 | 区域 | 形态 |
| --- | --- | --- |
| `envs/dev.tfvars` | ap-southeast-1 | 单 NAT、RDS 单可用区、Redis 单节点 |
| `envs/prod.tfvars` | ap-southeast-1 | 每可用区一个 NAT、RDS 多可用区、Redis 一主一从并开启 TLS |
| `envs/prod-dr.tfvars` | ap-northeast-1 | 另一套独立资源，不是 prod 的双活 |

EKS、NAT、RDS 都会产生费用。先 plan，确认账号和区域再 apply。

```bash
bash scripts/terraform.sh dev plan
bash scripts/terraform.sh dev apply
```

脚本为每个环境创建独立 workspace。RDS 小版本和 EKS 版本要和该区域当前支持的版本一致，不对就改 tfvars。

MySQL 只允许 EKS 集群安全组访问 3306，Redis 同样只开放 6379。口令由 `random_password` 生成，不写进 tfvars。

拿到 kubeconfig 之后：

```bash
$(terraform -chdir=infra/terraform output -raw update_kubeconfig)
bash scripts/render-prod-env.sh
```

`render-prod-env.sh` 把当前 workspace 的数据库和 Redis 地址写入 `deploy/k8s/overlays/prod/secret.env`，并改掉该目录里的 `REDIS_ADDR`。然后：

```bash
kubectl apply -k deploy/k8s/overlays/prod
```

仓库里的 `secret.env` 是 `replace-me` 占位。覆盖之后不要把真实口令提交回去。

阿里云上的对应关系是 VPC、ACK、RDS、云数据库 Redis。模块边界可以照搬，资源类型要换成阿里云 provider，这里不放一份未经验证的模板。

## GitLab CI

`.gitlab-ci.yml` 分三段：`go test`、构建并推送两个镜像、手动部署。部署任务需要这些 CI 变量：

- `KUBE_CONFIG`：base64 之后的 kubeconfig
- `MYSQL_DSN`
- `REDIS_PASSWORD`
- `REDIS_ADDR`，形如 `xxx.cache.amazonaws.com:6379`
- `REDIS_TLS`，`true` 或 `false`

部署前会把镜像名改成 `$CI_REGISTRY_IMAGE`，标签改成提交短 SHA。
