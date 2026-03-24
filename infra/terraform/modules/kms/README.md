# KMS Module for Opus Casino

Создаёт KMS ключи для шифрования данных.

## Usage

```hcl
module "kms" {
  source = "../../modules/kms"

  environment = "dev"
  
  keys = {
    rds        = {}
    s3         = {}
    secrets    = {}
    ebs        = {}
  }
}
```

## Outputs

| Name | Description |
|------|-------------|
| `key_arns` | ARN созданных ключей |
| `key_ids` | IDs созданных ключей |
