.PHONY: help build test test-race test-cover test-integration fmt vet lint check helm-lint docker-lint check-goose-version check-all clean migrate-up migrate-down migrate-status migrate-create helm-prepare

APP_NAME := alertmanager-webhook-relay
RUN := docker compose --profile tools run --rm dev
GOOSE := docker compose --profile db run --rm migrate

## Справка

help: ## Показать доступные цели
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk -F ':.*?## ' '{printf "  %-14s %s\n", $$1, $$2}'

## Сборка

build: ## Собрать бинарник
	$(RUN) go build -buildvcs=false -o $(APP_NAME) .

## Тесты

test: ## Запустить тесты
	$(RUN) go test ./...

test-race: ## Запустить тесты с race detector
	$(RUN) go test -race ./...

test-cover: ## Запустить тесты с отчётом покрытия
	$(RUN) sh -c 'go test -cover -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html'

test-integration: ## Запустить интеграционные тесты (Mailpit)
	docker compose --profile test up -d --wait mailpit
	docker compose --profile tools --profile test run --rm dev go test -tags=integration -count=1 -v ./...
	docker compose --profile test stop mailpit

## Качество кода

fmt: ## Отформатировать код
	$(RUN) gofmt -w .

vet: ## Запустить go vet
	$(RUN) go vet ./...

lint: ## Запустить линтер
	$(RUN) golangci-lint run

check: fmt vet lint test-race ## Полная проверка: fmt + vet + lint + test-race

helm-lint: ## Проверить Helm chart
	docker compose --profile tools run --rm helm lint deploy/helm/alertmanager-webhook-relay

docker-lint: ## Проверить Dockerfile (hadolint)
	docker compose --profile tools run --rm hadolint hadolint Dockerfile

check-goose-version: ## Сверить версию goose в go.mod, Dockerfile и workflow
	@VERSION=$$(awk '/github.com\/pressly\/goose\/v3 / {print $$2}' go.mod) ; \
	if [ -z "$$VERSION" ] ; then echo "goose version not found in go.mod"; exit 1 ; fi ; \
	grep -q "ARG GOOSE_VERSION=$$VERSION" build/goose/Dockerfile || { echo "build/goose/Dockerfile out of sync (expected $$VERSION)"; exit 1; } ; \
	grep -q "default: \"$$VERSION\"" .github/workflows/goose-image.yml || { echo "goose-image.yml default out of sync (expected $$VERSION)"; exit 1; } ; \
	grep -q "goose_version || '$$VERSION'" .github/workflows/goose-image.yml || { echo "goose-image.yml fallback out of sync (expected $$VERSION)"; exit 1; } ; \
	echo "goose version synced: $$VERSION"

check-all: check-goose-version check helm-lint docker-lint ## Полная проверка: Go + Helm + Dockerfile

## Миграции

migrate-up: ## Применить миграции
	$(GOOSE) -dir /migrations sqlite3 /data/alerts.db up

migrate-down: ## Откатить последнюю миграцию
	$(GOOSE) -dir /migrations sqlite3 /data/alerts.db down

migrate-status: ## Показать статус миграций
	$(GOOSE) -dir /migrations sqlite3 /data/alerts.db status

migrate-create: ## Создать новую миграцию (NAME=имя)
	$(GOOSE) -dir /migrations sqlite3 /data/alerts.db create $(NAME) sql

## Helm

CHART_DIR := deploy/helm/alertmanager-webhook-relay

helm-prepare: ## Скопировать миграции в Helm chart (для локальной разработки)
	mkdir -p $(CHART_DIR)/files/
	cp -r migrations/ $(CHART_DIR)/files/migrations/

## Очистка

clean: ## Удалить артефакты и остановить контейнеры
	rm -f $(APP_NAME) coverage.out coverage.html
	docker compose --profile tools down -v --remove-orphans
