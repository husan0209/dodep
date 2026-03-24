# EKS Module for Opus Casino

Создаёт managed EKS кластер с node groups и аддонами.

## Особенности

- Managed EKS кластер
- Node groups с autoscaling
- Karpenter для advanced autoscaling
- IRSA (IAM Roles for Service Accounts)
- EBS CSI driver
- AWS Load Balancer Controller
- External DNS
- cert-manager

## Usage

```hcl
module "eks" {
  source = "../modules/eks"

  environment     = "dev"
  cluster_name    = "opus-casino-dev"
  cluster_version = "1.28"
  vpc_id          = module.vpc.vpc_id
  subnet_ids      = module.vpc.private_subnet_ids

  node_groups = {
    system = {
      instance_types = ["t3.large"]
      min_size       = 2
      max_size       = 5
      desired_size   = 3
      capacity_type  = "SPOT"
    }
    application = {
      instance_types = ["t3.xlarge"]
      min_size       = 2
      max_size       = 10
      desired_size   = 3
      capacity_type  = "SPOT"
    }
  }

  enable_irsa = true
  enable_ebs_csi = true
}
```

## Outputs

| Name | Description |
|------|-------------|
| `cluster_name` | Имя EKS кластера |
| `cluster_endpoint` | Endpoint кластера |
| `cluster_arn` | ARN кластера |
| `cluster_security_group_id` | Security group ID |
| `oidc_provider_arn` | OIDC provider ARN |
| `oidc_provider_url` | OIDC provider URL |
| `node_group_arns` | ARNs node groups |

## Требования

- Terraform >= 1.6
- AWS Provider >= 5.0
- EKS кластер требует IAM роли для создания
