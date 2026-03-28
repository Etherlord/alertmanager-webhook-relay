# Alertmanager Webhook Relay

> Принимает вебхуки от Prometheus Alertmanager и пересылает уведомления в Pachca и Email.

Cloud-native Go-приложение для маршрутизации алертов из Alertmanager в каналы уведомлений с независимыми очередями, retry-механизмом и шаблонизацией. Разработано для Kubernetes, поддерживает SQLite (dev/single-replica) и PostgreSQL (production/HA).

## Quick Start

### Kubernetes (Helm)

```bash
helm repo add alertmanager-webhook-relay https://etherlord.github.io/alertmanager-webhook-relay
helm repo update
helm install my-relay alertmanager-webhook-relay/alertmanager-webhook-relay \
  --set secret.PACHCA_TOKEN="your-token" \
  --set config.PACHCA_CHAT_ID="12345"
```

### Разработка

```bash
git clone https://github.com/Etherlord/alertmanager-webhook-relay.git
cd alertmanager-webhook-relay
make build && make test
```

## Ключевые возможности

- **Приём вебхуков** — совместим с Alertmanager webhook API, валидация payload и защита от DoS
- **Два канала уведомлений** — Pachca (Markdown) и Email (SMTP) с независимыми очередями
- **Шаблоны** — кастомизация формата уведомлений с hot-reload без перезапуска
- **Надёжная доставка** — retry-механизм, circuit breaker, back-pressure protection
- **Kubernetes-ready** — health checks (`/healthz`, `/readyz`), graceful shutdown, Helm chart
- **Наблюдаемость** — Prometheus-метрики, структурированное JSON-логирование

## Пример

Отправка тестового алерта:

```bash
curl -X POST http://localhost:8080/api/v1/alerts \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "status": "firing",
      "labels": {"alertname": "ServiceDown", "severity": "critical"},
      "annotations": {"summary": "Service is down"}
    }]
  }'
```

---

## Документация

| Раздел | Описание |
|--------|----------|
| [Быстрый старт](docs/getting-started.md) | Установка, настройка, первый запуск |
| [Архитектура](docs/architecture.md) | Структура проекта, паттерны, data flow |
| [Конфигурация](docs/configuration.md) | Все ENV-переменные с описанием и дефолтами |
| [Деплой](docs/deployment.md) | Docker, Kubernetes, Helm chart |

## Лицензия

MIT
