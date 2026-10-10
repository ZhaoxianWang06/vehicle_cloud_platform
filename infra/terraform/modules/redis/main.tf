resource "random_password" "auth" {
  count   = var.transit_encryption_enabled ? 1 : 0
  length  = 32
  special = false
}

resource "aws_security_group" "this" {
  name        = "${var.name}-redis"
  description = "Redis from the EKS cluster security group"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-redis" })
}

resource "aws_security_group_rule" "from_cluster" {
  type                     = "ingress"
  security_group_id        = aws_security_group.this.id
  source_security_group_id = var.allowed_security_group_id
  protocol                 = "tcp"
  from_port                = 6379
  to_port                  = 6379
  description              = "EKS nodes and control plane ENIs"
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "${var.name}-redis"
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id       = "${var.name}-redis"
  description                = "vehicle cloud realtime state"
  engine                     = "redis"
  engine_version             = var.engine_version
  node_type                  = var.node_type
  num_cache_clusters         = var.num_cache_clusters
  automatic_failover_enabled = var.num_cache_clusters > 1
  multi_az_enabled           = var.num_cache_clusters > 1
  port                       = 6379
  subnet_group_name          = aws_elasticache_subnet_group.this.name
  security_group_ids         = [aws_security_group.this.id]
  at_rest_encryption_enabled = true
  transit_encryption_enabled = var.transit_encryption_enabled
  auth_token                 = var.transit_encryption_enabled ? random_password.auth[0].result : null
  apply_immediately          = true
  tags                       = var.tags
}
