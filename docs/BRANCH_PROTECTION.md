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
1. Architecture guards  2. CI - Go               3. CI - Rust
   (G1–G4: деньги,          (Lint, Test -race,      (fmt, clippy,
   логи, секреты,           gosec, govulncheck      nextest,
   @ts-ignore)              для 7 Go-сервисов)      cargo-audit,
     │                       │                       cargo-deny)
     ▼                       ▼                       ▼
4. CI - Python            5. CI - Frontend        6. Security Scan
   (ruff, black, mypy,      / Admin Panel           (Trivy, Semgrep,
   pytest, safety,          (ESLint, tsc,           CodeQL, secret
   bandit)                  build, vitest)          scan, checkov)
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
| `Architecture guards` | `.github/workflows/architecture-guards.yml` | Архитектурные правила гемблинга (запрет float для денег, запрет `fmt.Println`, запрет приватных ключей, запрет `@ts-ignore`) |
| `Lint` | `.github/workflows/ci-go.yml` | `golangci-lint` для `auth`, `user`, `payment`, `bonus`, `casino`, `notification`, `kyc` |
| `Test` | `.github/workflows/ci-go.yml` | `go test -race` + покрытие для всех семи Go-сервисов |
| `Security Scan` | `.github/workflows/ci-go.yml` | `gosec` и `govulncheck` для всех семи Go-сервисов |
| `Lint` | `.github/workflows/ci-rust.yml` | `cargo fmt` и `cargo clippy -D warnings` для `betting-engine`, `wallet-core`, `websocket-gateway` |
| `Test` | `.github/workflows/ci-rust.yml` | `cargo nextest` для тех же трёх Rust-сервисов |
| `Security Scan` | `.github/workflows/ci-rust.yml` | `cargo audit --deny warnings` и `cargo deny` |
| `Lint` / `Test` / `Security Scan` | `.github/workflows/ci-python.yml` | ruff, black, mypy, pytest, safety, bandit для `fraud-ml` и `analytics` |
| `Lint` / `Type Check` / `Test` | `.github/workflows/ci-frontend.yml` | ESLint, `tsc --noEmit`, vitest для Next.js Web Platform |
| `lint-and-build` | `.github/workflows/ci-admin-panel.yml` | ESLint, `tsc --noEmit` и production-сборка Admin Panel |
| `Security Summary` | `.github/workflows/security-scan.yml` | Итог Trivy / Semgrep / CodeQL / secret scan / checkov |

> **Почему нет `ci.yaml` и `deploy.yml`.** Оба файла были удалены как
> дублирующие: каждый проверяемый стек уже покрыт выделенным workflow
> (`ci-rust`, `ci-go`, `ci-python`, `ci-frontend`, `ci-admin-panel`,
> `ci-nextjs-web`, `security-scan`, `security-audit`, `cd-*`), причём
> строже — с `cargo deny`, `black`, `mypy`, `vitest`, `playwright`,
> `dependency-review` и `checkov`. `ci.yaml` дополнительно был нерабочим:
> он собирал Go 1.21 при `go 1.24+` в `go.mod` и не генерировал protobuf-стабы,
> поэтому ни один Go-сервис в нём не мог пройти типизацию.

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
   - Нажмите **Add checks** и добавьте чеки из таблицы раздела 2
8. Прокрутите в самый низ и нажмите **Create** (или **Save changes**).

---

## 4. Что контролирует Architecture Guards (`architecture-guards.yml`)

1. **G1: Запрет `float32`/`float64`/`f32`/`f64` для денег**: балансы и ставки обрабатываются строго в целых единицах (cents / satoshi) или Decimal.
2. **G2: Запрет неструктурированного логирования** `fmt.Println` / `println!` в Go- и Rust-сервисах — только структурированный логгер (`zap`, `tracing`).
3. **G3: Запрет приватных ключей** в поставляемом коде. Документация (`*.md`) исключена: Kubernetes/SSH-гайды обязаны показывать формат ключа и печатают усечённый плейсхолдер вида `b3BlbnNza...`.
4. **G4: Запрет `@ts-ignore`** во фронтенде — используется строгая типизация.
