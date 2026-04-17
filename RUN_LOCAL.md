# 🚀 Инструкция по локальному запуску Opus Casino

## ⚡ БЫСТРЫЙ СТАРТ

### Требование №1: Docker Desktop

**Без Docker Desktop проект не запустится!**

#### Запуск Docker Desktop:
1. Нажмите `Win + S` (поиск Windows)
2. Введите "Docker Desktop"
3. Запустите приложение
4. **Дождитесь** зелёного индикатора "Docker Desktop is running"
5. Проверьте: откройте терминал и выполните `docker ps`

Если `docker ps` работает — переходите дальше! ✅

---

### Шаг 1: Запуск инфраструктуры (БД, кэш, брокер)

```bash
cd "D:\проекты\opus casino"

# Запустить все контейнеры
docker-compose -f infra/docker/docker-compose.dev.yml up -d

# Проверить статус (должны быть 5 контейнеров)
docker-compose ps

# Ожидаемый результат:
# ✅ opus-postgres      (PostgreSQL 16)
# ✅ opus-dragonfly     (DragonflyDB/Redis)
# ✅ opus-clickhouse    (ClickHouse)
# ✅ opus-redpanda      (Redpanda/Kafka)
# ✅ opus-minio         (MinIO/S3)
```

**Проверка портов:**
- PostgreSQL: `localhost:5432`
- DragonflyDB: `localhost:6379`
- ClickHouse: `localhost:8123`
- Redpanda: `localhost:9092`
- MinIO Console: `localhost:9001`

---

### Шаг 2: Запуск Backend сервисов

**⚠️ ТРЕБУЕТСЯ установка Rust, Go, Python**

#### Вариант A: Запустить все сервисы (если установлены все зависимости)

```bash
# Terminal 1: Rust Betting Engine
cd services/rust/betting-engine
cargo run

# Terminal 2: Rust Wallet Core
cd services/rust/wallet-core
cargo run

# Terminal 3: Rust WebSocket Gateway
cd services/rust/websocket-gateway
cargo run

# Terminal 4: Go Auth Service
cd services/go/auth
go run main.go

# Terminal 5: Go Payment Service
cd services/go/payment
go run main.go
```

#### Вариант B: Запустить только один сервис для теста

```bash
# Тест Go Auth Service
cd services/go/auth
go run main.go

# Проверка: curl http://localhost:8083/health
```

---

### Шаг 3: Запуск Frontend

#### Web (Next.js)

```bash
cd apps/web
npm install
npm run dev

# Откройте: http://localhost:3000
```

#### Admin Panel (React)

```bash
cd apps/admin
npm install
npm run dev

# Откройте: http://localhost:3001
```

#### Mobile (Flutter)

```bash
cd apps/mobile
flutter pub get
flutter run

# iOS симулятор или Android эмулятор
```

---

## 🔧 Установка зависимостей

### Node.js (ОБЯЗАТЕЛЬНО)

```bash
# Проверка
node --version  # Должно быть: v20.x или v22.x
npm --version   # Должно быть: 10.x

# Если не установлен:
# Скачайте с https://nodejs.org/ (LTS версия)
```

### Rust (для backend)

```bash
# Проверка
rustc --version  # Должно быть: 1.75+

# Если не установлен:
# Скачайте с https://rustup.rs/
```

### Go (для backend)

```bash
# Проверка
go version  # Должно быть: 1.21+

# Если не установлен:
# Скачайте с https://go.dev/dl/
```

### Python (для ML)

```bash
# Проверка
python --version  # Должно быть: 3.11+

# Если не установлен:
# Скачайте с https://www.python.org/downloads/
```

### Docker Desktop (ОБЯЗАТЕЛЬНО)

```bash
# Проверка
docker --version  # Должно быть: 24.x+
docker ps         # Должен показать список контейнеров

# Если не установлен:
# Скачайте с https://www.docker.com/products/docker-desktop/
```

---

## 🎮 ЧТО МОЖНО ЗАПУСТИТЬ БЕЗ DOCKER

### Только Frontend (демо режим)

```bash
# Web
cd apps/web
npm install
npm run dev
# http://localhost:3000

# Admin
cd apps/admin
npm install
npm run dev
# http://localhost:3001
```

**⚠️ Без Docker frontend не сможет подключиться к API!**

---

## 🎮 ЧТО МОЖНО ЗАПУСТИТЬ БЕЗ RUST/GO

### Только Инфраструктура

```bash
# Запустить БД, кэш, брокер
docker-compose -f infra/docker/docker-compose.dev.yml up -d

# Проверить
docker-compose ps
```

### Подключение к БД

```bash
# PostgreSQL
docker exec -it opus-postgres psql -U postgres -d opus_casino

# DragonflyDB (Redis)
docker exec -it opus-dragonfly redis-cli

# ClickHouse
docker exec -it opus-clickhouse clickhouse-client -u default -d opus_casino
```

---

## 🛠 РЕШЕНИЕ ПРОБЛЕМ

### Проблема: Docker Desktop не запускается

**Решение:**
1. Убедитесь, что включена виртуализация в BIOS
2. Включите Hyper-V (Windows Pro):
   ```powershell
   Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
   ```
3. Перезагрузите компьютер
4. Запустите Docker Desktop от имени администратора

### Проблема: `docker ps` не работает

**Решение:**
1. Проверьте, что Docker Desktop запущен (зелёный индикатор)
2. Перезапустите Docker Desktop
3. Проверьте службу Docker:
   ```powershell
   Get-Service Docker
   ```

### Проблема: npm install выдаёт ошибки

**Решение:**
```bash
# Очистите cache
npm cache clean --force

# Удалите node_modules
rm -rf node_modules
rm package-lock.json

# Установите заново
npm install
```

### Проблема: Порты заняты

**Решение:**
```bash
# Остановите контейнеры
docker-compose down

# Запустите заново
docker-compose up -d

# Или измените порты в docker-compose.dev.yml
```

---

## ✅ МИНИМАЛЬНЫЙ ЗАПУСК

**Самый простой способ увидеть проект в действии:**

1. ✅ **Установите Docker Desktop**
2. ✅ **Запустите Docker Desktop**
3. ✅ **Выполните:**
   ```bash
   cd "D:\проекты\opus casino"
   docker-compose -f infra/docker/docker-compose.dev.yml up -d
   docker-compose ps
   ```
4. ✅ **Проверьте:**
   - PostgreSQL: `localhost:5432`
   - MinIO Console: `http://localhost:9001` (admin: minioadmin)

5. ✅ **Запустите Web Frontend:**
   ```bash
   cd apps/web
   npm install
   npm run dev
   ```
6. ✅ **Откройте:** `http://localhost:3000`

---

## 📞 ПОДДЕРЖКА

Если возникли проблемы:

1. Проверьте логи Docker:
   ```bash
   docker-compose logs
   ```

2. Проверьте логи конкретного сервиса:
   ```bash
   docker-compose logs postgres
   docker-compose logs dragonfly
   ```

3. Перезапустите всё:
   ```bash
   docker-compose down -v
   docker-compose up -d
   ```

---

**Made with ❤️ by Opus Casino Team**
