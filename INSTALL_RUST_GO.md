# 📥 Инструкция по установке Rust и Go на Windows

## 🔧 УСТАНОВКА RUST

### Способ 1: Через браузер (рекомендуется)

1. **Откройте ссылку:** https://static.rust-lang.org/rustup/dist/x86_64-pc-windows-msvc/rustup-init.exe

2. **Скачайте файл** `rustup-init.exe`

3. **Запустите установщик**
   - Дважды кликните на файл
   - Нажмите "Yes" для подтверждения
   - Выберите опцию по умолчанию: `1) Proceed with installation`
   - Дождитесь завершения установки

4. **Проверьте установку**
   ```bash
   # Откройте НОВЫЙ терминал (PowerShell или CMD)
   rustc --version
   cargo --version
   ```
   
   **Ожидаемый результат:**
   ```
   rustc 1.75.x (или новее)
   cargo 1.75.x (или новее)
   ```

### Способ 2: Через PowerShell (если работает)

```powershell
# Откройте PowerShell от имени администратора
Invoke-WebRequest -Uri https://static.rust-lang.org/rustup/dist/x86_64-pc-windows-msvc/rustup-init.exe -OutFile $env:TEMP\rustup-init.exe
Start-Process "$env:TEMP\rustup-init.exe" -Wait -ArgumentList "-y"
```

---

## 🐹 УСТАНОВКА GO

### Способ 1: Через браузер (рекомендуется)

1. **Откройте ссылку:** https://go.dev/dl/

2. **Скачайте установщик для Windows**
   - Файл: `go1.21.x.windows-amd64.msi`

3. **Запустите установщик**
   - Дважды кликните на файл
   - Нажмите "Next"
   - Оставьте путь по умолчанию: `C:\Program Files\Go`
   - Нажмите "Install"
   - Дождитесь завершения

4. **Проверьте установку**
   ```bash
   # Откройте НОВЫЙ терминал
   go version
   ```
   
   **Ожидаемый результат:**
   ```
   go version go1.21.x windows/amd64
   ```

### Способ 2: Через PowerShell (если работает)

```powershell
# Откройте PowerShell от имени администратора
$url = "https://go.dev/dl/go1.21.5.windows-amd64.msi"
$output = "$env:TEMP\go.msi"
Invoke-WebRequest -Uri $url -OutFile $output
Start-Process "msiexec.exe" -Wait -ArgumentList "/i $output /quiet"
```

---

## ✅ ПРОВЕРКА УСТАНОВКИ

После установки обоих инструментов выполните:

```bash
# Откройте НОВЫЙ терминал (PowerShell или CMD)

# Проверка Rust
rustc --version
cargo --version
rustup --version

# Проверка Go
go version
go env

# Проверка Python
python --version
pip --version
```

**Все команды должны вернуть версии!**

---

## 🚀 ЗАПУСК BACKEND СЕРВИСОВ ПОСЛЕ УСТАНОВКИ

### Terminal 1: Rust Betting Engine

```bash
cd "D:\проекты\opus casino\services\rust\betting-engine"
cargo run
```

**Ожидаемый результат:**
```
Starting Betting Engine on http://localhost:8080
```

### Terminal 2: Rust Wallet Core

```bash
cd "D:\проекты\opus casino\services\rust\wallet-core"
cargo run
```

**Ожидаемый результат:**
```
Starting Wallet Core on http://localhost:8081
```

### Terminal 3: Rust WebSocket Gateway

```bash
cd "D:\проекты\opus casino\services\rust\websocket-gateway"
cargo run
```

**Ожидаемый результат:**
```
Starting WebSocket Gateway on ws://localhost:8082
```

### Terminal 4: Go Auth Service

```bash
cd "D:\проекты\opus casino\services\go\auth"
go run main.go
```

**Ожидаемый результат:**
```
Starting Auth Service on http://localhost:8083
```

### Terminal 5: Go Payment Service

```bash
cd "D:\проекты\opus casino\services\go\payment"
go run main.go
```

**Ожидаемый результат:**
```
Starting Payment Service on http://localhost:8084
```

---

## 🛠 РЕШЕНИЕ ПРОБЛЕМ

### Проблема: rustc не найден после установки

**Решение:**
1. Закройте все терминалы
2. Откройте НОВЫЙ терминал
3. Проверьте: `rustc --version`

Если всё ещё не работает, добавьте в PATH вручную:
```powershell
# PowerShell
$env:Path += ";$env:USERPROFILE\.cargo\bin"
```

### Проблема: go не найден после установки

**Решение:**
1. Закройте все терминалы
2. Откройте НОВЫЙ терминал
3. Проверьте: `go version`

Если всё ещё не работает, добавьте в PATH вручную:
```powershell
# PowerShell
$env:Path += ";C:\Program Files\Go\bin"
```

### Проблема: cargo build выдаёт ошибки

**Решение:**
```bash
# Очистите cache
cargo clean

# Обновите toolchain
rustup update

# Попробуйте снова
cargo build
```

### Проблема: go mod download выдаёт ошибки

**Решение:**
```bash
# Очистите Go cache
go clean -modcache

# Обновите модули
go mod tidy

# Попробуйте снова
go mod download
```

---

## 📞 ПОДДЕРЖКА

Если возникли проблемы:

1. **Перезагрузите компьютер** после установки
2. **Откройте НОВЫЙ терминал** (старые не увидят новые переменные окружения)
3. **Проверьте версии** всех инструментов
4. **Посмотрите логи** ошибок

---

**После успешной установки переходите к запуску backend сервисов!** 🚀

Made with ❤️ by Opus Casino Team
