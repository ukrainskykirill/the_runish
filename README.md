# The Runish

Клон therunish.ru на Go (SSR): каталог услуг, корзина, оплата через T-Bank эквайринг, авторизация через Telegram, личные подписки с уведомлениями о продлении, серверная ручка для управления каталогом.

## Стек

- **Go 1.26** — stdlib (`net/http`, `html/template`, `database/sql`, `crypto/hmac`, `log/slog`)
- **PostgreSQL** через `pgx/v5` (stdlib-драйвер)
- Без фреймворков, Redis, ORM — роутинг на `http.ServeMux` (Go 1.22+ patterns)
- Telegram и T-Bank — на голом `http.Client`

## Структура

```
cmd/web/       — HTTP-сервер (SSR + API + webhook)
cmd/worker/    — фоновый воркер (напоминания, истечение подписок, GetState)
internal/
  config/      — загрузка env
  domain/      — сущности и бизнес-правила
  storage/     — доступ к Postgres
  session/     — сессии (в Postgres)
  auth/        — Telegram Login Widget + middleware
  payment/     — PaymentProvider (tbank + mock)
  telegram/    — бот-клиент + тексты уведомлений
  handlers/    — HTTP-обработчики
  render/      — обёртка над html/template
web/
  templates/   — HTML-шаблоны
  static/      — CSS
migrations/    — SQL-миграции
```

## Запуск (Docker Compose)

```bash
cp .env.example .env  # заполнить BOT_TOKEN, T-Bank ключи
docker compose up --build
```

Сайт: http://localhost:8080

## Запуск (локально)

```bash
# 1. Поднять Postgres
docker run -d -p 5432:5432 -e POSTGRES_DB=therunish -e POSTGRES_PASSWORD=postgres postgres:16-alpine

# 2. Установить env (или .env)
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/therunish?sslmode=disable"
export PAYMENT_PROVIDER=mock
# ...

# 3. Запустить web
go run ./cmd/web

# 4. Запустить worker (в другом терминале)
go run ./cmd/worker
```

## Маршруты

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/` | Главная |
| GET | `/runners` | Каталог услуг |
| GET | `/shop` | Магазин |
| GET | `/legal/offer` | Оферта |
| GET | `/legal/privacy` | Политика ПД |
| GET/POST | `/cart` | Корзина |
| POST | `/cart/remove` | Удалить из корзины |
| POST | `/checkout` | Оформление заказа |
| POST | `/payment/webhook` | T-Bank Notification |
| GET | `/payment/success` | SuccessURL |
| GET | `/payment/fail` | FailURL |
| GET | `/auth/telegram` | Telegram Login |
| POST | `/auth/logout` | Выход |
| GET | `/me` | Личный кабинет |
| GET/POST | `/admin/services` | CRUD каталога (admin) |
| PATCH/DELETE | `/admin/services/{id}` | Управление услугой |

## Конфигурация

Все настройки — через env (см. `.env.example`).