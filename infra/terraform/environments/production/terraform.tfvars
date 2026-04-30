# Production Environment Terraform Variables
# Production окружение - максимальная надёжность

# AWS Configuration
aws_region       = "us-east-1"
aws_region_replica = "us-west-2"

# VPC Configuration
vpc_cidr = "10.2.0.0/16"

# Cluster Configuration
cluster_name    = "opus-casino-production"
cluster_version = "1.28"

# Node Groups - Production (все ON_DEMAND для критических workload'ов)
system_node_min_size     = 5
system_node_max_size     = 15
system_node_desired_size = 8

application_node_min_size     = 10
application_node_max_size     = 40
application_node_desired_size = 15

data_node_min_size     = 6
data_node_max_size     = 20
data_node_desired_size = 10

spot_node_min_size     = 5
spot_node_max_size     = 30
spot_node_desired_size = 10

# Database - Production конфигурация
db_instance_class    = "db.r5.2xlarge"
db_allocated_storage = 500
db_multi_az          = true
db_read_replicas     = 2

# Redis - Cluster mode для production
redis_node_type       = "cache.r5.2xlarge"
redis_num_cache_nodes = 3
redis_cluster_mode    = true

# Tags
additional_tags = {
  Environment = "production"
  Team        = "platform"
  CostCenter  = "production"
  Compliance  = "SOC2"
  Criticality = "high"
}
