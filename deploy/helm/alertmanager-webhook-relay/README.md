# Alertmanager Webhook Relay Helm Chart

Helm chart для деплоя Alertmanager Webhook Relay в Kubernetes.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.x
- PV provisioner (если `persistence.enabled: true`)

## Установка

```bash
helm install my-relay ./deploy/helm/alertmanager-webhook-relay
```

С кастомными значениями:

```bash
helm install my-relay ./deploy/helm/alertmanager-webhook-relay \
  --set secret.DATABASE_DSN="file:/data/alerts.db?_journal=WAL" \
  --set secret.PACHCA_TOKEN="your-token" \
  --set config.PACHCA_CHAT_ID="12345"
```

Или через файл values:

```bash
helm install my-relay ./deploy/helm/alertmanager-webhook-relay -f my-values.yaml
```

## Конфигурация

### Основные параметры

| Параметр | Описание | Default |
|----------|----------|---------|
| `replicaCount` | Количество реплик | `1` |
| `image.repository` | Docker-образ | `alertmanager-webhook-relay` |
| `image.tag` | Тег образа | `appVersion` из Chart.yaml |
| `image.pullPolicy` | Pull policy | `IfNotPresent` |
| `strategy.type` | Стратегия деплоя (`Recreate` для SQLite) | `RollingUpdate` |

### Service

| Параметр | Описание | Default |
|----------|----------|---------|
| `service.type` | Тип сервиса | `ClusterIP` |
| `service.port` | Порт сервиса | `8080` |

### Persistence (SQLite)

| Параметр | Описание | Default |
|----------|----------|---------|
| `persistence.enabled` | Включить PVC | `true` |
| `persistence.size` | Размер тома | `1Gi` |
| `persistence.accessMode` | Access mode | `ReadWriteOnce` |
| `persistence.storageClass` | Storage class | `""` |

### Network Policy

| Параметр | Описание | Default |
|----------|----------|---------|
| `networkPolicy.enabled` | Включить NetworkPolicy | `true` |
| `networkPolicy.alertmanager.namespaceSelector` | Namespace selector для Alertmanager | `{}` |
| `networkPolicy.alertmanager.podSelector` | Pod selector для Alertmanager | `{}` |

### Database Migrations

| Параметр | Описание | Default |
|----------|----------|---------|
| `migration.enabled` | Включить initContainer с goose для миграций | `false` |
| `migration.image.repository` | Образ goose | `ghcr.io/pressly/goose` |
| `migration.image.tag` | Тег образа goose | `3.27.1` |
| `migration.resources` | Resource limits/requests для initContainer | requests: 50m/32Mi, limits: 100m/64Mi |

Миграции выполняются initContainer перед стартом приложения. SQL-файлы загружаются из ConfigMap, который собирается из `migrations/<driver>/` при публикации чарта.

### Security

- Pod запускается от non-root пользователя (UID 65534)
- Read-only root filesystem
- Все Linux capabilities отключены
- Seccomp profile: RuntimeDefault
- ServiceAccount: automountServiceAccountToken disabled
- NetworkPolicy: default deny-all с явным allow-list

### Config (ConfigMap)

| Параметр | Описание | Default |
|----------|----------|---------|
| `configMap.create` | Создавать ConfigMap с параметрами из `config` | `true` |
| `configMap.existingConfigMap` | Имя существующего ConfigMap (вместо создания нового) | `""` |

Non-sensitive параметры передаются через ConfigMap. Полный список — в `values.yaml`, секция `config`.

### Secret

| Параметр | Описание | Default |
|----------|----------|---------|
| `secret.create` | Создавать Secret с данными из `secret.*` | `true` |
| `secret.existingSecret` | Имя существующего Secret (вместо создания нового) | `""` |
| `secret.useInEnvFrom` | Подключать Secret через envFrom в контейнер | `true` |

Sensitive параметры передаются через Secret. Замените placeholder-значения перед деплоем:

| Параметр | Описание |
|----------|----------|
| `secret.DATABASE_DSN` | DSN базы данных |
| `secret.PACHCA_TOKEN` | Токен Pachca API |
| `secret.EMAIL_USERNAME` | SMTP логин |
| `secret.EMAIL_PASSWORD` | SMTP пароль |

## Примеры

### SQLite с миграциями

```yaml
migration:
  enabled: true
strategy:
  type: Recreate   # рекомендуется для SQLite (single-writer, ReadWriteOnce PVC)
secret:
  DATABASE_DSN: "file:/data/alerts.db?_journal=WAL"
```

Стратегии деплоя:
- `Recreate` — безопасная стратегия для SQLite (рекомендуется). Гарантирует, что только один Pod обращается к PVC.
- `RollingUpdate` — работает, если storage class позволяет multi-pod mount на одной ноде (стандартное поведение `ReadWriteOnce`).

### Минимальный деплой с SQLite

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

### Деплой с Email-уведомлениями

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

### Использование внешнего Secret

Если секреты управляются вне Helm (например, через External Secrets Operator):

```yaml
secret:
  create: false
  existingSecret: "my-external-secret"  # имя существующего Secret в namespace
```

### Использование внешнего ConfigMap

```yaml
configMap:
  create: false
  existingConfigMap: "my-external-configmap"  # имя существующего ConfigMap в namespace
```

### Секреты без envFrom (монтирование через volumes)

Если секреты передаются не через envFrom, а, например, через volume mount:

```yaml
secret:
  create: false
  useInEnvFrom: false  # secretRef не будет в envFrom
```

### Ограничение ingress до конкретного namespace

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

## Удаление

```bash
helm uninstall my-relay
```

PVC не удаляется автоматически. Для полной очистки:

```bash
kubectl delete pvc my-relay-alertmanager-webhook-relay
```
