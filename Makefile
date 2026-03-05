MIGRATE=go run github.com/golang-migrate/migrate/v4/cmd/migrate

.PHONY: up down run migrate-up test test-integration coverage-critical

up:
	docker compose -f deploy/docker-compose.yml up --build

down:
	docker compose -f deploy/docker-compose.yml down -v

run:
	go run ./cmd/api

migrate-up:
	$(MIGRATE) -path migrations -database "$$MYSQL_DSN" up

test:
	go test ./... -count=1

test-integration:
	go test -mod=mod -tags=integration ./tests/integration -count=1 -v

coverage-critical:
	go test ./internal/service -coverprofile=coverage-service.out -count=1
	go tool cover -func=coverage-service.out | tail -n 1
