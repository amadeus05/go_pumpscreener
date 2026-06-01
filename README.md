# Pumpscreener

Telegram-бот для мониторинга Bybit USDT perpetual-пар. Правила глобальные: одно правило проверяет все разрешенные пары, кроме blacklist.

## Правила

Формат:

```text
/add up 3 30m 15m
/add down 5 1h 30m
/add up 3 5m 1m hold 3m
```

Это значит: найти любую пару, которая выросла или упала на указанный процент внутри окна. Уведомление приходит сразу при достижении условия, не только в конце окна.

Максимальный интервал правила по умолчанию `30m`. Больше бот не даст добавить, чтобы удерживать память под контролем на слабом сервере.

Режимы:

```text
instant      сигнал сразу при достижении условия
hold 3m      сигнал только если условие держится непрерывно 3 минуты
```

## Команды

```text
/add up 3 30m 15m
/add up 3 5m 1m hold 3m
/rules
/delete 3
/pause 3
/resume 3
/blacklist
/blacklist_add BTCUSDT
/blacklist_remove BTCUSDT
/status
```

## Web endpoints

Сервис слушает порт `8000` по умолчанию.

```text
GET  /
GET  /uptime
GET  /health
HEAD /health
```

`/uptime` показывает человекочитаемый uptime, состояние WebSocket, количество пар и правил.

## Запуск

1. Установить Go 1.22+.
2. Скопировать `.env.example` в `.env` и заполнить Telegram token/chat id.
3. Экспортировать переменные окружения или задать их в панели хостинга.
4. Запустить:

```bash
go mod tidy
go run .
```

Для сервера удобнее собрать бинарник:

```bash
go build -o pumpscreener .
./pumpscreener
```

## Docker

Локальная сборка:

```bash
docker build -t pumpscreener .
docker run --rm -p 8000:8000 --env-file .env pumpscreener
```

Проверка:

```bash
curl http://localhost:8000/uptime
curl -I http://localhost:8000/health
```

## Render Web Service

На Render создавай именно `Web Service`, не worker, потому что сервис слушает HTTP-порт для uptime checks.

Настройки:

```text
Environment: Docker
Health Check Path: /health
```

Environment variables:

```text
TELEGRAM_BOT_TOKEN=...
TELEGRAM_CHAT_ID=...
STORAGE_BACKEND=file
DATABASE_PATH=/app/data/pumpscreener.json
MAX_RULES=20
MAX_INTERVAL=30m
MAX_POINTS_PER_SYMBOL=4096
PRICE_BUCKET_INTERVAL=1s
BLACKLIST=USDCUSDT
```

## Supabase/Postgres storage

Default storage is still the local JSON file:

```text
STORAGE_BACKEND=file
DATABASE_PATH=/app/data/pumpscreener.json
```

To use Supabase, take the Postgres connection string from Supabase project settings and set:

```text
STORAGE_BACKEND=supabase
SUPABASE_DB_URL=postgresql://postgres.PROJECT_REF:PASSWORD@aws-0-REGION.pooler.supabase.com:6543/postgres?sslmode=require&default_query_exec_mode=simple_protocol
```

`DATABASE_URL` also works if `SUPABASE_DB_URL` is not set. The app creates the `rules`, `blacklist`, and `signals` tables automatically on startup.

`PORT` Render задает сам. Если Telegram-переменные пустые, сигналы будут писаться в логи Render.

## Архитектура

```text
src/domain          чистые модели и проверка правил
src/application     price windows, cooldown, commands, screener
src/infrastructure  Bybit, Telegram, HTTP, storage
src/core            config, uptime, duration helpers, app state
```

Для слабого сервера код держит ограниченные каналы и хранит не каждый тик, а компактные OHLC buckets. По умолчанию `PRICE_BUCKET_INTERVAL=1s`: все тики внутри одной секунды обновляют один bucket.
