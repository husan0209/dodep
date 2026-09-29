# Branch Protection & CI Status Checks — Opus Casino (`dodep`)

> **Репозиторий:** `https://github.com/husan0209/dodep`  
> **Целевые ветки:** `main` (продакшен), `develop` (интеграционная ветка разработки)  
> **Настройка:** GitHub Rulesets (**Settings → Rules → Rulesets**)

---

## 1. Схема обязательных проверок (Status Checks)

При открытии любого Pull Request в ветку `main` или `develop` GitHub Actions запускает пайплайны. Ни один разработчик или AI-агент не может смержить PR, пока все назначенные чеки не станут зелёными ✅.

```
                  [ Pull Request в dodep ]
                             │
     ┌───────────────────────┼───────────────────────┐
     ▼                       ▼                       ▼
1. rust-build           2. go-build             3. python-build
   (Rust сервисы:          (Go сервисы:            (ML/Fraud/Analytics:
   betting, wallet,        auth, payment,          ruff, mypy,
   gateway)                casino, kyc)            pytest)
     │                       │                       │
     ▼                       ▼                       ▼
4. frontend-build       5. security-scan        6. Architecture guards
   (Next.js web +          (Trivy, Semgrep,        (Grep-проверки денег,
   Admin panel)            Gitleaks секреты)       безопасности, правил)
     │                       │                       │
     └───────────────────────┼───────────────────────┘
                             ▼
                    [ Все проверки прошли? ]
                       /            \
                    ДА               НЕТ
                    ▼                 ▼
             [ Кнопка Merge ]   [ Кнопка Merge ]
              РАЗБЛОКИРОВАНА     ЗАБЛОКИРОВАНА ❌
```

---

## 2. Имена Status Checks для добавления в GitHub

В GitHub Ruleset в поле **«Status checks that are required»** нужно найти и добавить следующие чеки:

| Имя чека в GitHub | Workflow-источник | Что проверяет |
|---|---|---|
| `rust-build` | `.github/workflows/ci.yaml` | Сборка, clippy и cargo test для Rust-микросервисов (`betting-engine`, `wallet-core`, `websocket-gateway`) |
| `go-build` | `.github/workflows/ci.yaml` | Сборка, go vet, race detector и тесты для Go (`auth`, `payment`, `casino`, `notification`, `kyc`) |
| `python-build` | `.github/workflows/ci.yaml` | Lint (ruff), проверка типов (mypy) и pytest для Python (`fraud-ml`, `analytics`) |
| `frontend-build` | `.github/workflows/ci.yaml` | Сборка, ESLint и тесты для Next.js Web и Admin Panel |
| `security-scan` | `.github/workflows/ci.yaml` | Сканирование уязвимостей Trivy + Semgrep SAST |
| `Architecture guards` | `.github/workflows/architecture-guards.yml` | Архитектурные правила гемблинга (Serializable в кошельке, запрет float для денег, запрет секретов) |

---

## 3. Пошаговая инструкция по настройке в личном кабинете GitHub

1. Откройте в браузере репозиторий: **`https://github.com/husan0209/dodep`**
2. Перейдите в **Settings** (Настройки) → слева раздел **Rules** → **Rulesets**
3. Нажмите **New ruleset → New branch ruleset**
4. Заполните поля:
   - **Ruleset Name:** `main-protection`
   - **Enforcement status:** `Active` (зелёный переключатель)
5. **Target branches** (Целевые ветки):
   - Нажмите **Add target → Include by pattern**
   - Добавьте: `main`
   - *(Опционально)* Добавьте вторую ветку: `develop`
6. **Branch rules** (Правила):
   - [x] **Restrict deletions** — включить (запрет удаления ветки)
   - [x] **Block force pushes** — включить (запрет `git push --force`)
   - [x] **Require linear history** — включить (чистая история коммитов)
   - [x] **Require a pull request before merging** — включить:
     - **Required approvals:** `1` (или `0` при работе в одиночку)
     - [x] **Dismiss stale pull request approvals when new commits are pushed**
7. **Require status checks to pass** (Обязательные проверки):
   - Включите галочку [x]
   - Включите [x] **Require branches to be up to date before merging**
   - Нажмите **Add checks** и через поиск добавьте 6 чеков:
     1. `rust-build`
     2. `go-build`
     3. `python-build`
     4. `frontend-build`
     5. `security-scan`
     6. `Architecture guards`
8. Прокрутите в самый низ и нажмите **Create** (или **Save changes**).

---

## 4. Что контролирует Architecture Guards (`architecture-guards.yml`)

1. **G1: Запрет `float32`/`float64`/`f32`/`f64` для денег**: балансы и ставки обрабатываются строго в целых единицах (cents / satoshi) или Decimal.
2. **G2: Запрет хаков `@ts-ignore` / `unsafe` без объяснений**.
3. **G3: Запрет отладочного вывода `println!` / `fmt.Println` в продакшен-сервисах** (только структурированный логгер).
4. **G4: Контроль изоляции транзакций кошелька** (`Serializable` / Row Lock).
5. **G5: Запрет секретов и приватных ключей в репозитории**.
