# Staging Environment Terraform Variables
# Staging окружение - ближе к production

# AWS Configuration
aws_region = "us-east-1"

# VPC Configuration
vpc_cidr = "10.1.0.0/16"

# Cluster Configuration
cluster_name    = "opus-casino-staging"
cluster_version = "1.28"

# Node Groups - Mixed Spot + On-Demand
system_node_min_size     = 3
system_node_max_size     = 8
system_node_desired_size = 5

application_node_min_size     = 3
application_node_max_size     = 15
application_node_desired_size = 5

data_node_min_size     = 2
data_node_max_size     = 6
data_node_desired_size = 3

# Database - больше чем dev
db_instance_class      = "db.r5.large"
db_allocated_storage   = 100

# Redis - 2 узла для отказоустойчивости
redis_node_type        = "cache.r5.large"
redis_num_cache_nodes  = 2

# Tags
additional_tags = {
  Environment = "staging"
  Team        = "platform"
  CostCenter  = "engineering"
}
