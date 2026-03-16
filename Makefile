.PHONY: build test test-race test-cover fmt vet lint check clean

APP_NAME := alertmanager-webhook-relay
RUN := docker compose run --rm dev

## Сборка

build:
	$(RUN) go build -buildvcs=false -o $(APP_NAME) .

## Тесты

test:
	$(RUN) go test ./...

test-race:
	$(RUN) go test -race ./...

test-cover:
	$(RUN) sh -c 'go test -cover -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html'

## Качество кода

fmt:
	$(RUN) gofmt -w .

vet:
	$(RUN) go vet ./...

lint:
	$(RUN) golangci-lint run

check: fmt vet lint test-race

## Очистка

clean:
	rm -f $(APP_NAME) coverage.out coverage.html
	docker compose down -v --remove-orphans
