[Back to README](../README.md) · [Архитектура →](architecture.md)

# Быстрый старт

## Требования

- Docker и Docker Compose (сборка, тесты, линтер — всё в контейнерах)
- Make

> Go на хосте не требуется — всё выполняется внутри Docker-контейнеров.

## Установка

```bash
git clone https://github.com/Etherlord/alertmanager-webhook-relay.git
cd alertmanager-webhook-relay
```

## Сборка

```bash
make build
```

## Запуск тестов

```bash
make test          # все тесты
make test-race     # с race detector
make test-cover    # с отчётом покрытия (coverage.html)
```

Интеграционные тесты (поднимают Mailpit для проверки Email):

```bash
make test-integration
```

## Проверка качества кода

```bash
make check         # fmt + vet + lint + test-race (полная проверка)
make check-all     # + helm-lint + docker-lint
```

## Запуск приложения

### Минимальная конфигурация (только Pachca)

```bash
export PACHCA_TOKEN="your-token"
export PACHCA_CHAT_ID="12345"

docker compose --profile tools run --rm dev ./alertmanager-webhook-relay
```

### С Email-уведомлениями

```bash
export EMAIL_SMTP_HOST="smtp.example.com"
export EMAIL_FROM="alerts@example.com"
export EMAIL_TO="team@example.com"
export EMAIL_USERNAME="alerts@example.com"
export EMAIL_PASSWORD="your-password"
export PACHCA_TOKEN="your-token"
export PACHCA_CHAT_ID="12345"

docker compose --profile tools run --rm dev ./alertmanager-webhook-relay
```

### Проверка работы

Liveness check:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Readiness check:

```bash
curl http://localhost:8080/readyz
# {"status":"ok"}
```

Отправка тестового алерта:

```bash
curl -X POST http://localhost:8080/api/v1/alerts \
  -H "Content-Type: application/json" \
  -d @.scripts/alert-samples/06_*_ServiceDown.json
```

## Миграции (SQLite)

```bash
make migrate-up       # применить
make migrate-down     # откатить последнюю
make migrate-status   # статус
```

## Доступные Make-цели

| Цель | Описание |
|------|----------|
| `build` | Собрать бинарник |
| `test` | Запустить тесты |
| `test-race` | Тесты с race detector |
| `test-cover` | Тесты с покрытием |
| `test-integration` | Интеграционные тесты (Mailpit) |
| `fmt` | Отформатировать код |
| `vet` | Запустить go vet |
| `lint` | Запустить golangci-lint |
| `check` | fmt + vet + lint + test-race |
| `check-all` | check + helm-lint + docker-lint |
| `migrate-up` | Применить миграции |
| `migrate-down` | Откатить последнюю миграцию |
| `clean` | Удалить артефакты |

## Дальнейшие шаги

- [Конфигурация](configuration.md) — все ENV-переменные
- [Деплой](deployment.md) — запуск в Kubernetes с Helm

## See Also

- [Архитектура](architecture.md) — структура проекта и паттерны
- [Конфигурация](configuration.md) — полный справочник ENV-переменных
