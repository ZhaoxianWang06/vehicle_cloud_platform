output "endpoint" { value = aws_elasticache_replication_group.this.primary_endpoint_address }
output "auth_token" {
  value     = var.transit_encryption_enabled ? random_password.auth[0].result : ""
  sensitive = true
}
