# ArgoCD Configuration for Opus Casino

## Структура

```
argocd/
├── install/           ← ArgoCD installation manifests
├── projects/          ← ArgoCD projects
├── applications/      ← ArgoCD applications
└── app-of-apps/       ← App of Apps pattern
```

## Быстрый старт

### Установка ArgoCD

```bash
# Install ArgoCD
kubectl apply -f install/argocd.yaml

# Wait for installation
kubectl wait --for=condition=Available deployment/argocd-server -n argocd

# Get initial password
kubectl -n argocd get secret argocd-initial-password-secret \
  -o jsonpath="{.data.password}" | base64 -d

# Login
argocd login argocd.opus.casino --username admin --password <password>
```

### Создание проектов

```bash
# Create projects
kubectl apply -f projects/

# Create applications
kubectl apply -f applications/
```

## Проекты

| Проект | Описание | Namespaces |
|--------|----------|------------|
| platform | Инфраструктурные компоненты | argocd, istio-system, monitoring |
| services | Микросервисы платформы | dev, staging, production |
| data | Базы данных и кэш | data |

## Приложения

| Приложение | Проект | Репозиторий | Path |
|------------|--------|-------------|------|
| argocd | platform | infra/helm/charts/argocd | |
| istio | platform | infra/k8s/istio | |
| monitoring | platform | infra/k8s/monitoring | |
| betting-engine | services | infra/helm/charts/betting-engine | |
| wallet-core | services | infra/helm/charts/wallet-core | |
| auth | services | infra/helm/charts/auth | |
