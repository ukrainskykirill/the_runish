# Security & Code Review — The Runish

Ревью бэкенда (Go) + инфраструктуры. Ниже — найденные проблемы и план изменений.
Код пока не менялся.

---

## Сводка проблем безопасности

| # | Severity | Проблема | Где |
|---|----------|----------|-----|
| S1 | 🔴 Critical | Боевые секреты (`BOT_TOKEN`, `ADMIN_TOKEN`) в репозитории | docker-compose.yml:29,43 |
| S2 | 🔴 Critical | Бэкдор dev-логина обходится `?dev=1` в проде | api_auth.go:132 |
| S3 | 🔴 Critical | Слабые дефолтные креды админки (`test`/`1111`) | config.go:50 |
| S4 | 🟠 High | Mock-оплата `POST /payment/mock/{id}` активна без авторизации в любом режиме | app.go:91, payment_mock.go:72 |
| S5 | 🟠 High | Нет rate-limit на вход в админку (брутфорс) | admin_auth.go:26 |
| S6 | 🟡 Medium | Админ-cookie без `Secure`/`SameSite` | admin_auth.go:68 |
| S7 | 🟡 Medium | Нет security-заголовков (CSP, X-Frame-Options, nosniff) | app.go |
| S8 | 🟡 Medium | CSRF держится только на `SameSite=Lax` | весь admin |
| S9 | 🔵 Low | `auth_date` из будущего проходит валидацию | telegram.go:73 |
| S10 | 🔵 Low | Webhook не сверяет `Amount` с суммой заказа | payment_webhook.go |
| S11 | 🔵 Low | `APIMe` отдаёт весь `domain.User` (incl. `is_admin`) | api.go:11 |

**Что уже сделано хорошо:** весь SQL параметризован (инъекций нет), `html/template`
автоэкранирует, токены 256-бит CSPRNG, HMAC/Token-сравнения constant-time,
`ActivateSubscriptionTx` идемпотентна с `FOR UPDATE`.

---

## Детали критичных находок

### S1 — Боевые секреты в репозитории
В `docker-compose.yml:29,43` ранее были захардкожены реальный `BOT_TOKEN` и
`ADMIN_TOKEN` (значения вырезаны из этого отчёта; сейчас выносятся в env/секреты). HMAC-секрет проверки
Telegram-логина = `SHA256(BOT_TOKEN)` (`telegram.go:77`), поэтому с утёкшим токеном можно
**подделать подпись Login Widget и войти под любым Telegram-ID**. Токен считать
скомпрометированным.

### S2 — Бэкдор dev-логина
`api_auth.go:132`: проверка `if BotToken != "" && query["dev"] != "1" { forbid }`.
Флаг `?dev=1` контролирует атакующий → в проде `POST /api/auth/dev?dev=1` логинит без
аутентификации под `telegram_id 999000001`.

### S3 — Слабые дефолтные креды админки
`config.go:50-51`: `ADMIN_LOGIN` по умолчанию `test`, `ADMIN_PASSWORD` — `1111`, и они не
входят в `required`. Недонастроенный деплой открывает админ-панель.

### S4 — Mock-оплата всегда зарегистрирована
`app.go:91-92` регистрирует `POST /payment/mock/{orderID}` безусловно, а
`PaymentMockConfirm` (`payment_mock.go:72`) активирует подписку **без авторизации и без
проверки `PAYMENT_PROVIDER`**. В режиме `tbank` — путь к бесплатной подписке.

---

## Проблемы качества кода

| # | Проблема | Где |
|---|----------|-----|
| C1 | Неиспользуемый `args` (запрос использует `id = ANY($1)` с `ids`) | services.go:130 |
| C2 | Мёртвый код `pager.args/placeholders`, параметр `start` | db.go:148 |
| C3 | Дублирование скана строк платежа | payments.go (GetPaymentByTBankOrderID / ListPending...) |
| C4 | `context.Background()` вместо `r.Context()` в запросах внутри обработки запроса | manager.go:41, admin_auth.go:61 |
| C5 | Корзина привязана к токену сессии — теряется при logout/ротации | api_cart.go:29 |

---

## План изменений (по приоритету)

1. **S1** — убрать литералы `BOT_TOKEN`/`ADMIN_TOKEN` из `docker-compose.yml`, брать из
   env/`.env` (уже в `.gitignore`). Токен перевыпустить в @BotFather (вручную).
2. **S2** — добавить в `config.go` флаг `DevLogin bool` (default false); регистрировать
   `POST /api/auth/dev` в `app.go` только при включённом флаге; убрать зависимость от
   `?dev=1`.
3. **S3 + .env.example** — перенести `ADMIN_LOGIN`/`ADMIN_PASSWORD` в `required` в
   `config.go` (без дефолтов); поправить `.env.example`.
4. **S4** — регистрировать роуты `/payment/mock/*` в `app.go` только при
   `cfg.PaymentProvider == "mock"`.
5. **S6** — добавить `Secure`/`SameSite` для админ-cookie в `admin_auth.go` (протянуть
   `secure`-флаг через `App`/`Deps`).
6. **S7** — middleware security-заголовков вокруг `Routes()` в `app.go`
   (nosniff, frame-deny, базовый CSP).
7. **S5** — лёгкий in-memory rate-limit на `POST /admin/login`.
8. **S9, S10, C1–C4** — мелкие правки корректности и чистка кода.

Опционально (вне основного объёма): S8 (CSRF-токены), S11 (DTO для `/api/me`), C5.

---

## Проверка после изменений
- `go build ./...` и `go vet ./...`.
- `go test ./...` (точечные тесты на отклонение будущего `auth_date` и на guard `APIAuthDev`).
- Вручную: при `PAYMENT_PROVIDER=tbank` — `POST /payment/mock/<id>` отдаёт 404; при заданном
  `BOT_TOKEN` — `POST /api/auth/dev?dev=1` запрещён; старт падает при пустом `ADMIN_PASSWORD`.
