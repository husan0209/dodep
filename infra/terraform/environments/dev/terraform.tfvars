# Dev Environment Terraform Variables
# Development окружение - минимальные ресурсы для разработки

# AWS Configuration
aws_region = "us-east-1"

# VPC Configuration
vpc_cidr = "10.0.0.0/16"

# Cluster Configuration
cluster_name    = "opus-casino-dev"
cluster_version = "1.28"

# Node Groups - Spot instances для экономии
system_node_min_size     = 2
system_node_max_size     = 5
system_node_desired_size = 3

application_node_min_size     = 2
application_node_max_size     = 10
application_node_desired_size = 3

# Database - маленький инстанс для dev
db_instance_class      = "db.t3.medium"
db_allocated_storage   = 50

# Redis - 1 узел для dev
redis_node_type        = "cache.t3.medium"
redis_num_cache_nodes  = 1

# Tags
additional_tags = {
  Environment = "dev"
  Team        = "platform"
  CostCenter  = "engineering"
}
