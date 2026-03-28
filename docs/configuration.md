[← Архитектура](architecture.md) · [Back to README](../README.md) · [Деплой →](deployment.md)

# Конфигурация

Вся конфигурация задаётся через переменные окружения. Значения нормализуются (trim, lowercase для enum) и проходят строгую валидацию при старте.

## Основные параметры

| Переменная | Описание | Default | Диапазон |
|------------|----------|---------|----------|
| `PORT` | Порт HTTP-сервера | `8080` | 1–65535 |
| `LOG_LEVEL` | Уровень логирования | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | Таймаут graceful shutdown | `15s` | > 0, ≤ 5m |
| `PRE_STOP_DELAY` | Задержка перед SIGTERM (Kubernetes preStop) | — | 0–30s |

## База данных

| Переменная | Описание | Default | Ограничения |
|------------|----------|---------|-------------|
| `DATABASE_DRIVER` | Драйвер БД | `sqlite` | `sqlite` |
| `DATABASE_DSN` | Строка подключения | `data/alerts.db` | ≤ 2048 символов, без control chars |

## Защита от DoS

| Переменная | Описание | Default | Диапазон |
|------------|----------|---------|----------|
| `MAX_PAYLOAD_SIZE` | Максимальный размер тела запроса (байт) | `1048576` (1 MB) | 1024–10485760 |
| `MAX_ALERTS_PER_PAYLOAD` | Максимум алертов в одном payload | `100` | 1–1000 |

## Система уведомлений

| Переменная | Описание | Default | Диапазон |
|------------|----------|---------|----------|
| `NOTIFY_POLL_INTERVAL` | Интервал опроса новых алертов | `5s` | 1s–60s |
| `NOTIFY_BATCH_SIZE` | Размер батча за одну итерацию | `50` | 1–500 |
| `NOTIFY_QUEUE_SIZE` | Размер очереди уведомлений | `100` | 10–10000, ≥ BATCH_SIZE |
| `NOTIFY_SEND_TIMEOUT` | Таймаут отправки одного уведомления | `30s` | 5s–120s |

## Pachca

Канал включается автоматически при установке `PACHCA_TOKEN`.

| Переменная | Описание | Default | Ограничения |
|------------|----------|---------|-------------|
| `PACHCA_TOKEN` | API-токен Pachca | — | 1–512 символов, printable ASCII |
| `PACHCA_CHAT_ID` | ID чата для отправки | — | 1–999999999 |
| `PACHCA_BASE_URL` | Базовый URL API | `https://api.pachca.com` | http/https, ≤ 2048 |

## Email

Канал включается автоматически при установке `EMAIL_SMTP_HOST`.

| Переменная | Описание | Default | Ограничения |
|------------|----------|---------|-------------|
| `EMAIL_SMTP_HOST` | SMTP-сервер | — | ≤ 253 символов |
| `EMAIL_SMTP_PORT` | Порт SMTP | `587` | 1–65535 |
| `EMAIL_FROM` | Адрес отправителя | — | валидный email (RFC 5322) |
| `EMAIL_TO` | Получатели (через запятую) | — | валидные email-адреса |
| `EMAIL_USERNAME` | SMTP логин | — | ≤ 256 символов |
| `EMAIL_PASSWORD` | SMTP пароль | — | ≤ 512 символов, printable ASCII |
| `EMAIL_TLS_MODE` | Режим TLS | `starttls` | `starttls`, `tls`, `none` |
| `EMAIL_SUBJECT_PREFIX` | Префикс темы письма | `[Alert]` | ≤ 128 символов |

> `EMAIL_USERNAME` и `EMAIL_PASSWORD` должны быть оба заданы или оба пусты.

> `EMAIL_TLS_MODE=none` — трафик не шифруется. Используйте только в dev/test.

## Шаблоны

| Переменная | Описание | Default | Ограничения |
|------------|----------|---------|-------------|
| `TEMPLATE_DIR` | Директория с шаблонами | — | ≤ 512 символов |
| `TEMPLATE_RELOAD` | Hot-reload шаблонов | — | `true`, `false` |

Если `TEMPLATE_DIR` не задан, используется форматирование по умолчанию.
При `TEMPLATE_RELOAD=true` приложение отслеживает изменения файлов в `TEMPLATE_DIR` и перезагружает шаблоны без перезапуска.

## Кросс-полевая валидация

- `NOTIFY_QUEUE_SIZE` ≥ `NOTIFY_BATCH_SIZE`
- `PRE_STOP_DELAY` < `SHUTDOWN_TIMEOUT`

## Безопасность

- `DATABASE_DSN`, `PACHCA_TOKEN`, `EMAIL_PASSWORD` редактируются в логах (`[REDACTED]`)
- Все строковые параметры проверяются на control chars и опасные последовательности (CRLF, null byte, shell expansion, template injection)
- Токены и пароли принимают только printable ASCII (0x21–0x7E)

## See Also

- [Быстрый старт](getting-started.md) — первый запуск с минимальной конфигурацией
- [Деплой](deployment.md) — как передавать секреты в Kubernetes
