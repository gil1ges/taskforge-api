# TaskForge API

REST API для управления командами, задачами, инвайтами и отчетами.

## Быстрый старт

Скопируйте этот репозиторий и перейдите в директорию проекта:

```bash
git clone https://github.com/gil1ges/taskforge-api.git
cd taskforge-api
```

## Требования

- Go `1.24+`
- Docker + Docker Compose
- GNU Make

## Настройка `.env`

Создайте файл `.env` в корне проекта:

```bash
cat > .env <<'ENV'
HTTP_PORT=8080

MYSQL_DSN=taskforge:taskforge@tcp(mysql:3306)/taskforge?parseTime=true&multiStatements=true

REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_TASKS_TTL_MIN=5

JWT_SECRET=dev-secret-change-me
RATE_LIMIT_PER_MIN=100

INVITE_TTL_MIN=1440
INVITE_NOTIFY_URL=
ENV
```

`INVITE_NOTIFY_URL` опционален: если пустой, внешние уведомления об инвайтах отключены.

## Запуск (Docker, рекомендовано)

Поднимет MySQL, Redis, применит миграции и запустит API:

```bash
make up
```

Проверка:

```bash
curl -i http://localhost:8080/health
```

Остановка и очистка volumes:

```bash
make down
```

## Запуск локально (API на хосте)

1. Поднимите только MySQL и Redis:

```bash
docker compose -f deploy/docker-compose.yml up -d mysql redis
```

2. Примените миграции (DSN для хоста):

```bash
MYSQL_DSN='taskforge:taskforge@tcp(localhost:3306)/taskforge?parseTime=true&multiStatements=true' make migrate-up
```

3. Экспортируйте переменные окружения и запустите API:

```bash
export HTTP_PORT=8080
export MYSQL_DSN='taskforge:taskforge@tcp(localhost:3306)/taskforge?parseTime=true&multiStatements=true'
export REDIS_ADDR='localhost:6379'
export REDIS_PASSWORD=''
export REDIS_DB=0
export REDIS_TASKS_TTL_MIN=5
export JWT_SECRET='dev-secret-change-me'
export RATE_LIMIT_PER_MIN=100
export INVITE_TTL_MIN=1440
export INVITE_NOTIFY_URL=''

make run
```

## Миграции

Применить миграции вручную:

```bash
MYSQL_DSN='taskforge:taskforge@tcp(localhost:3306)/taskforge?parseTime=true&multiStatements=true' make migrate-up
```

## API маршруты

Базовый префикс для бизнес-ручек: `/api/v1`  
Сервисные ручки доступны без префикса: `/health`, `/metrics`

Публичные:

- `POST /register`
- `POST /login`
- `GET /health`
- `GET /metrics`

С JWT (`Authorization: Bearer <token>`):

- `POST /teams`
- `GET /teams`
- `POST /teams/{id}/invite`
- `POST /teams/{id}/accept`
- `POST /tasks`
- `GET /tasks`
- `PUT /tasks/{id}`
- `GET /tasks/{id}/history`
- `GET /reports/team-summaries`
- `GET /reports/top-creators`
- `GET /reports/invalid-assignees`

## Быстрые примеры запросов

Регистрация:

```bash
curl -s -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123"}'
```

Логин и получение токена:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123"}' | jq -r '.token')

echo "$TOKEN"
```

Для команды выше нужен `jq`.

Создание команды:

```bash
curl -s -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"backend"}'
```

Создание задачи:

```bash
curl -s -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"team_id":1,"title":"Сделать API","status":"todo"}'
```

## Метрики Prometheus

Метрики доступны по адресу:

```bash
curl -s http://localhost:8080/metrics
```

Примеры PromQL:

```promql
sum(rate(http_requests_total[1m]))
```

```promql
sum(rate(http_requests_total{path="/api/v1/login"}[5m])) by (status)
```

## Тесты

Unit/обычные тесты:

```bash
make test
```

Интеграционные тесты (реальные MySQL + Redis через testcontainers):

```bash
make test-integration
```

Для интеграционных тестов должен быть запущен Docker daemon.

Покрытие критичного слоя (`internal/service`):

```bash
make coverage-critical
```

## Структура проекта

```text
cmd/api            # входная точка приложения
internal/app       # сборка зависимостей и роутера
internal/config    # конфиг из env
internal/domain    # доменные модели/ошибки
internal/http      # хендлеры и middleware
internal/repo      # MySQL/Redis репозитории
internal/service   # бизнес-логика
migrations         # SQL миграции
tests/integration  # интеграционные тесты
```
