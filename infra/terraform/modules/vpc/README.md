# VPC Module for Opus Casino

Создаёт VPC с публичными и приватными подсетями в нескольких AZ.

## Особенности

- 3 Availability Zones
- Public subnets для NAT Gateway и Load Balancers
- Private subnets для EKS нод
- Isolated subnets для баз данных
- NAT Gateway для исходящего трафика
- Internet Gateway для публичного доступа
- VPC Flow Logs для аудита

## Usage

```hcl
module "vpc" {
  source = "../../modules/vpc"

  environment = "dev"
  vpc_cidr    = "10.0.0.0/16"
  azs         = ["us-east-1a", "us-east-1b", "us-east-1c"]
  
  public_subnet_cidrs  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  private_subnet_cidrs = ["10.0.11.0/24", "10.0.12.0/24", "10.0.13.0/24"]
  isolated_subnet_cidrs = ["10.0.21.0/24", "10.0.22.0/24", "10.0.23.0/24"]
  
  enable_nat_gateway   = true
  single_nat_gateway   = false
  enable_flow_logs     = true
}
```

## Outputs

| Name | Description |
|------|-------------|
| `vpc_id` | ID VPC |
| `vpc_cidr` | CIDR блок VPC |
| `public_subnet_ids` | IDs публичных подсетей |
| `private_subnet_ids` | IDs приватных подсетей |
| `isolated_subnet_ids` | IDs изолированных подсетей |
| `nat_gateway_ids` | IDs NAT Gateway |
| `internet_gateway_id` | ID Internet Gateway |

## Требования

- Terraform >= 1.6
- AWS Provider >= 5.0
