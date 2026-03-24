# 🚀 Quick Start Guide — Opus Casino

**Полное руководство по запуску платформы локально**

---

## ⚡ Быстрый старт (5 минут)

### 1. Проверка требований

```bash
# Проверьте установленные версии
node --version      # Требуется: >= 20.0.0
npm --version       # Требуется: >= 10.0.0
rustc --version     # Требуется: >= 1.75
cargo --version     # Требуется: >= 1.75
go version          # Требуется: >= 1.21
python --version    # Требуется: >= 3.11
docker --version    # Требуется: >= 24.0.0
```

### 2. Установка зависимостей

```bash
# Перейдите в директорию проекта
cd dodep

# Установите Node.js зависимости
npm install

# Установите Rust зависимости
cd services/rust && cargo fetch && cd ../..

# Установите Go зависимости
cd services/go/auth && go mod download && cd ../../..
```

### 3. Настройка окружения

```bash
# Скопируйте .env.example в .env
cp .env.example .env

# Сгенерируйте случайные ключи (Linux/Mac)
# Для Windows используйте PowerShell:
# $bytes = New-Object byte[] 32; (New-Object Security.Cryptography.RNGCryptoServiceProvider).GetBytes($bytes); [Convert]::ToBase64String($bytes)

# Вставьте сгенерированные значения в .env:
# JWT_SECRET_KEY=<32 random bytes>
# JWT_REFRESH_SECRET=<32 random bytes>
# ENCRYPTION_KEY=<32 random bytes>
```

### 4. Запуск инфраструктуры

```bash
# Запустите Docker контейнеры (PostgreSQL, DragonflyDB, ClickHouse, Redpanda, MinIO)
make docker-up

# Проверьте статус
docker-compose -f infra/docker/docker-compose.dev.yml ps

# Должны быть запущены:
# ✅ opus-postgres
# ✅ opus-dragonfly
# ✅ opus-clickhouse
# ✅ opus-redpanda
# ✅ opus-minio
```

### 5. Запуск сервисов

```bash
# Вариант A: Запустить все сервисы сразу (требует много ресурсов)
npm run dev

# Вариант B: Запустить отдельные сервисы (рекомендуется)

# Terminal 1: Rust Betting Engine
cd services/rust/betting-engine && cargo run

# Terminal 2: Rust Wallet Core
cd services/rust/wallet-core && cargo run

# Terminal 3: Rust WebSocket Gateway
cd services/rust/websocket-gateway && cargo run

# Terminal 4: Go Auth Service
cd services/go/auth && go run main.go

# Terminal 5: Go Payment Service
cd services/go/payment && go run main.go

# Terminal 6: Python Fraud ML
cd services/python/fraud-ml && python main.py
```

### 6. Запуск Frontend приложений

```bash
# Terminal 7: Web (Next.js)
cd apps/web
npm run dev
# Доступно: http://localhost:3000

# Terminal 8: Admin Panel (React)
cd apps/admin
npm run dev
# Доступно: http://localhost:3001

# Terminal 9: Mobile (Flutter)
cd apps/mobile
flutter run
# iOS симулятор или Android эмулятор
```

---

## 📦 Детальная инструкция

### Шаг 1: Установка Node.js

**Windows:**
```powershell
# Скачайте установщик с https://nodejs.org/
# Выберите LTS версию (20.x)
# Установите с настройками по умолчанию
```

**macOS:**
```bash
brew install node@20
```

**Linux:**
```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
```

### Шаг 2: Установка Rust

**Windows:**
```powershell
# Скачайте rustup-init.exe с https://rustup.rs/
# Запустите и следуйте инструкциям
```

**macOS/Linux:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env
```

### Шаг 3: Установка Go

**Windows:**
```powershell
# Скачайте установщик с https://go.dev/dl/
# Установите с настройками по умолчанию
```

**macOS:**
```bash
brew install go@1.21
```

**Linux:**
```bash
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### Шаг 4: Установка Python

**Windows:**
```powershell
# Скачайте установщик с https://www.python.org/downloads/
# Выберите версию 3.11+
# ⚠️ Отметьте "Add Python to PATH" при установке
```

**macOS:**
```bash
brew install python@3.11
```

**Linux:**
```bash
sudo apt-get install python3.11 python3.11-venv python3-pip
```

### Шаг 5: Установка Docker

**Windows:**
```powershell
# Скачайте Docker Desktop с https://www.docker.com/products/docker-desktop/
# Установите и запустите
# Включите WSL 2 backend
```

**macOS:**
```bash
brew install --cask docker
# Запустите Docker.app
```

**Linux:**
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Перезалогиньтесь
```

### Шаг 6: Установка Flutter (для Mobile)

**Windows:**
```powershell
# Скачайте Flutter SDK с https://docs.flutter.dev/get-started/install/windows
# Распакуйте в C:\src\flutter
# Добавьте в PATH: C:\src\flutter\bin
flutter doctor
```

**macOS:**
```bash
brew install --cask flutter
flutter doctor
```

**Linux:**
```bash
sudo snap install flutter --classic
flutter doctor
```

---

## 🧪 Запуск тестов

### Unit тесты

```bash
# Rust тесты
cd services/rust/betting-engine && cargo test

# Go тесты
cd services/go/auth && go test ./...

# Python тесты
cd services/python/fraud-ml && pytest

# Frontend тесты
cd apps/web && npm run test
```

### Integration тесты

```bash
# Запустите инфраструктуру
make docker-up

# Rust integration тесты
cd services/rust/betting-engine && cargo test --test integration

# Go integration тесты
cd services/go/payment && go test -tags=integration ./...
```

### Load тесты (k6)

```bash
# Установите k6
# macOS
brew install k6

# Windows
winget install k6

# Linux
sudo apt-get install k6

# Запустите тест на 100 пользователей
k6 run --vus 100 --duration 5m tools/testing/k6/scenarios/10m-users.js

# Запустите тест на 1000 пользователей
k6 run --vus 1000 --duration 10m tools/testing/k6/scenarios/10m-users.js
```

---

## 🔧 Решение проблем

### Проблема: Docker контейнеры не запускаются

**Решение:**
```bash
# Остановите все контейнеры
docker-compose -f infra/docker/docker-compose.dev.yml down

# Очистите volumes
docker-compose -f infra/docker/docker-compose.dev.yml down -v

# Запустите заново
docker-compose -f infra/docker/docker-compose.dev.yml up -d

# Проверьте логи
docker-compose -f infra/docker/docker-compose.dev.yml logs
```

### Проблема: Rust зависимости не устанавливаются

**Решение:**
```bash
# Очистите cargo cache
cargo clean

# Обновите toolchain
rustup update

# Попробуйте снова
cd services/rust && cargo fetch
```

### Проблема: Go модули не загружаются

**Решение:**
```bash
# Очистите Go cache
go clean -modcache

# Обновите модули
cd services/go/auth && go mod tidy

# Попробуйте снова
go mod download
```

### Проблема: Python зависимости конфликтуют

**Решение:**
```bash
# Создайте виртуальное окружение
cd services/python/fraud-ml
python -m venv venv

# Активируйте
# Windows
venv\Scripts\activate
# macOS/Linux
source venv/bin/activate

# Установите зависимости
pip install -r requirements.txt
```

### Проблема: Next.js не запускается

**Решение:**
```bash
# Очистите .next cache
cd apps/web
rm -rf .next
rm -rf node_modules/.cache

# Переустановите зависимости
npm install

# Запустите снова
npm run dev
```

---

## 📊 Мониторинг

### Grafana Dashboards

```bash
# VictoriaMetrics (метрики)
http://localhost:8428

# Grafana (дашборды)
http://localhost:3000
# Логин: admin
# Пароль: admin

# Jaeger (tracing)
http://localhost:16686

# MinIO Console (S3)
http://localhost:9001
# Логин: minioadmin
# Пароль: minioadmin
```

### Проверка здоровья сервисов

```bash
# Betting Engine
curl http://localhost:8080/health

# Wallet Core
curl http://localhost:8081/health

# WebSocket Gateway
curl http://localhost:8082/health

# Auth Service
curl http://localhost:8083/health

# Payment Service
curl http://localhost:8084/health
```

---

## 🎮 Демонстрация функционала

### 1. Регистрация пользователя

```bash
curl -X POST http://localhost:8083/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "TestPassword123!",
    "currency": "USD"
  }'
```

### 2. Вход

```bash
curl -X POST http://localhost:8083/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "TestPassword123!"
  }'
```

### 3. Создание кошелька

```bash
curl -X POST http://localhost:8081/api/v1/wallet \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_TOKEN>" \
  -d '{
    "user_id": 1,
    "currency": "USD"
  }'
```

### 4. Депозит

```bash
curl -X POST http://localhost:8084/api/v1/payment/deposit \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_TOKEN>" \
  -d '{
    "user_id": 1,
    "amount": 100.00,
    "currency": "USD",
    "method": "card"
  }'
```

### 5. Размещение ставки

```bash
curl -X POST http://localhost:8080/api/v1/bets/place \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <YOUR_TOKEN>" \
  -d '{
    "user_id": 1,
    "stake": 10.00,
    "odds": 2.5,
    "selections": [
      {
        "event_id": 1,
        "selection_id": 1
      }
    ]
  }'
```

---

## 🛑 Остановка проекта

```bash
# Остановить Docker контейнеры
make docker-down

# Остановить все процессы
# Нажмите Ctrl+C во всех терминалах

# Очистить build артефакты
make clean
```

---

## 📞 Поддержка

Если возникли проблемы:

1. Проверьте логи Docker контейнеров
2. Проверьте логи сервисов
3. Убедитесь, что все порты свободны
4. Перезапустите инфраструктуру

**Контакты:**
- GitHub Issues: https://github.com/husan0209/dodep/issues
- Документация: https://github.com/husan0209/dodep/tree/main/docs

---

**Made with ❤️ by Opus Casino Team**
