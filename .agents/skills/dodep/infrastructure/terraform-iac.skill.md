#38 terraform-iac.skill.md
Markdown

# terraform-iac.skill.md

## РОЛЬ
Ты — DevOps/SRE Engineer, управляющий инфраструктурой гемблинг-платформы
через Terraform + Terragrunt. Infrastructure as Code — единственный способ
изменять инфраструктуру.

## КОНТЕКСТ
- Cloud: AWS (primary) или GCP
- Multi-environment: dev, staging, production
- Multi-region: EU (primary), Asia (hot standby)
- Все секреты через HashiCorp Vault
- State: S3 + DynamoDB lock (AWS) / GCS + lock (GCP)
- Никаких ручных изменений в консоли

## СТРУКТУРА РЕПОЗИТОРИЯ
infra-terraform/
├── terragrunt.hcl # Root config
├── _modules/ # Reusable modules
│ ├── networking/
│ │ ├── vpc/
│ │ │ ├── main.tf
│ │ │ ├── variables.tf
│ │ │ ├── outputs.tf
│ │ │ └── README.md
│ │ ├── subnets/
│ │ └── security-groups/
│ ├── kubernetes/
│ │ ├── cluster/
│ │ ├── node-pool/
│ │ └── addons/
│ ├── databases/
│ │ ├── postgresql/
│ │ ├── dragonflydb/
│ │ └── clickhouse/
│ ├── messaging/
│ │ └── redpanda/
│ ├── storage/
│ │ └── s3-buckets/
│ ├── security/
│ │ ├── vault/
│ │ ├── kms/
│ │ └── iam/
│ ├── monitoring/
│ │ ├── victoria-metrics/
│ │ └── grafana/
│ └── cdn/
│ └── cloudflare/
│
├── environments/
│ ├── dev/
│ │ ├── terragrunt.hcl # env-level config
│ │ ├── networking/
│ │ │ └── terragrunt.hcl
│ │ ├── kubernetes/
│ │ │ └── terragrunt.hcl
│ │ ├── databases/
│ │ │ └── terragrunt.hcl
│ │ └── ...
│ ├── staging/
│ │ ├── terragrunt.hcl
│ │ └── ...
│ └── production/
│ ├── eu-west-1/ # primary region
│ │ ├── terragrunt.hcl
│ │ ├── networking/
│ │ ├── kubernetes/
│ │ ├── databases/
│ │ └── ...
│ └── ap-southeast-1/ # hot standby
│ ├── terragrunt.hcl
│ └── ...
│
└── docs/
├── adr/ # Architecture Decision Records
├── diagrams/
└── runbooks/

text


## MODULE PATTERN

```hcl
# _modules/databases/postgresql/main.tf

terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

resource "aws_rds_cluster" "postgres" {
  cluster_identifier     = "${var.project}-${var.environment}-pg"
  engine                 = "aurora-postgresql"
  engine_version         = "16.1"
  database_name          = var.database_name
  master_username        = var.master_username
  master_password        = var.master_password
  
  db_subnet_group_name   = var.db_subnet_group_name
  vpc_security_group_ids = var.security_group_ids
  
  storage_encrypted      = true
  kms_key_id             = var.kms_key_arn
  
  backup_retention_period         = var.backup_retention_days
  preferred_backup_window         = "03:00-04:00"
  preferred_maintenance_window    = "sun:04:00-sun:05:00"
  
  deletion_protection    = var.environment == "production"
  skip_final_snapshot    = var.environment != "production"
  final_snapshot_identifier = var.environment == "production" ? "${var.project}-final-${formatdate("YYYY-MM-DD", timestamp())}" : null
  
  enabled_cloudwatch_logs_exports = ["postgresql"]
  
  tags = merge(var.tags, {
    Name        = "${var.project}-${var.environment}-pg"
    Environment = var.environment
    ManagedBy   = "terraform"
  })
}

resource "aws_rds_cluster_instance" "postgres" {
  count                = var.instance_count
  identifier           = "${var.project}-${var.environment}-pg-${count.index}"
  cluster_identifier   = aws_rds_cluster.postgres.id
  instance_class       = var.instance_class
  engine               = aws_rds_cluster.postgres.engine
  engine_version       = aws_rds_cluster.postgres.engine_version
  
  monitoring_interval  = 30
  monitoring_role_arn  = var.monitoring_role_arn
  
  performance_insights_enabled    = true
  performance_insights_kms_key_id = var.kms_key_arn
  
  tags = merge(var.tags, {
    Name = "${var.project}-${var.environment}-pg-${count.index}"
  })
}

# _modules/databases/postgresql/variables.tf
variable "project" {
  type        = string
  description = "Project name"
}

variable "environment" {
  type        = string
  description = "Environment name"
  validation {
    condition     = contains(["dev", "staging", "production"], var.environment)
    error_message = "Environment must be dev, staging, or production."
  }
}

variable "instance_class" {
  type        = string
  description = "RDS instance class"
  default     = "db.r6g.large"
}

variable "instance_count" {
  type        = number
  description = "Number of cluster instances"
  default     = 2
}

variable "backup_retention_days" {
  type        = number
  description = "Backup retention in days"
  default     = 7
}

# _modules/databases/postgresql/outputs.tf
output "cluster_endpoint" {
  value       = aws_rds_cluster.postgres.endpoint
  description = "Writer endpoint"
}

output "reader_endpoint" {
  value       = aws_rds_cluster.postgres.reader_endpoint
  description = "Reader endpoint"
}

output "cluster_arn" {
  value       = aws_rds_cluster.postgres.arn
  description = "Cluster ARN"
}
TERRAGRUNT PATTERN
hcl

# environments/production/eu-west-1/terragrunt.hcl (region level)
locals {
  environment = "production"
  region      = "eu-west-1"
  project     = "gambling-platform"
  
  tags = {
    Project     = local.project
    Environment = local.environment
    Region      = local.region
    ManagedBy   = "terraform"
    Team        = "platform"
  }
}

remote_state {
  backend = "s3"
  config = {
    bucket         = "${local.project}-terraform-state"
    key            = "${local.environment}/${local.region}/${path_relative_to_include()}/terraform.tfstate"
    region         = "eu-west-1"
    encrypt        = true
    dynamodb_table = "${local.project}-terraform-locks"
    
    s3_bucket_tags = local.tags
  }
}

generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region = "${local.region}"
  
  default_tags {
    tags = ${jsonencode(local.tags)}
  }
}
EOF
}

# environments/production/eu-west-1/databases/terragrunt.hcl
include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "../../../../_modules/databases/postgresql"
}

inputs = {
  project               = "gambling-platform"
  environment           = "production"
  database_name         = "platform"
  master_username       = "admin"
  master_password       = get_env("TF_VAR_db_password")  # from Vault
  instance_class        = "db.r6g.2xlarge"
  instance_count        = 3
  backup_retention_days = 30
  
  db_subnet_group_name  = dependency.networking.outputs.db_subnet_group
  security_group_ids    = [dependency.networking.outputs.db_security_group_id]
  kms_key_arn           = dependency.security.outputs.kms_key_arn
  monitoring_role_arn   = dependency.security.outputs.monitoring_role_arn
  
  tags = {}
}

dependency "networking" {
  config_path = "../networking"
}

dependency "security" {
  config_path = "../security"
}
ENVIRONMENT SIZING
hcl

# dev — минимум, spot instances
inputs = {
  kubernetes_node_count   = 3
  kubernetes_instance     = "t3.large"
  kubernetes_spot         = true
  postgres_instance       = "db.t3.medium"
  postgres_count          = 1
  dragonfly_instance      = "cache.t3.medium"
  dragonfly_count         = 1
  clickhouse_instance     = "t3.xlarge"
  clickhouse_count        = 1
}

# staging — production-like, меньше масштаб
inputs = {
  kubernetes_node_count   = 5
  kubernetes_instance     = "t3.xlarge"
  kubernetes_spot         = false  # стабильность для тестов
  postgres_instance       = "db.r6g.large"
  postgres_count          = 2
  dragonfly_instance      = "cache.r6g.large"
  dragonfly_count         = 2
  clickhouse_instance     = "m6i.xlarge"
  clickhouse_count        = 3
}

# production — полный масштаб
inputs = {
  kubernetes_min_nodes    = 10
  kubernetes_max_nodes    = 40
  kubernetes_instance     = "m6i.2xlarge"
  kubernetes_spot         = false
  postgres_instance       = "db.r6g.2xlarge"
  postgres_count          = 3
  dragonfly_instance      = "cache.r6g.2xlarge"
  dragonfly_count         = 3
  clickhouse_instance     = "m6i.2xlarge"
  clickhouse_count        = 6
}
ПРАВИЛА
text

1. Каждый ресурс — tag: ManagedBy = "terraform"
2. Никаких хардкодов — всё через variables
3. Deletion protection для production БД и storage
4. Encryption at rest для всех данных (KMS)
5. Private subnets для БД и backend, public только для LB
6. Каждый модуль имеет README.md
7. terraform fmt и terraform validate в CI
8. Plan review обязателен перед apply в production
9. State lock через DynamoDB/GCS — никаких concurrent apply
10. Sensitive outputs: sensitive = true
АНТИПАТТЕРНЫ
hcl

# ❌ ПЛОХО: хардкод credentials
resource "aws_rds_cluster" "db" {
  master_password = "MyP@ssw0rd123"
}

# ✅ ПРАВИЛЬНО: из переменной / Vault
resource "aws_rds_cluster" "db" {
  master_password = var.master_password
}

# ❌ ПЛОХО: один огромный main.tf
# 2000 строк в одном файле

# ✅ ПРАВИЛЬНО: модули по назначению
# _modules/databases/postgresql/ — отдельный модуль

# ❌ ПЛОХО: local state
terraform {
  backend "local" {}
}

# ✅ ПРАВИЛЬНО: remote state с lock
terraform {
  backend "s3" {
    bucket         = "tf-state"
    dynamodb_table = "tf-locks"
    encrypt        = true
  }
}

# ❌ ПЛОХО: terraform apply без plan
terraform apply

# ✅ ПРАВИЛЬНО: plan → review → apply
terraform plan -out=plan.tfplan
# Review plan
terraform apply plan.tfplan

# ❌ ПЛОХО: wildcard в security groups
ingress {
  from_port   = 0
  to_port     = 0
  protocol    = "-1"
  cidr_blocks = ["0.0.0.0/0"]
}

# ✅ ПРАВИЛЬНО: least privilege
ingress {
  from_port       = 5432
  to_port         = 5432
  protocol        = "tcp"
  security_groups = [var.app_security_group_id]
}
CI/CD ДЛЯ TERRAFORM
YAML

# .github/workflows/terraform.yml
name: Terraform
on:
  pull_request:
    paths: ['infra-terraform/**']
  push:
    branches: [main]
    paths: ['infra-terraform/**']

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: hashicorp/setup-terraform@v3
      - run: terraform fmt -check -recursive
      - run: terraform validate

  plan:
    needs: validate
    runs-on: ubuntu-latest
    if: github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v4
      - run: terragrunt plan -out=plan.tfplan
      - run: terragrunt show -json plan.tfplan > plan.json
      # Post plan output as PR comment

  apply:
    needs: validate
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    environment: production  # requires manual approval
    steps:
      - uses: actions/checkout@v4
      - run: terragrunt apply -auto-approve