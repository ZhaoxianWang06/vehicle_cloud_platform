output "cluster_name" {
  value = module.eks.cluster_name
}

output "update_kubeconfig" {
  value = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}

output "mysql_endpoint" {
  description = "RDS 主机名，不含端口。"
  value       = module.mysql.endpoint
}

output "mysql_password" {
  value     = module.mysql.password
  sensitive = true
}

output "redis_endpoint" {
  description = "Redis 主节点主机名，不含端口。"
  value       = module.redis.endpoint
}

output "redis_auth_token" {
  value     = module.redis.auth_token
  sensitive = true
}
