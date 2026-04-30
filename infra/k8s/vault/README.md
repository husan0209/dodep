# HashiCorp Vault Configuration
# Secrets management для Opus Casino platform

## Структура

```
vault/
├── install/           ← Vault installation manifests
├── config/            ← Vault configuration
├── policies/          ← Vault policies
└── secrets/           ← Secret engines configuration
```

## Быстрый старт

### Установка Vault

```bash
# Install Vault
kubectl apply -f install/

# Initialize Vault
kubectl exec -it vault-0 -n security -- vault operator init

# Unseal Vault
kubectl exec -it vault-0 -n security -- vault operator unseal <key1>
kubectl exec -it vault-1 -n security -- vault operator unseal <key1>
kubectl exec -it vault-2 -n security -- vault operator unseal <key1>

# Login
kubectl exec -it vault-0 -n security -- vault login
```

### Настройка секретов

```bash
# Enable KV secrets engine
vault secrets enable -path=secret kv-v2

# Create secret
vault kv put secret/database/postgres username=postgres password=xxx

# Read secret
vault kv get secret/database/postgres
```

### Kubernetes auth

```bash
# Enable Kubernetes auth
vault auth enable kubernetes

# Configure Kubernetes auth
vault write auth/kubernetes/config \
  kubernetes_host="https://kubernetes.default.svc:443" \
  kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
```

## Secret Engines

| Engine | Path | Описание |
|--------|------|----------|
| KV v2 | secret/ | Static secrets |
| Database | database/ | Dynamic DB credentials |
| PKI | pki/ | Certificates |
| Transit | transit/ | Encryption as a service |

## Policies

| Policy | Описание |
|--------|----------|
| betting-engine | Доступ к DB, Redis, Kafka secrets |
| wallet-core | Доступ к DB, KMS, Redis secrets |
| auth | Доступ к DB, JWT secrets, Redis |
