#!/usr/bin/env bash
# 读取当前 Terraform workspace 的输出，写成 prod overlay 要用的连接信息。
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root/infra/terraform"
mysql_host=$(terraform output -raw mysql_endpoint)
mysql_pass=$(terraform output -raw mysql_password)
redis_host=$(terraform output -raw redis_endpoint)
redis_pass=$(terraform output -raw redis_auth_token)
tls=false
if [[ -n "$redis_pass" ]]; then
  tls=true
fi
umask 077
cat > "$root/deploy/k8s/overlays/prod/secret.env" <<EOF
MYSQL_DSN=vehicle:${mysql_pass}@tcp(${mysql_host}:3306)/vehicle_cloud?parseTime=true&charset=utf8mb4&loc=UTC
REDIS_PASSWORD=${redis_pass}
EOF
kfile="$root/deploy/k8s/overlays/prod/kustomization.yaml"
sed -i "s#REDIS_ADDR=.*#REDIS_ADDR=${redis_host}:6379#" "$kfile"
sed -i "s#REDIS_TLS=.*#REDIS_TLS=${tls}#" "$kfile"
echo "wrote deploy/k8s/overlays/prod/secret.env"
echo "redis ${redis_host}:6379 tls=${tls}"
echo "下一步：kubectl apply -k deploy/k8s/overlays/prod"
