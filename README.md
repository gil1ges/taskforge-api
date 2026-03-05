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

## Соответствие ТЗ (по пунктам)

Команды запускать из корня проекта `taskforge-api`.
Для ручных `curl`-проверок предполагается, что API запущен (`make up`) и доступен на `http://localhost:8080`.

### 1. Регистрация и аутентификация

Реализовано:
- `POST /api/v1/register`
- `POST /api/v1/login`

Проверка:

```bash
go test -mod=mod -tags=integration ./tests/integration -run TestAuthRegisterAndLogin -count=1 -v
go test -mod=mod -tags=integration ./tests/integration -run TestAuthLoginInvalidPassword -count=1 -v
```

Ручная проверка (`curl`):

```bash
curl -i -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"manual-user@example.com","password":"password123"}'

curl -i -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"manual-user@example.com","password":"password123"}'
```

### 2. Управление командами

Реализовано:
- `POST /api/v1/teams`
- `GET /api/v1/teams`
- `POST /api/v1/teams/{id}/invite`

Проверка:

```bash
go test -mod=mod -tags=integration ./tests/integration -run TestTeamsRequiresAuth -count=1 -v
go test -mod=mod -tags=integration ./tests/integration -run TestTeamsCreateAndList -count=1 -v
```

Ручная проверка (`curl`):

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"manual-user@example.com","password":"password123"}' | jq -r '.token')

curl -i -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"manual-team"}'

curl -i -X GET http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN"
```

### 3. Управление задачами

Реализовано:
- `POST /api/v1/tasks`
- `GET /api/v1/tasks` (фильтры + пагинация)
- `PUT /api/v1/tasks/{id}`
- `GET /api/v1/tasks/{id}/history`

Проверка:

```bash
go test -mod=mod -tags=integration ./tests/integration -run TestTasksCreateListUpdateHistory -count=1 -v
```

Ручная проверка (`curl`):

```bash
TASK_ID=$(curl -s -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"team_id":1,"title":"manual-task","status":"todo"}' | jq -r '.task_id')

curl -i -X GET "http://localhost:8080/api/v1/tasks?team_id=1&page=1&size=20" \
  -H "Authorization: Bearer $TOKEN"

curl -i -X PUT "http://localhost:8080/api/v1/tasks/$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}'

curl -i -X GET "http://localhost:8080/api/v1/tasks/$TASK_ID/history" \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Сложные SQL-запросы

Реализовано (эндпоинты):
- `GET /api/v1/reports/team-summaries` (JOIN + агрегация)
- `GET /api/v1/reports/top-creators` (оконная функция, топ-3)
- `GET /api/v1/reports/invalid-assignees` (проверка целостности)

Проверка:

```bash
go test -mod=mod -tags=integration ./tests/integration -run TestReportsEndpoints -count=1 -v
```

Ручная проверка (`curl`):

```bash
curl -i -X GET http://localhost:8080/api/v1/reports/team-summaries \
  -H "Authorization: Bearer $TOKEN"

curl -i -X GET http://localhost:8080/api/v1/reports/top-creators \
  -H "Authorization: Bearer $TOKEN"

curl -i -X GET http://localhost:8080/api/v1/reports/invalid-assignees \
  -H "Authorization: Bearer $TOKEN"
```

### 5. Оптимизация

Кеширование в Redis (TTL 5 минут):

```bash
rg -n "REDIS_TASKS_TTL_MIN|Set\\(ctx, key, b, c.ttl\\)" .env internal/config/config.go internal/repo/redis/cache.go
go test -mod=mod -tags=integration ./tests/integration -run TestTasksListWorksWhenRedisUnavailable -count=1 -v
```

Индексы в MySQL:

```bash
rg -n "KEY " migrations/0001_init.up.sql migrations/0002_invites.up.sql
```

Connection pooling для БД:

```bash
rg -n "SetMaxOpenConns|SetMaxIdleConns|SetConnMaxLifetime" internal/repo/mysql/db.go
```

Пагинация на уровне БД:

```bash
rg -n "LIMIT \\? OFFSET \\?|Page|Size|offset" internal/repo/mysql/tasks.go internal/http/handler/tasks.go
```

### 6. Тестирование

Unit-тесты на бизнес-логику:

```bash
make test
```

Интеграционные тесты с MySQL (testcontainers):

```bash
make test-integration
```

Минимум 85% покрытия по критическим методам:

```bash
make coverage-critical
```

### 7. Дополнительные пункты

Circuit breaker для внешнего сервиса приглашений:

```bash
go test ./internal/service -run TestCircuitBreakerNotifier -count=1 -v
```

Rate limiting (100 запросов/мин на пользователя):

```bash
go test -mod=mod -tags=integration ./tests/integration -run TestRateLimitReturns429 -count=1 -v
```

Graceful shutdown:

```bash
rg -n "signal.Notify|Shutdown|context.WithTimeout" cmd/api/main.go internal/app/app.go
```

Prometheus метрики (запросы/ошибки/время ответа):

```bash
curl -s http://localhost:8080/metrics | rg "http_requests_total|http_errors_total|http_request_latency_seconds"
```

Конфигурация через ENV:

```bash
rg -n "os.Getenv|getInt|getStr|Load\\(" internal/config/config.go
```

## Структура проекта

```text
cmd/api            # входная точка приложения
internal/app       # сборка зависимостей 
internal/config    # конфиг из env
internal/domain    # доменные модели/ошибки
internal/http      # хендлеры и middleware
internal/repo      # MySQL/Redis репозитории
internal/service   # бизнес-логика
migrations         # SQL миграции
tests/integration  # интеграционные тесты
```
