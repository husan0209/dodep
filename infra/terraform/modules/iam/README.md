# IAM Module for Opus Casino

Создаёт IAM роли и политики для сервисов платформы.

## Usage

```hcl
module "iam" {
  source = "../../modules/iam"

  environment = "dev"
  
  # EKS roles
  enable_eks_roles = true
  
  # RDS roles
  enable_rds_roles = true
  
  # Lambda roles
  enable_lambda_roles = false
}
```

## Outputs

| Name | Description |
|------|-------------|
| `eks_cluster_role_arn` | ARN роли для EKS кластера |
| `eks_node_role_arn` | ARN роли для EKS нод |
| `rds_role_arn` | ARN роли для RDS |
