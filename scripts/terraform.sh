#!/usr/bin/env bash
# 用法：scripts/terraform.sh dev plan
# 每个环境一个 workspace，state 不会串。
set -euo pipefail
env_name="${1:?usage: scripts/terraform.sh dev|prod|prod-dr plan|apply}"
action="${2:-plan}"
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root/infra/terraform"
terraform init
if ! terraform workspace select "$env_name"; then
  terraform workspace new "$env_name"
fi
terraform "$action" -var-file="envs/${env_name}.tfvars"
