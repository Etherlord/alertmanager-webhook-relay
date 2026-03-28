[← Конфигурация](configuration.md) · [Back to README](../README.md)

# Деплой

## Docker

### Сборка образа

Multi-stage Dockerfile: `dev` (с линтером и тестами) и `prod` (distroless, ~5 MB).

```bash
# Production-образ
docker build --target prod -t alertmanager-webhook-relay:1.0.0 .
```

### Запуск

```bash
docker run -d \
  -p 8080:8080 \
  -v alertdata:/data \
  -e DATABASE_DSN="data/alerts.db" \
  -e PACHCA_TOKEN="your-token" \
  -e PACHCA_CHAT_ID="12345" \
  alertmanager-webhook-relay:1.0.0
```

### Docker Compose (разработка)

```bash
# Сборка и тесты
make build
make test

# Запуск приложения
docker compose --profile tools run --rm -p 8080:8080 dev ./alertmanager-webhook-relay
```

## Kubernetes (Helm)

### Требования

- Kubernetes 1.26+
- Helm 3.x
- PV provisioner (для SQLite persistence)

### Добавление Helm-репозитория

Чарт публикуется в Helm-репозитории на GitHub Pages:

```bash
helm repo add alertmanager-webhook-relay https://etherlord.github.io/alertmanager-webhook-relay
helm repo update
```

### Установка

```bash
helm install my-relay alertmanager-webhook-relay/alertmanager-webhook-relay
```

С кастомными values:

```bash
helm install my-relay alertmanager-webhook-relay/alertmanager-webhook-relay \
  --set secret.DATABASE_DSN="file:/data/alerts.db?_journal=WAL" \
  --set secret.PACHCA_TOKEN="your-token" \
  --set config.PACHCA_CHAT_ID="12345"
```

Или через файл:

```bash
helm install my-relay alertmanager-webhook-relay/alertmanager-webhook-relay -f my-values.yaml
```

> **Для разработки** можно ставить из локального чарта: `helm install my-relay ./deploy/helm/alertmanager-webhook-relay`

### Минимальный деплой (SQLite + Pachca)

```yaml
# my-values.yaml
strategy:
  type: Recreate  # обязательно для SQLite (single-writer, ReadWriteOnce PVC)
config:
  PACHCA_CHAT_ID: "12345"
secret:
  DATABASE_DSN: "file:/data/alerts.db?_journal=WAL"
  PACHCA_TOKEN: "your-token"
```

### Деплой с Email

```yaml
config:
  EMAIL_SMTP_HOST: "smtp.example.com"
  EMAIL_SMTP_PORT: "587"
  EMAIL_FROM: "alerts@example.com"
  EMAIL_TO: "team@example.com"
  EMAIL_TLS_MODE: "starttls"
secret:
  EMAIL_USERNAME: "alerts@example.com"
  EMAIL_PASSWORD: "your-password"
```

### Helm-параметры

| Параметр | Описание | Default |
|----------|----------|---------|
| `replicaCount` | Количество реплик | `1` |
| `image.repository` | Docker-образ | `alertmanager-webhook-relay` |
| `image.tag` | Тег образа | `appVersion` из Chart.yaml |
| `strategy.type` | Стратегия деплоя | `RollingUpdate` |
| `service.type` | Тип сервиса | `ClusterIP` |
| `service.port` | Порт | `8080` |
| `persistence.enabled` | PVC для SQLite | `true` |
| `persistence.size` | Размер тома | `1Gi` |
| `networkPolicy.enabled` | NetworkPolicy | `true` |

### Безопасность

- Pod запускается от **non-root** пользователя (UID 65534)
- **Read-only** root filesystem
- Все Linux capabilities отключены
- Seccomp profile: `RuntimeDefault`
- `automountServiceAccountToken: false`
- NetworkPolicy: default **deny-all** с явным allow-list

### NetworkPolicy

Ограничение ingress до namespace monitoring:

```yaml
networkPolicy:
  enabled: true
  alertmanager:
    namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: monitoring
    podSelector:
      matchLabels:
        app.kubernetes.io/name: alertmanager
```

### Health Checks

| Endpoint | Тип | Описание |
|----------|-----|----------|
| `GET /healthz` | Liveness | Всегда 200 OK |
| `GET /readyz` | Readiness | Проверяет доступность БД |

### Graceful Shutdown

1. Kubernetes отправляет preStop hook → приложение ждёт `PRE_STOP_DELAY`
2. SIGTERM → HTTP-сервер прекращает приём запросов
3. Dispatcher дожидает in-flight уведомлений
4. Store закрывается (WAL checkpoint для SQLite)

`PRE_STOP_DELAY` должен быть меньше `SHUTDOWN_TIMEOUT` — иначе Kubernetes может SIGKILL до завершения drain.

### Удаление

```bash
helm uninstall my-relay
```

PVC не удаляется автоматически:

```bash
kubectl delete pvc my-relay-alertmanager-webhook-relay
```

## CI/CD

Проект использует GitHub Actions:

| Workflow | Описание |
|----------|----------|
| `ci.yml` | Lint, test, helm-lint |
| `release.yml` | Сборка и публикация Docker-образа |
| `helm-publish.yml` | Публикация Helm chart |

## See Also

- [Конфигурация](configuration.md) — все ENV-переменные и их ограничения
- [Архитектура](architecture.md) — graceful shutdown, data flow
