locals {
  tags = {
    Project = "vehicle-cloud"
    Env     = var.env
  }
}

module "network" {
  source       = "./modules/network"
  name         = var.name
  cidr         = var.vpc_cidr
  cluster_name = var.name
  nat_per_az   = var.nat_per_az
  tags         = local.tags
}

module "eks" {
  source          = "./modules/eks"
  name            = var.name
  k8s_version     = var.k8s_version
  subnet_ids      = concat(module.network.private_subnet_ids, module.network.public_subnet_ids)
  node_subnet_ids = module.network.private_subnet_ids
  instance_types  = var.node_instance_types
  desired_size    = var.node_desired
  min_size        = var.node_min
  max_size        = var.node_max
  tags            = local.tags
}

module "mysql" {
  source                    = "./modules/rds"
  name                      = var.name
  vpc_id                    = module.network.vpc_id
  subnet_ids                = module.network.private_subnet_ids
  allowed_security_group_id = module.eks.cluster_security_group_id
  instance_class            = var.db_instance_class
  engine_version            = var.db_engine_version
  multi_az                  = var.db_multi_az
  backup_retention          = var.db_backup_retention
  deletion_protection       = var.db_deletion_protection
  skip_final_snapshot       = var.skip_final_snapshot
  tags                      = local.tags
}

module "redis" {
  source                     = "./modules/redis"
  name                       = var.name
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = module.network.private_subnet_ids
  allowed_security_group_id  = module.eks.cluster_security_group_id
  node_type                  = var.redis_node_type
  engine_version             = var.redis_engine_version
  num_cache_clusters         = var.redis_replicas
  transit_encryption_enabled = var.redis_transit_encryption
  tags                       = local.tags
}
