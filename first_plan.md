# The Runish — технический план реализации (Go, SSR)

Клон сайта therunish.ru на Go: каталог услуг, корзина, оплата через T-Bank
эквайринг, авторизация через Telegram, личные подписки с уведомлениями о
продлении, серверная ручка для управления каталогом.

**Стек:** Go 1.26, stdlib (`net/http`, `html/template`, `database/sql`,
`crypto/hmac`, `crypto/sha256`, `log/slog`), PostgreSQL через `pgx/v5`.
Единственная внешняя зависимость — драйвер БД. Без Redis (сессии в Postgres),
без роутеров и ORM, Telegram и T-Bank — на голом `http.Client`.

---

## 0. Принципы

- **Только stdlib + pgx.** Никаких фреймворков. Роутинг — `http.ServeMux` с
  паттернами Go 1.22+ (`mux.HandleFunc("POST /admin/services", h)`).
- **Деньги — целые в копейках** (`int64`, поля `*_kop`). Никаких float.
- **Транзакции на любой многотабличной записи.** Webhook, создание заказа с
  позициями, активация подписки — всё атомарно через `BeginTx`.
- **Идемпотентность входящих событий.** Webhook T-Bank приходит несколько раз —
  повторная доставка не должна активировать подписку дважды.
- **Платёжный провайдер за интерфейсом.** `PaymentProvider` с реализацией
  `tbank` и `mock` для локальной разработки/тестов.
- **Конфиг и секреты — из env.** Токен бота, пароль терминала, admin-токен,
  DSN. Ничего в коде.

---

## 1. Структура репозитория

```
therunish/
├── go.mod                      # module ..., go 1.26
├── cmd/
│   ├── web/main.go             # HTTP-сервер сайта (SSR + API + webhook)
│   └── worker/main.go          # фоновый воркер: напоминания, истечение подписок
├── internal/
│   ├── config/                 # загрузка env в структуру Config
│   ├── domain/                 # сущности и бизнес-правила (без I/O)
│   │   ├── service.go          # Service, ServiceKind
│   │   ├── user.go             # User
│   │   ├── order.go            # Order, OrderItem
│   │   ├── payment.go          # Payment, PaymentStatus
│   │   └── subscription.go     # Subscription, статусы, флаги напоминаний
│   ├── storage/                # доступ к Postgres (database/sql + pgx stdlib)
│   │   ├── db.go               # пул, helper WithTx
│   │   ├── services.go
│   │   ├── users.go
│   │   ├── orders.go
│   │   ├── payments.go
│   │   ├── subscriptions.go
│   │   └── sessions.go
│   ├── session/                # логика сессий поверх storage.sessions
│   ├── auth/
│   │   ├── telegram.go         # проверка подписи Login Widget (HMAC-SHA256)
│   │   └── middleware.go       # requireUser, requireAdmin
│   ├── payment/
│   │   ├── provider.go         # интерфейс PaymentProvider
│   │   ├── tbank.go            # реализация: Init/GetState/Cancel + токен-подпись
│   │   └── mock.go             # заглушка для разработки
│   ├── telegram/
│   │   ├── client.go           # sendMessage, getUpdates/webhook
│   │   └── notify.go           # формирование текстов уведомлений
│   ├── handlers/
│   │   ├── pages.go            # /, /runners, /shop, /legal/*
│   │   ├── cart.go             # корзина
│   │   ├── checkout.go         # оформление -> создание заказа+платежа
│   │   ├── payment_webhook.go  # приём Notification от T-Bank
│   │   ├── auth.go             # вход через Telegram, logout
│   │   └── admin.go            # CRUD каталога (серверная ручка)
│   └── render/                 # обёртка над html/template (кэш, хелперы)
├── web/
│   ├── templates/              # layout.html, index.html, runners.html, ...
│   └── static/                 # CSS, JS, картинки (перенос с оригинала)
└── migrations/
    ├── 0001_init.sql
    └── ...
```

Два бинарника: `web` (сайт) и `worker` (фоновые задачи). Делят `internal/`.

---

## 2. Схема БД (миграции)

Связи через `REFERENCES users(id)`. Порядок создания учитывает FK.

```sql
-- users
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    telegram_id     BIGINT UNIQUE NOT NULL,
    username        TEXT,
    full_name       TEXT,
    phone           TEXT,
    bot_dialog_open BOOLEAN NOT NULL DEFAULT false,  -- нажал ли /start у бота
    is_admin        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- services: каталог (подписки + разовые тренировки + бесплатные)
CREATE TABLE services (
    id            BIGSERIAL PRIMARY KEY,
    kind          TEXT NOT NULL,            -- 'subscription'|'training'|'free'
    title         TEXT NOT NULL,
    description   TEXT,
    price_kop     BIGINT NOT NULL,
    duration_days INT,                      -- для подписок; NULL для разовых
    sort_order    INT NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- orders: что купили
CREATE TABLE orders (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    amount_kop  BIGINT NOT NULL,
    status      TEXT NOT NULL,              -- 'created'|'paid'|'cancelled'
    contact_phone TEXT,
    contact_name  TEXT,
    contact_tg    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_user ON orders(user_id);

-- order_items: позиции (снимок цены на момент покупки)
CREATE TABLE order_items (
    id            BIGSERIAL PRIMARY KEY,
    order_id      BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    service_id    BIGINT NOT NULL REFERENCES services(id),
    title         TEXT NOT NULL,            -- снимок названия
    price_kop     BIGINT NOT NULL,          -- снимок цены
    qty           INT NOT NULL DEFAULT 1
);
CREATE INDEX idx_order_items_order ON order_items(order_id);

-- payments: как платили (T-Bank)
CREATE TABLE payments (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id),
    order_id         BIGINT NOT NULL REFERENCES orders(id),
    amount_kop       BIGINT NOT NULL,
    status           TEXT NOT NULL,         -- 'new'|'pending'|'confirmed'|'rejected'|'refunded'
    provider         TEXT NOT NULL DEFAULT 'tbank',
    tbank_payment_id TEXT,                  -- PaymentId из Init
    tbank_order_id   TEXT NOT NULL,         -- наш OrderId в Init (уникальный)
    tbank_status     TEXT,                  -- сырой статус банка
    payment_url      TEXT,                  -- PaymentURL для редиректа
    error_code       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at          TIMESTAMPTZ,
    CONSTRAINT uq_tbank_order_id UNIQUE (tbank_order_id)
);
CREATE INDEX idx_payments_user  ON payments(user_id);
CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_tbank ON payments(tbank_payment_id);

-- subscriptions: привязка подписки к юзеру + сроки + флаги напоминаний
CREATE TABLE subscriptions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    service_id   BIGINT NOT NULL REFERENCES services(id),
    payment_id   BIGINT REFERENCES payments(id),
    status       TEXT NOT NULL,             -- 'active'|'expired'|'cancelled'
    started_at   TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    reminded_7d  BOOLEAN NOT NULL DEFAULT false,
    reminded_3d  BOOLEAN NOT NULL DEFAULT false,
    reminded_1d  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subs_user ON subscriptions(user_id);
CREATE INDEX idx_subs_active_expiry
    ON subscriptions(expires_at) WHERE status = 'active';

-- sessions: логин (замена Redis)
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,           -- случайный 256-бит токен (hex/base64url)
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cart_json   JSONB NOT NULL DEFAULT '[]',-- корзина прямо в сессии
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_sessions_expiry ON sessions(expires_at);
```

Миграции — простыми `.sql` файлами, применяются скриптом на старте `web`
(или отдельной командой `go run ./cmd/web -migrate`). Без библиотеки миграций:
таблица `schema_migrations(version)`, накат по возрастанию.

---

## 3. Слой storage и транзакции

Хелпер для транзакций — единая точка, чтобы не плодить ручной
commit/rollback:

```go
// internal/storage/db.go
func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
    tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
    if err != nil {
        return err
    }
    defer func() {
        if p := recover(); p != nil {
            _ = tx.Rollback()
            panic(p)
        }
    }()
    if err := fn(tx); err != nil {
        _ = tx.Rollback()
        return err
    }
    return tx.Commit()
}
```

**Где транзакции обязательны:**

1. **Checkout** — создать `orders` + N строк `order_items` + строку `payments`
   в статусе `new`. Всё либо целиком, либо ничего.
2. **Webhook CONFIRMED** — перевести `payments.status` в `confirmed`, проставить
   `paid_at`, перевести `orders.status` в `paid`, создать/продлить
   `subscriptions`. Всё в одной транзакции с идемпотентной проверкой.
3. **Воркер истечения** — батч-перевод просроченных подписок в `expired`.

**Идемпотентность webhook** (ключевой момент). Внутри транзакции сначала
блокируем строку платежа и проверяем текущий статус:

```go
err := store.WithTx(ctx, func(tx *sql.Tx) error {
    var cur string
    err := tx.QueryRowContext(ctx,
        `SELECT status FROM payments WHERE tbank_order_id = $1 FOR UPDATE`,
        notif.OrderID).Scan(&cur)
    if err != nil {
        return err
    }
    if cur == "confirmed" {
        return nil // уже обработали — повторная доставка, выходим без изменений
    }
    // ... UPDATE payments, UPDATE orders, INSERT/UPDATE subscriptions
    return nil
})
```

`FOR UPDATE` защищает от гонки при параллельных доставках одного уведомления.

**Продление подписки** (а не создание новой, если активная уже есть): если у
юзера есть `active`-подписка на ту же услугу, новый срок добавляется к текущему
`expires_at` и сбрасываются флаги `reminded_*`; иначе создаётся новая.

---

## 4. Платежи: T-Bank эквайринг

Документация: developer.tbank.ru/eacq/api. Используем «свою платёжную форму»
(API напрямую), флоу: **Init → редирект на PaymentURL → Notification webhook →
GetState для подстраховки**.

### 4.1. Интерфейс

```go
// internal/payment/provider.go
type InitResult struct {
    PaymentID  string
    PaymentURL string
    Status     string
}

type PaymentProvider interface {
    Init(ctx context.Context, p InitParams) (InitResult, error)
    GetState(ctx context.Context, paymentID string) (string, error)
    Cancel(ctx context.Context, paymentID string) error
}
```

Реализации: `tbank.go` (боевая) и `mock.go` (локально — сразу возвращает
фейковый PaymentURL на внутреннюю страницу-эмулятор).

### 4.2. Подпись токена (T-Bank)

Каждый запрос подписывается: берём все корневые параметры запроса (кроме
вложенных объектов/массивов и самого `Token`), добавляем `Password` терминала,
сортируем по ключу, конкатенируем значения, считаем `SHA-256`, hex в нижнем
регистре — это поле `Token`. Та же схема используется для **валидации
входящего webhook**: пересчитываем токен от тела уведомления и сравниваем с
присланным `Token`. Несовпадение → 403, событие игнорируем.

```go
func sign(params map[string]string, password string) string {
    params["Password"] = password
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    var sb strings.Builder
    for _, k := range keys {
        sb.WriteString(params[k])
    }
    sum := sha256.Sum256([]byte(sb.String()))
    return hex.EncodeToString(sum[:])
}
```

### 4.3. Init

POST `https://securepay.tinkoff.ru/v2/Init` с телом: `TerminalKey`, `Amount`
(в копейках), `OrderId` (наш `tbank_order_id`), `Description`,
`NotificationURL` (наш webhook), `SuccessURL`, `FailURL`, `Token`. При работе
по 54-ФЗ — объект `Receipt` (позиции чека, email/phone клиента, НДС). В ответе
`PaymentId`, `PaymentURL`, `Status` — сохраняем в `payments`, редиректим юзера
на `PaymentURL`.

### 4.4. Webhook (Notification)

T-Bank шлёт POST на `NotificationURL` при смене статуса. Обработчик:
1. Прочитать тело, провалидировать `Token`.
2. Найти платёж по `OrderId` (`tbank_order_id`).
3. На `Status == "CONFIRMED"` — транзакция из п.3.2 (идемпотентно).
4. На `REJECTED`/`AUTH_FAIL` — пометить платёж `rejected`, сохранить
   `ErrorCode`.
5. **Ответить ровно `HTTP 200` с телом `OK`** — иначе банк повторяет доставку.

Статусы банка → наши: `NEW/FORM_SHOWED/AUTHORIZING → pending`,
`CONFIRMED → confirmed`, `REJECTED/AUTH_FAIL/DEADLINE_EXPIRED → rejected`,
`REFUNDED/PARTIAL_REFUNDED → refunded`.

### 4.5. Подстраховка через GetState

Webhook может потеряться. Воркер раз в N минут берёт `payments` в статусе
`pending` старше M минут и дёргает `GetState` — синхронизирует статус. Та же
идемпотентная транзакция активации.

---

## 5. Авторизация через Telegram (один бот)

Один бот через @BotFather: логин + личные уведомления + добавление в чаты.
Нужны `BOT_TOKEN` и `BOT_USERNAME` в env.

### 5.1. Login Widget (основной путь)

Виджет на странице входа отдаёт подписанные данные: `id`, `first_name`,
`username`, `photo_url`, `auth_date`, `hash`. Сервер валидирует:
- `secret_key = SHA256(BOT_TOKEN)`;
- `data_check_string` = отсортированные `key=value` (кроме `hash`) через `\n`;
- `hash == HMAC_SHA256(data_check_string, secret_key)`;
- `auth_date` не старше ~24 ч (защита от replay).

При успехе: upsert `users` по `telegram_id`, создать `sessions`, поставить
cookie (`HttpOnly`, `Secure`, `SameSite=Lax`, срок = `expires_at` сессии).

### 5.2. Deep-link фолбэк

Если виджет недоступен: сайт генерит `nonce`, ссылка
`t.me/<BOT_USERNAME>?start=<nonce>`. Юзер жмёт Start → бот ловит `/start nonce`
→ привязывает `telegram_id` к сессии, ставит `bot_dialog_open = true`. Сайт
логинит по SSE/короткому поллингу. Бонус: после этого пути личка с ботом
гарантированно открыта.

### 5.3. Право на личные сообщения

Бот может писать юзеру, только если тот нажал Start. Поэтому:
- `users.bot_dialog_open` ставится `true` при получении `/start` от этого
  `telegram_id`;
- воркер напоминаний шлёт только тем, у кого флаг `true`;
- после виджет-логина, если флаг `false`, сайт показывает кнопку «Подключить
  уведомления» со ссылкой на бота.

### 5.4. Middleware

`requireUser` (есть валидная сессия) и `requireAdmin` (`users.is_admin` или
совпадение с admin-токеном) — обычные `func(http.Handler) http.Handler`.

---

## 6. Уведомления о продлении (воркер)

`cmd/worker` — тикер раз в сутки (например 10:00 МСК). Выборка кандидатов:

```sql
SELECT s.id, s.user_id, s.service_id, s.expires_at,
       s.reminded_7d, s.reminded_3d, s.reminded_1d, u.telegram_id
FROM subscriptions s
JOIN users u ON u.id = s.user_id
WHERE s.status = 'active'
  AND u.bot_dialog_open = true
  AND (
    (NOT s.reminded_7d AND (s.expires_at::date - CURRENT_DATE) = 7) OR
    (NOT s.reminded_3d AND (s.expires_at::date - CURRENT_DATE) = 3) OR
    (NOT s.reminded_1d AND (s.expires_at::date - CURRENT_DATE) = 1)
  );
```

Для каждой: `sendMessage(telegram_id, текст со сроком и ссылкой на /runners)`,
затем выставить соответствующий `reminded_Nd = true` (флаги, а не вычисление по
дате — переживают пропущенные тики и рестарты). На ошибку `403 Forbidden`
(юзер заблокировал бота) — снять `bot_dialog_open`. Отдельный тик переводит
`expires_at < now()` из `active` в `expired`.

---

## 7. Серверная ручка управления каталогом

Защищённый CRUD для добавления/правки услуг и подписок. Защита — `requireAdmin`
(статичный `ADMIN_TOKEN` из env на первом этапе, далее — по `is_admin`).

```
POST   /admin/services         создать услугу/подписку
PATCH  /admin/services/{id}     отредактировать
DELETE /admin/services/{id}     мягкое удаление (is_active=false)
GET    /admin/services          список (вкл. неактивные)
```

Тело `POST` (JSON):

```json
{
  "kind": "subscription",
  "title": "Беговое комьюнити The Runish",
  "description": "...",
  "price_kop": 800000,
  "duration_days": 30,
  "sort_order": 50
}
```

Валидация: `kind` из допустимого множества; для `subscription` обязателен
`duration_days > 0`; для `free` — `price_kop == 0`; `price_kop >= 0`.

---

## 8. Веб-страницы (SSR) и маршруты

`html/template` с общим `layout.html` и блоками. Шаблоны парсятся один раз на
старте и кэшируются в `render`.

```
GET  /                     лендинг/вход (Telegram Login Widget)
GET  /runners              каталог услуг (карточки из services)
GET  /shop                 магазин
GET  /legal/offer          публичная оферта (отдельная страница, для SEO)
GET  /legal/privacy        политика обработки ПД
POST /cart                 добавить/изменить позицию (в session.cart_json)
GET  /cart                 показать корзину
POST /checkout             создать order+payment, редирект на PaymentURL
POST /payment/webhook      Notification от T-Bank (без авторизации, по Token)
GET  /payment/success      возврат при успехе (SuccessURL)
GET  /payment/fail         возврат при отказе (FailURL)
GET  /auth/telegram        приём данных Login Widget -> сессия
POST /auth/logout          выход
GET  /me                   личный кабинет (этап 2): подписки, история
```

Корзина живёт в `sessions.cart_json` (массив `{service_id, qty}`); цены и
названия резолвятся из `services` на момент показа и фиксируются снимком при
checkout. Контент (логотип, иконки, тексты оферты/политики, расписание) —
переносится с оригинала в `web/static` и шаблоны.

---

## 9. Конфигурация (env)

```
DATABASE_URL          postgres://...
HTTP_ADDR             :8080
BASE_URL              https://therunish.ru     # для NotificationURL/Success/Fail
SESSION_TTL           720h

BOT_TOKEN             <от @BotFather>
BOT_USERNAME          therunish_bot

TBANK_TERMINAL_KEY    <из ЛК>
TBANK_PASSWORD        <из ЛК>
TBANK_API_BASE        https://securepay.tinkoff.ru/v2

ADMIN_TOKEN           <случайный секрет>
PAYMENT_PROVIDER      tbank | mock
```

---

## 10. Порядок реализации (этапы)

1. **Каркас.** go.mod (go 1.26), config, подключение pgx, `WithTx`, скелет
   миграций и таблицы `schema_migrations`, healthcheck.
2. **БД.** Миграция `0001_init.sql` со всеми таблицами из п.2. Сидинг каталога
   текущими услугами/ценами оригинала.
3. **Storage.** CRUD-методы по таблицам, `WithTx`-операции для checkout и
   активации.
4. **SSR.** `render` + шаблоны: `/`, `/runners`, `/shop`, `/legal/*`. Перенос
   статики и контента.
5. **Корзина + checkout.** Сессии в Postgres, cookie, создание order+items+
   payment в транзакции.
6. **Платежи.** `PaymentProvider` + `mock`, затем `tbank` (Init, подпись,
   webhook с валидацией Token, success/fail).
7. **Telegram-логин.** Валидация Login Widget, сессия; deep-link фолбэк через
   бота; `bot_dialog_open`.
8. **Подписки.** Активация/продление в webhook-транзакции; привязка к юзеру.
9. **Воркер.** Напоминания 7/3/1 день, истечение подписок, GetState-подстраховка
   для зависших платежей.
10. **Admin-ручка.** CRUD каталога + `requireAdmin`.
11. **Прод.** Dockerfile (2 бинарника), webhook по HTTPS, прогон сценария
    оплаты на тестовом терминале T-Bank.

---

## 11. Тесты (stdlib `testing`)

- `payment`: юнит на функцию подписи токена (фикстуры из доки T-Bank);
  валидация webhook-Token на подделанном теле → отказ.
- `auth`: проверка HMAC Login Widget на корректных/протухших/подделанных
  данных.
- `storage`: интеграционные на тестовой БД — идемпотентность webhook (двойная
  доставка CONFIRMED создаёт одну подписку), продление существующей подписки,
  батч-истечение.
- `worker`: выбор кандидатов на напоминание по флагам и датам, отсутствие
  дублей.

---

## 12. Открытые вопросы

- Чеки по 54-ФЗ: нужна ли онлайн-касса/`Receipt` в `Init` (зависит от
  оформления ИП клуба) — уточнить, влияет на тело `Init`.
- Боевой `BOT_TOKEN`/`BOT_USERNAME`: завести бота или взять существующий.
- Тестовый терминал T-Bank: ключ и пароль для прогона до боевого подключения.
- Список Telegram-чатов по услугам для авто-добавления после оплаты (п. 5.6
  оферты) — на этап 2.