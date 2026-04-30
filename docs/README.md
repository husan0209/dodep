# Документация Opus Casino

## Содержание

1. [Архитектура](./architecture.md)
2. [API Документация](./api.md)
3. [Развертывание](./deployment.md)
4. [Мониторинг](./monitoring.md)
5. [Безопасность](./security.md)

## Быстрый старт для разработчиков

### Требования

- Node.js >= 20
- Rust >= 1.75
- Go >= 1.21
- Python >= 3.11
- Docker >= 24
- kubectl >= 1.28

### Установка

```bash
# Установить зависимости
npm install

# Запустить все сервисы (dev режим)
npm run dev

# Запустить конкретный сервис
nx serve betting-engine
nx serve auth
nx serve web
```

### Структура проекта

```
opus-casino/
├── apps/           # Приложения (web, mobile, admin)
├── services/       # Микросервисы (rust, go, python)
├── libs/           # Общие библиотеки
├── infra/          # Инфраструктура (k8s, terraform, docker)
├── tools/          # Инструменты разработки
└── docs/           # Документация
```

### Тестирование

```bash
# Запустить все тесты
npm run test

# Запустить тесты для конкретного сервиса
nx test betting-engine
nx test auth
```

### Линтинг

```bash
# Проверка всех проектов
npm run lint

# Форматирование кода
npm run format
```
