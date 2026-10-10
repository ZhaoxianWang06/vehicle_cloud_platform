# 第二区域是一套独立的 VPC、集群和数据库，不是和 prod 共享存储的双活。
# 车况按区域入库。车控如果要跨区域重试，需要单独的冲突处理，这里没有假装已经做好。
region                     = "ap-northeast-1"
env                        = "prod-dr"
name                       = "vc-prod-dr"
nat_per_az                 = true
k8s_version                = "1.31"
node_instance_types        = ["t3.large"]
node_desired               = 3
node_min                   = 3
node_max                   = 8
db_instance_class          = "db.t3.medium"
db_multi_az                = true
db_backup_retention        = 7
db_deletion_protection     = true
skip_final_snapshot        = false
redis_node_type            = "cache.t3.small"
redis_replicas             = 2
redis_transit_encryption   = true
