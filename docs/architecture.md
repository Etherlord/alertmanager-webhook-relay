[← Быстрый старт](getting-started.md) · [Back to README](../README.md) · [Конфигурация →](configuration.md)

# Архитектура

## Обзор

Alertmanager Webhook Relay построен по паттерну **Modular Monolith** — единый Go-бинарник с чёткими модульными границами внутри `internal/`. Каждый модуль инкапсулирует свою бизнес-область и общается с другими через Go-интерфейсы.

## Data Flow

```
Alertmanager                                             Pachca API
    │                                                        ▲
    ▼                                                        │
┌─────────┐    ┌─────────┐    ┌──────────┐    ┌──────────┐  │
│  HTTP    │───▶│ Alerts  │───▶│ Notify   │───▶│ Channel  │──┘
│ Server   │    │ Service │    │Dispatcher│    │ (Pachca) │
│          │    │         │    │          │    └──────────┘
│ /healthz │    │ Store   │    │ Queue    │    ┌──────────┐
│ /readyz  │    │ Valid.  │    │ per ch.  │───▶│ Channel  │──▶ SMTP
└─────────┘    └────┬─────┘    └──────────┘    │ (Email)  │
                    │                          └──────────┘
                    ▼
              ┌──────────┐
              │ SQLite / │
              │PostgreSQL│
              └──────────┘
```

1. **HTTP Server** принимает POST от Alertmanager, применяет middleware (Metrics → Logging → Recovery)
2. **Alerts Service** валидирует payload, сохраняет в Store
3. **Notify Dispatcher** забирает новые алерты из Store и маршрутизирует по каналам
4. **Channels** (Pachca, Email) отправляют уведомления независимо, каждый со своей очередью

## Структура проекта

```
.
├── main.go                      # Composition root: wiring, graceful shutdown
├── internal/
│   ├── config/                  # ENV-конфигурация со строгой валидацией
│   ├── logging/                 # Структурированное JSON-логирование
│   ├── server/                  # HTTP-сервер, роутинг
│   │   ├── health.go            # /healthz, /readyz, preStop hook
│   │   └── middleware/          # Metrics → Logging → Recovery
│   ├── alerts/                  # Приём и хранение алертов
│   │   ├── handler.go           # HTTP handler для webhook
│   │   ├── model.go             # Alert, AlertGroup
│   │   ├── service.go           # Бизнес-логика
│   │   ├── store.go             # Repository interface
│   │   └── validation.go        # Валидация payload, DoS protection
│   ├── notify/                  # Система уведомлений
│   │   └── dispatcher.go        # Маршрутизация по каналам
│   ├── channel/                 # Реализации каналов
│   │   ├── pachca/              # Pachca (Markdown)
│   │   └── email/               # Email (SMTP)
│   ├── template/                # Шаблоны с hot-reload
│   ├── storage/                 # Реализации Store
│   │   └── sqlite/              # SQLite (dev, single-replica)
│   ├── cleanup/                 # Автоочистка по retention policy
│   ├── metrics/                 # Prometheus метрики
│   └── testutil/                # Тестовые утилиты
├── migrations/sqlite/           # SQL-миграции (goose)
├── templates/email/             # Шаблоны Email-уведомлений
├── deploy/helm/                 # Helm chart для Kubernetes
└── .scripts/alert-samples/      # Примеры реальных алертов
```

## Ключевые принципы

### Interface Segregation

Каждый модуль определяет интерфейсы, которые ему нужны. `alerts.Store` определён в `alerts/store.go`, реализован в `storage/sqlite/`.

### Composition Root

`main.go` — единственное место, где модули связываются. Модули не знают друг о друге напрямую.

### Независимые очереди

Каждый канал уведомлений имеет свою очередь и worker. Медленный Email не блокирует Pachca.

### Storage Abstraction

Бизнес-логика работает с interfaces. Выбор SQLite/PostgreSQL — решение конфигурации.

## Правила зависимостей

| Направление | Допустимо? |
|-------------|-----------|
| `main.go` → любой модуль | ✅ composition root |
| `server` → `alerts`, `notify` | ✅ регистрация handlers |
| `notify` → `channel` interface | ✅ через interface |
| `channel/*` → `template` | ✅ форматирование |
| `storage/*` → interfaces из `alerts` | ✅ реализация |
| `alerts` → `storage/sqlite` напрямую | ❌ только interface |
| `config`, `logging` → бизнес-модули | ❌ cross-cutting |
| Циклические импорты | ❌ всегда |

## Graceful Shutdown

Последовательность завершения гарантирует, что все in-flight уведомления будут обработаны:

1. **HTTP Server** — прекращает приём новых запросов, дожидается текущих
2. **Dispatcher** — останавливается, дожидается drain in-flight уведомлений
3. **Store** — закрывает соединения (WAL checkpoint для SQLite)

## See Also

- [Быстрый старт](getting-started.md) — установка и первый запуск
- [Конфигурация](configuration.md) — все параметры приложения
