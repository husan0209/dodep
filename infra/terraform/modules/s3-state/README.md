# S3 State Backend Module

Создаёт S3 bucket для хранения Terraform state и DynamoDB table для lock.

## Usage

```hcl
module "state_backend" {
  source = "../../modules/s3-state"

  environment = "dev"
  bucket_name = "opus-casino-terraform-state"
}
```

## Outputs

| Name | Description |
|------|-------------|
| `bucket_name` | Имя S3 bucket |
| `bucket_arn` | ARN S3 bucket |
| `dynamodb_table_name` | Имя DynamoDB table |
| `dynamodb_table_arn` | ARN DynamoDB table |
| `kms_key_id` | ID KMS ключа для шифрования |
