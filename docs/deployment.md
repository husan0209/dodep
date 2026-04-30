# Развертывание

## Требования

- Kubernetes кластер (EKS/GKE)
- Terraform >= 1.7
- ArgoCD
- HashiCorp Vault

## Инфраструктура (Terraform)

```bash
cd infra/terraform

# Инициализация
terraform init

# План
terraform plan -out=tfplan

# Применить
terraform apply tfplan
```

## Деплой приложений (ArgoCD)

```bash
# Login в ArgoCD
argocd login <argocd-server>

# Sync приложения
argocd app sync opus-casino-production

# Статус
argocd app get opus-casino-production
```

## Ручной деплой (kubectl)

```bash
# Создать namespace
kubectl apply -f infra/k8s/namespace.yaml

# Применить конфиги
kubectl apply -f infra/k8s/configmap.yaml
kubectl apply -f infra/k8s/secrets.yaml

# Деплой сервисов
kubectl apply -f infra/k8s/betting-engine.yaml
kubectl apply -f infra/k8s/wallet-core.yaml
kubectl apply -f infra/k8s/auth.yaml

# Проверка
kubectl get pods -n opus-casino
```

## Canary деплой (Argo Rollouts)

```bash
# Обновление образа
kubectl argo rollouts set image betting-engine \
  betting-engine=ghcr.io/opus-casino/betting-engine:new-version

# Откат
kubectl argo rollouts abort betting-engine -n opus-casino
```

## Мониторинг деплоя

```bash
# Логи
kubectl logs -f deployment/betting-engine -n opus-casino

# Метрики
kubectl top pods -n opus-casino

# Трейсы
# Проверить Jaeger UI: https://jaeger.opus-casino.com
```
