# Terraform Environment Variables for Opus Casino

## Структура окружений

```
environments/
├── dev/          # Development окружение
├── staging/      # Staging окружение
└── production/   # Production окружение
```

## Быстрый старт

### Инициализация dev окружения

```bash
cd environments/dev
terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

### Переменные окружения

```bash
# AWS credentials
export AWS_PROFILE=opus-casino
export AWS_REGION=us-east-1

# Или через access keys
export AWS_ACCESS_KEY_ID=xxx
export AWS_SECRET_ACCESS_KEY=xxx
```

## Различия между окружениями

| Параметр | Dev | Staging | Production |
|----------|-----|---------|------------|
| Node count | 3-5 | 5-8 | 10-15 |
| Instance type | t3.large | t3.xlarge | m6i.2xlarge |
| Spot instances | 100% | 50% | 0% |
| Multi-AZ | No | Yes | Yes (3 AZ) |
| Autoscaling | Yes | Yes | Yes |
| Backup retention | 7 days | 14 days | 30 days |
