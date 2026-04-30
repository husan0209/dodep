# S3 Buckets Module

Production-ready S3 модуль для Opus Casino с versioning, lifecycle policies, encryption и cross-region replication.

## Особенности

- **Versioning** — сохранение всех версий объектов
- **Lifecycle Policies** — автоматическая миграция на холодные storage классы
- **Encryption** — KMS шифрование (server-side)
- **Block Public Access** — все bucket'ы приватные
- **Cross-Region Replication** — репликация в DR регион
- **CloudWatch Alarms** — мониторинг размера bucket'ов
- **Bucket Policies** — поддержка custom policies

## Использование

```hcl
module "s3" {
  source = "../modules/s3-buckets"

  environment = "production"

  # Buckets to create
  buckets = {
    uploads  = ""  # User uploads (images, documents)
    backups  = ""  # Database backups
    logs     = ""  # Application logs
    assets   = ""  # Static assets (CDN origin)
    archives = ""  # Long-term archives
  }

  # Versioning
  enable_versioning = true

  # Encryption
  enable_encryption = true

  # Lifecycle rules
  lifecycle_rules = {
    uploads = [{
      id     = "transition-to-ia"
      prefix = ""
      transitions = [{
        days          = 30
        storage_class = "STANDARD_IA"
      }]
      expiration = {
        days = 365
      }
      noncurrent_version_expiration = {
        noncurrent_days = 30
      }
    }]
    logs = [{
      id     = "transition-to-glacier"
      prefix = ""
      transitions = [{
        days          = 30
        storage_class = "STANDARD_IA"
      }, {
        days          = 90
        storage_class = "GLACIER"
      }]
      expiration = {
        days = 730  # 2 years
      }
    }]
    backups = [{
      id     = "keep-backups"
      prefix = ""
      noncurrent_version_expiration = {
        noncurrent_days = 90
      }
    }]
  }

  # Cross-region replication
  enable_cross_region_replication     = true
  replication_destination_bucket_arn  = "arn:aws:s3:::dr-region-bucket"

  # Alarms
  enable_size_alarm    = true
  size_threshold_bytes = 1099511627776  # 1 TB
  alarm_actions        = [aws_sns_topic.platform_alerts.arn]

  tags = {
    Project = "opus-casino"
  }
}
```

## Архитектура

```
┌─────────────────────────────────────────────────────────────┐
│                    US-East-1 (Primary)                       │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  S3 Buckets (KMS Encrypted + Versioning)             │   │
│  │                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│  │  │  uploads    │  │   backups   │  │    logs     │  │   │
│  │  │  (images)   │  │   (RDS)     │  │  (app logs) │  │   │
│  │  │  IA > 30d   │  │  keep 90d   │  │  GLR > 90d  │  │   │
│  │  │  del > 365d │  │             │  │  del > 730d │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │   │
│  │                                                       │   │
│  │  ┌─────────────┐  ┌─────────────┐                    │   │
│  │  │   assets    │  │  archives   │                    │   │
│  │  │  (CDN)      │  │ (compliance)│                    │   │
│  │  │  standard   │  │  deep archive│                   │   │
│  │  └─────────────┘  └─────────────┘                    │   │
│  └──────────────────────────────────────────────────────┘   │
│                            │                                 │
│                            │ replication                    │
│                            ▼                                 │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                   US-West-2 (DR)                      │   │
│  │                                                       │   │
│  │  ┌────────────────────────────────────────────────┐  │   │
│  │  │           Replicated Buckets                   │  │   │
│  │  │  (read-only, for disaster recovery)            │  │   │
│  │  └────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Storage Classes

| Class | Назначение | Стоимость (за GB) |
|-------|------------|------------------|
| STANDARD | Частый доступ | $0.023 |
| STANDARD_IA | Редкий доступ (30+ дней) | $0.0125 |
| ONEZONE_IA | Очень редкий (одна AZ) | $0.01 |
| GLACIER | Архив (минуты retrieval) | $0.004 |
| GLACIER_DEEP_ARCHIVE | Долгий архив (12+ часов) | $0.00099 |

## Lifecycle Rules Examples

### User Uploads

```hcl
uploads = [{
  id     = "lifecycle-uploads"
  prefix = ""
  transitions = [
    {
      days          = 30
      storage_class = "STANDARD_IA"
    },
    {
      days          = 90
      storage_class = "GLACIER"
    }
  ]
  expiration = {
    days = 365
  }
  noncurrent_version_expiration = {
    noncurrent_days = 30
  }
}]
```

### Application Logs

```hcl
logs = [{
  id     = "lifecycle-logs"
  prefix = ""
  transitions = [
    {
      days          = 30
      storage_class = "STANDARD_IA"
    },
    {
      days          = 90
      storage_class = "GLACIER"
    }
  ]
  expiration = {
    days = 730  # 2 года
  }
}]
```

### Database Backups

```hcl
backups = [{
  id     = "lifecycle-backups"
  prefix = ""
  # Храним в STANDARD для быстрого восстановления
  noncurrent_version_expiration = {
    noncurrent_days = 90
  }
}]
```

## Bucket Policies

Пример policy для uploads bucket (разрешить загрузку только с определённого IAM role):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowUploadFromApp",
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::123456789012:role/opus-casino-app"
      },
      "Action": [
        "s3:PutObject",
        "s3:GetObject"
      ],
      "Resource": "arn:aws:s3:::production-opus-uploads/*"
    }
  ]
}
```

```hcl
bucket_policies = {
  uploads = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowUploadFromApp"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/opus-casino-app"
        }
        Action = [
          "s3:PutObject",
          "s3:GetObject"
        ]
        Resource = "${aws_s3_bucket.main["uploads"].arn}/*"
      }
    ]
  })
}
```

## CloudWatch Alarms

Модуль создаёт alarm для каждого bucket:

- **Bucket Size** > threshold (по умолчанию 1 TB)

```hcl
enable_size_alarm    = true
size_threshold_bytes = 1099511627776  # 1 TB
alarm_actions        = [aws_sns_topic.platform_alerts.arn]
```

## Cross-Region Replication

Для включения репликации:

```hcl
enable_cross_region_replication     = true
replication_destination_bucket_arn  = "arn:aws:s3:::dr-region-bucket"
replication_storage_class = {
  uploads  = "STANDARD_IA"
  backups  = "STANDARD"
  logs     = "GLACIER"
}
```

## Стоимость (пример для production)

```
uploads:     500GB STANDARD + 200GB IA + 100GB Glacier  ~$20/месяц
backups:     1TB STANDARD                                ~$23/месяц
logs:        200GB STANDARD + 500GB IA + 1TB Glacier    ~$15/месяц
assets:      100GB STANDARD                              ~$2.3/месяц
archives:    500GB Deep Archive                          ~$0.5/месяц

Replication (в DR регион):
  Data transfer: 500GB × $0.02 = ~$10/месяц
  DR storage:    ~$20/месяц

Requests & API calls:                                  ~$5/месяц
─────────────────────────────────────────────────────────────
Итого: ~$95/месяц
```

## Best Practices

### Безопасность

1. Block Public Access включён для всех bucket'ов
2. KMS encryption обязателен для production
3. Используйте bucket policies для ограничения доступа
4. Включите versioning для критичных данных
5. Access logging для аудита

### Оптимизация затрат

1. Настройте lifecycle rules для миграции на IA/Glacier
2. Удаляйте старые версии объектов
3. Используйте Intelligent-Tiering для unpredictable access
4. Сжимайте логи перед загрузкой

### Надёжность

1. Cross-region replication для critical data
2. Versioning для защиты от accidental deletion
3. Регулярно тестируйте восстановление из backup
4. Мониторьте размер bucket'ов

## Доступ к bucket'ам из приложения

```python
# Python (boto3)
import boto3
from botocore.config import Config

s3 = boto3.client('s3', config=Config(
    retries={'max_attempts': 3}
))

# Upload
s3.upload_file('local.jpg', 'production-opus-uploads', 'user/123/image.jpg')

# Generate presigned URL (для загрузки клиентом)
url = s3.generate_presigned_url(
    'put_object',
    Params={'Bucket': 'production-opus-uploads', 'Key': 'user/123/image.jpg'},
    ExpiresIn=3600
)
```

```rust
// Rust (aws-sdk-s3)
use aws_sdk_s3::{Client, Config};

let config = Config::builder()
    .region(Region::new("us-east-1"))
    .retry_strategy(StandardRetryStrategy::default())
    .build();

let client = Client::from_conf(config);

// Upload
client.put_object()
    .bucket("production-opus-uploads")
    .key("user/123/image.jpg")
    .body(file_content.into())
    .send()
    .await?;
```
