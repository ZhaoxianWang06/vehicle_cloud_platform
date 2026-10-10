resource "random_password" "this" {
  length  = 24
  special = false
}

resource "aws_security_group" "this" {
  name        = "${var.name}-mysql"
  description = "MySQL from the EKS cluster security group"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-mysql" })
}

resource "aws_security_group_rule" "from_cluster" {
  type                     = "ingress"
  security_group_id        = aws_security_group.this.id
  source_security_group_id = var.allowed_security_group_id
  protocol                 = "tcp"
  from_port                = 3306
  to_port                  = 3306
  description              = "EKS nodes and control plane ENIs"
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.name}-mysql"
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_db_instance" "this" {
  identifier                 = "${var.name}-mysql"
  engine                     = "mysql"
  engine_version             = var.engine_version
  instance_class             = var.instance_class
  allocated_storage          = 20
  max_allocated_storage      = 100
  storage_type               = "gp3"
  storage_encrypted          = true
  db_name                    = "vehicle_cloud"
  username                   = "vehicle"
  password                   = random_password.this.result
  db_subnet_group_name       = aws_db_subnet_group.this.name
  vpc_security_group_ids     = [aws_security_group.this.id]
  multi_az                   = var.multi_az
  publicly_accessible        = false
  backup_retention_period    = var.backup_retention
  skip_final_snapshot        = var.skip_final_snapshot
  final_snapshot_identifier  = var.skip_final_snapshot ? null : "${var.name}-mysql-final"
  deletion_protection        = var.deletion_protection
  apply_immediately          = true
  auto_minor_version_upgrade = true
  tags                       = var.tags
}
