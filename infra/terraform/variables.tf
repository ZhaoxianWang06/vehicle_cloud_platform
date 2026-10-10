variable "region" {
  type        = string
  description = "部署区域。第二个区域用 envs/prod-dr.tfvars，而不是把两套集群写进同一份状态。"
  default     = "ap-southeast-1"
}

variable "env" {
  type    = string
  default = "dev"
}

variable "name" {
  type        = string
  description = "资源名前缀，账户内必须唯一，且只能使用小写字母、数字和中划线。"
  default     = "vc-dev"
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

variable "nat_per_az" {
  type        = bool
  description = "true 时每个可用区一个 NAT，避免单点；dev 用一个 NAT 控制费用。"
  default     = false
}

variable "k8s_version" {
  type        = string
  description = "EKS 版本。apply 前用 aws eks describe-cluster-versions 核对当前仍在支持窗口内的版本。"
  default     = "1.31"
}

variable "node_instance_types" {
  type    = list(string)
  default = ["t3.medium"]
}

variable "node_desired" {
  type    = number
  default = 2
}

variable "node_min" {
  type    = number
  default = 2
}

variable "node_max" {
  type    = number
  default = 4
}

variable "db_instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "db_engine_version" {
  type        = string
  description = "RDS MySQL 小版本。区域里没有这个版本时改成控制台给出的当前 8.0 小版本。"
  default     = "8.0.40"
}

variable "db_multi_az" {
  type    = bool
  default = false
}

variable "db_backup_retention" {
  type    = number
  default = 1
}

variable "db_deletion_protection" {
  type    = bool
  default = false
}

variable "skip_final_snapshot" {
  type    = bool
  default = true
}

variable "redis_node_type" {
  type    = string
  default = "cache.t3.micro"
}

variable "redis_engine_version" {
  type    = string
  default = "7.1"
}

variable "redis_replicas" {
  type        = number
  description = "缓存节点总数，含主节点。大于 1 时打开多可用区和自动故障转移。"
  default     = 1
}

variable "redis_transit_encryption" {
  type    = bool
  default = false
}
