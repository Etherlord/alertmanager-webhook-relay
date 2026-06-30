.PHONY: help build test test-race test-cover test-integration fmt vet lint check helm-lint docker-lint check-goose-version check-golangci-version check-all clean migrate-up migrate-down migrate-status migrate-create helm-prepare

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

check-goose-version: ## Сверить версию и repo goose: go.mod ↔ Dockerfile ↔ workflow ↔ Helm ↔ docs
	@VERSION=$$(awk '/github.com\/pressly\/goose\/v3 / {print $$2}' go.mod) ; \
	if [ -z "$$VERSION" ] ; then echo "goose version not found in go.mod"; exit 1 ; fi ; \
	TAG=$${VERSION#v} ; \
	REPO=$$(awk '/repository:.*goose/ {print $$2}' deploy/helm/alertmanager-webhook-relay/values.yaml) ; \
	if [ -z "$$REPO" ] ; then echo "goose repository not found in Helm values.yaml"; exit 1 ; fi ; \
	grep -q "ARG GOOSE_VERSION=$$VERSION" build/goose/Dockerfile || { echo "build/goose/Dockerfile out of sync (expected $$VERSION)"; exit 1; } ; \
	grep -q "default: \"$$VERSION\"" .github/workflows/goose-image.yml || { echo "goose-image.yml default out of sync (expected $$VERSION)"; exit 1; } ; \
	grep -q "goose_version || '$$VERSION'" .github/workflows/goose-image.yml || { echo "goose-image.yml fallback out of sync (expected $$VERSION)"; exit 1; } ; \
	grep -q "$$REPO:" .github/workflows/goose-image.yml || { echo "goose-image.yml registry out of sync (expected $$REPO)"; exit 1; } ; \
	grep -q "tag: \"$$TAG\"" deploy/helm/alertmanager-webhook-relay/values.yaml || { echo "Helm values.yaml migration.image.tag out of sync (expected $$TAG)"; exit 1; } ; \
	grep -q "\`$$TAG\`" deploy/helm/alertmanager-webhook-relay/README.md || { echo "Helm README.md goose tag out of sync (expected $$TAG)"; exit 1; } ; \
	grep -q "\`$$REPO\`" deploy/helm/alertmanager-webhook-relay/README.md || { echo "Helm README.md goose repository out of sync (expected $$REPO)"; exit 1; } ; \
	grep -q "\`$$TAG\`" docs/deployment.md || { echo "docs/deployment.md goose tag out of sync (expected $$TAG)"; exit 1; } ; \
	grep -q "\`$$REPO\`" docs/deployment.md || { echo "docs/deployment.md goose repository out of sync (expected $$REPO)"; exit 1; } ; \
	echo "goose synced: $$REPO version=$$VERSION (helm tag: $$TAG)"

check-golangci-version: ## Сверить версию golangci-lint: Dockerfile ↔ ci.yml
	@VERSION=$$(grep -oE 'golangci-lint@v[0-9]+\.[0-9]+\.[0-9]+' Dockerfile | head -1 | sed 's/^golangci-lint@//') ; \
	if [ -z "$$VERSION" ] ; then echo "golangci-lint version not found in Dockerfile"; exit 1 ; fi ; \
	grep -q "version: $$VERSION" .github/workflows/ci.yml || { echo ".github/workflows/ci.yml golangci-lint version out of sync (expected $$VERSION)"; exit 1; } ; \
	echo "golangci-lint synced: $$VERSION"

check-all: check-goose-version check-golangci-version check helm-lint docker-lint ## Полная проверка: Go + Helm + Dockerfile

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
