.PHONY: help build test test-race test-cover fmt vet lint check clean

APP_NAME := alertmanager-webhook-relay
RUN := docker compose --profile tools run --rm dev

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

## Качество кода

fmt: ## Отформатировать код
	$(RUN) gofmt -w .

vet: ## Запустить go vet
	$(RUN) go vet ./...

lint: ## Запустить линтер
	$(RUN) golangci-lint run

check: fmt vet lint test-race ## Полная проверка: fmt + vet + lint + test-race

## Очистка

clean: ## Удалить артефакты и остановить контейнеры
	rm -f $(APP_NAME) coverage.out coverage.html
	docker compose --profile tools down -v --remove-orphans
