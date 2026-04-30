# Terraform Infrastructure for Opus Casino

## Структура

```
terraform/
├── modules/           # Переиспользуемые модули
│   ├── vpc/          # VPC и сеть
│   ├── eks/          # Kubernetes кластер
│   ├── karpenter/    # Autoscaling
│   ├── iam/          # IAM роли и политики
│   ├── s3-state/     # S3 bucket для state
│   ├── kms/          # KMS ключи
│   ├── rds/          # PostgreSQL
│   ├── redis/        # Redis/Dragonfly
│   └── cloudflare/   # CloudFlare настройки
├── environments/      # Окружения
│   ├── dev/          # Development
│   ├── staging/      # Staging
│   └── production/   # Production
└── scripts/          # Вспомогательные скрипты
```

## Быстрый старт

### Инициализация state backend

```bash
# Создать S3 bucket для state
aws s3 mb s3://opus-casino-terraform-state --region us-east-1

# Включить versioning
aws s3api put-bucket-versioning \
  --bucket opus-casino-terraform-state \
  --versioning-configuration Status=Enabled

# Создать DynamoDB table для lock
aws dynamodb create-table \
  --table-name opus-casino-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

### Запуск для dev окружения

```bash
cd environments/dev
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

## Требования

- Terraform >= 1.6
- AWS CLI >= 2.0
- kubectl >= 1.28

## Переменные окружения

```bash
export AWS_PROFILE=opus-casino
export AWS_REGION=us-east-1
```
