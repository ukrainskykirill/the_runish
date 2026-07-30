# AGENTS.md — The Runish

Project guide for humans and AI agents: stack, how to run, architecture and — most importantly —
the **non-obvious business logic**.

---

## 1. What it is

A web app (SSR admin + React SPA) for the running club **The Runish**: catalog of services and
subscriptions, cart, payment via **T-Bank acquiring** (with 54-FZ fiscal receipts), **Telegram**
auth, personal subscriptions with reminders, a Telegram-bot onboarding survey, training schedule,
news, merch and an **admin panel**.

### Stack
- **Backend:** Go 1.26, stdlib only (`net/http`, `html/template`, `database/sql`, `crypto/hmac`,
  `log/slog`). No frameworks/ORM. Routing — `http.ServeMux` (Go 1.22+ patterns: `GET /path`, `{id}`).
- **DB:** PostgreSQL 16 via `pgx/v5` (stdlib driver). Hand-rolled migrations (no lib), see `migrations/`.
- **Frontend:** React + TypeScript + Vite (SPA), `react-router-dom`. Lives in `web/frontend/`.
- **Payments:** T-Bank over a bare `http.Client` (`internal/payment/tbank.go`) + a `mock` for local dev.
- **Telegram:** bot client over `http.Client` (`internal/telegram/`).
- **Deploy:** Docker Compose (`db` + `web` + `worker`).

---

## 2. Run & deploy

```bash
docker compose up -d --build        # brings up db + web + worker
# web listens on :8080, frontend is built inside Docker (multi-stage) into web/frontend/dist
```

- **Migrations** run automatically on `web` startup (read from disk in `migrations/`, tracked in the
  `schema_migrations` table). To add one — drop a new `00NN_*.sql`.
- **Catalog seeding** is done NOT via a migration but via raw SQL: `scripts/seed_catalog_v2.sql`
  (deactivates old services, inserts the current catalog). Run:
  `docker compose exec -T db psql -U postgres -d therunish < scripts/seed_catalog_v2.sql`.
- **Admin templates** are listed explicitly in `cmd/web/main.go` (`render.New(...)`). A new admin
  template must be added to that list, otherwise you get `template not found`.
- **External access (demo):** via `cloudflared tunnel --url http://localhost:8080` (quick tunnel,
  URL is ephemeral). **Important:** when the URL changes, update `BASE_URL` in `docker-compose.yml`
  (web + worker) and recreate the containers — otherwise the T-Bank webhook and payment redirects break.

### Access / env (see `internal/config/config.go`, `.env.example`)
- **Admin:** `/admin/login`, defaults login `ADMIN_LOGIN=test`, password `ADMIN_PASSWORD=1111`.
  Session — cookie `runish_admin` (table `admin_sessions`). Guard — `RequireAdminToken`.
- **Coach:** `/coach/login`, defaults `COACH_LOGIN=runishtren` / `COACH_PASSWORD=12345678`.
  Separate cookie `runish_coach`, same `admin_sessions` table but with `role='coach'`. Guard —
  `auth.RequirePanel(store, roles…)`. The coach only sees the **Планы** section; the admin sees
  it too (same handlers mounted under both `/coach/plans` and `/admin/plans`, see `panelBase`).
- Key env: `DATABASE_URL`, `BASE_URL`, `PAYMENT_PROVIDER` (`tbank|mock`),
  `TBANK_TERMINAL_KEY/PASSWORD/API_BASE`, `TBANK_TAXATION/TAX/PAYMENT_OBJECT/PAYMENT_METHOD` (54-FZ receipt),
  `BOT_TOKEN/BOT_USERNAME`, `SESSION_TTL`, `ADMIN_LOGIN/PASSWORD/TOKEN`, `COACH_LOGIN/PASSWORD`.

---

## 3. Architecture

```
cmd/web/        — HTTP server (SSR admin + JSON API + webhook + SPA fallback)
cmd/worker/     — background worker + Telegram bot (polling)
internal/
  config/       — env loading
  domain/       — entities (User, Service, Subscription, Payment, Order, Training, Survey, News, Merch)
  storage/      — Postgres access (one file per entity) + migrations (db.go)
  session/      — user sessions (in Postgres, cookie runish_session)
  auth/         — Telegram Login + middleware (LoadUser "soft", RequireUser, RequireAdminToken)
  payment/      — PaymentProvider (tbank + mock), Token signing, receipts, Refund
  telegram/     — bot client (sendMessage, inline keyboards, callback_query) + texts
  survey/       — onboarding survey definition (pure logic, branching)
  handlers/     — HTTP handlers (api*, admin*, payment*, pricing.go)
  render/       — html/template wrapper (+ funcMap)
  worker/       — pending-payment poller (safety net)
web/frontend/   — React SPA (public site)
web/templates/  — ADMIN HTML templates (public site is React; templates are admin-only)
web/static/     — admin CSS static
migrations/     — SQL migrations
scripts/        — raw SQL (catalog seed)
```

**Important about the public site:** it's a React SPA from `web/frontend/dist`, which Go serves as
static + an index.html fallback (`serveSPA`). `web/templates/*.html` are **admin only**.

---

## 4. Data model & entities

- **`services`** — catalog. The `kind` field:
  - `free` — free trial training;
  - `entry` — **membership entry fee**;
  - `bundle` — **entry fee + first month of subscription** (single payment);
  - `subscription` — subscription (monthly);
  - `training` — one-off / individual service.
  - Prices: `price_kop` (base), `price_with_sub_kop` ("with subscription" discount, nullable),
    `promo_price_kop` ("first 30" promo, nullable). `duration_days` — for subscription/bundle.
- **`users`** — has the flag **`entry_fee_paid`** (entry fee paid — permanent). (`sub_autorenew` was
  removed — there is no auto-renew, see migration 0010.)
- **`subscriptions`** — active/expired/cancelled. `service_id` points to the service (for a bundle
  purchase this is the bundle service!). `payment_id` — which payment granted it.
- **`payments`** / **`orders`** / **`order_items`** — orders and payments. `order_items.title` is a
  snapshot of the service name (matters for the receipt).
- **`app_settings`** — key-value: `first30_promo_enabled`, `subscription_reminder_days` (e.g. `7,3,1`).
- **`survey_responses`** — onboarding survey state/answers (`pending|in_progress|completed`).
- **`trainings`** — schedule (recurring, by day of week `weekday` 1..7 + `start_time`).
- **`training_plans`** — weekly plan written by the coach. One row per week (`week_start`, always a
  Monday, UNIQUE), `status` `draft|published`, and the whole grid in JSONB `groups`
  (`[{title, days:[{date, weekday, kind, task, link_label, link_url} ×7]}]`) + `materials`
  (`[{label,url}]`). Stored as JSONB because the editor saves the whole grid in one submit.
- Others: `news`, `merch`, `cart_items` (cart keyed by `session_id`), `sessions`, `admin_sessions`,
  `login_requests` (deep-link login), `subscription_reminder_logs`.

---

## 5. Key business logic (must read)

### 5.1 Single source of price & availability — `internal/handlers/pricing.go`
`pricingContext(ctx, user)` gathers: `promoActive`, `hasActiveSub`, `entryPaid`.
`apply(svc)` sets on each service:
- **`EffectivePriceKop`**: base → if promo and `promo_price_kop` set → promo → if active subscription
  and `price_with_sub_kop` set → "with subscription" price.
- **`Locked`** = `kind==subscription && !entryPaid` (can't buy a subscription without the entry fee).
- **`Owned`** = `(kind∈{entry,bundle} && entryPaid)` OR `(kind==subscription && hasActiveSub)`
  (entry/bundle — once and for life; don't re-buy a subscription while one is active).

This function is the **single source of truth**: used by the catalog (`/api/catalog`, `/api/home`)
AND by checkout. The client is NOT trusted — checkout recomputes everything.

### 5.2 Entry fee ↔ subscription (the model)
- **You cannot buy the subscription (4300/mo) until the entry fee is paid** (`entry_fee_paid`).
- **Bundle** (entry + first month) — the newcomer path: paying the bundle sets `entry_fee_paid=true`
  **forever** and creates a subscription. Available once.
- The **entry fee** can also be bought standalone (`entry`).
- Paying `entry`/`bundle` → `entry_fee_paid=true` (see `ActivateSubscriptionTx`).
- **"First 30" promo:** the discounted price applies while `first30_promo_enabled` (admin toggle)
  **AND** the number of users who paid the fee is `< 30` (dynamic counter `CountEntryFeePaid`).

### 5.3 "With subscription" discounts
Configured **per product** in the admin service form (field "Price with subscription"). Applied
automatically to active subscribers (via `pricing.apply`) — in the catalog, cart and checkout.

### 5.4 Cart rules (`internal/handlers/api_cart.go`, `api_checkout.go`)
- **Each product — max 1 unit** (`AddToCart` = `ON CONFLICT DO NOTHING`).
- **A subscription-type product (subscription/bundle) — at most one in the cart.** Adding a second
  one → `400 subscription_already_in_cart`. Checkout duplicates the check (`multiple_subscriptions`).
- Checkout also gates `Locked` (`entry_fee_required`) and `Owned` (`already_owned`), and computes the
  total / price snapshot from `EffectivePriceKop`.

### 5.5 T-Bank payments (`internal/payment/tbank.go`)
- **Token signature:** root-level scalars + `Password`, keys sorted, values concatenated, SHA-256 hex.
  Nested objects (`Receipt`) are **excluded** from the token (see `sign`/`isScalar`).
- **54-FZ receipt:** built in `App.buildReceipt(order)` (only if `ContactPhone` is present); `Name` =
  snapshot of the service name. Used in both `/Init` and **`/Refund` (refund receipt)**.
- **Webhook** (`/payment/webhook`): validate Token, then by status → `ActivateSubscriptionTx` /
  `RejectPaymentTx` / `RefundPaymentTx`. ⚠️ **T-Bank quirk:** in the webhook `PaymentId` arrives as a
  **number**, while in the `/Init` response it's a string. Handled by the `flexID` type (accepts both).
- **Safety net:** the `worker` polls `GetState` every 10 min for stuck pending payments.

### 5.6 Refunds (`RefundPaymentTx` in storage/payments.go)
Full refund (`REFUNDED`): payment→refunded, order→cancelled, active subscriptions of this payment→
cancelled, **and if the order contained `entry`/`bundle` → `entry_fee_paid=false`** (membership
revoked). Partial (`PARTIAL_REFUNDED`) — only records the status (manual handling).

### 5.7 Admin membership management
- The user card shows a "Member status" block: entry fee / active subscription.
- Buttons **"Revoke fee" / "Mark as paid"** (`POST /admin/users/{id}/entry-fee`).
- **Deleting a bundle-subscription from admin also revokes `entry_fee_paid`** (the bundle included the
  fee); deleting a plain subscription does NOT touch the fee.

### 5.8 Telegram-bot onboarding survey (`internal/survey`, `cmd/worker/survey.go`)
- Created **only for new** users: on first registration (INSERT) `UpsertUser` (CTE via `xmax=0`)
  creates `survey_responses(status='pending')`. Existing users don't get it.
- The bot drives the dialog with inline keyboards (single/multi) + free text, with **branching** by
  running experience. After completion `status='completed'` — never shown again.
- Answers are visible to the admin on the user card.

### 5.9 Subscription reminders
- Periods are configured in admin (`subscription_reminder_days`, e.g. `7,3,1`).
- `worker.runReminders` sends a Telegram message N days before expiry; logged in
  `subscription_reminder_logs` to avoid duplicates.

### 5.10 Weekly training plan (coach)
- The coach fills a week at `/coach/plans/{id}/edit`: several **groups** («The Runish Start»,
  «The Runish Progress»), each with 7 rows `Дата · День недели · Тип тренировки · Задание`.
  Dates are always derived from `week_start` — never trusted from the form.
- Flow **draft → publish → notify**. «Сохранить и опубликовать» saves the grid first (both buttons
  submit the same form), so publishing never loses unsaved edits.
- **«Отправить уведомление»** is enabled only for a published plan. It sends
  `tmpl_plan_published` (editable in `/admin/settings`, placeholders `{week}` `{url}`) to
  `ListNotificationUsers(ctx, "active")` — i.e. **only users with an active subscription** and an
  open bot dialog — via the shared `App.broadcast` helper. Result is stored in
  `notified_at`/`notify_sent`.
- **The plan is part of the paid subscription:** `GET /api/plan` returns
  `403 subscription_required` without an active subscription, and the SPA hides the «План» nav item
  and the `/me` section entirely (`subscriptions.length === 0`).

### 5.11 "Free trial" gating (`/api/me`)
`CanBookFreeLesson = !hasConfirmedPayments && !entry_fee_paid && !hasAnySubscription`
(the trial is only for brand-new users). `CanChooseSubscription = no active subscriptions`.

---

## 6. API

**Public JSON** (prefix `/api`): `me`, `home` (aggregate for the homepage), `catalog`, `news`,
`news/{id}`, `merch`, `schedule`, `cart` (GET/POST), `cart/remove`, `checkout`,
`auth/telegram/{start,callback,status,complete}`, `auth/dev`, `auth/logout`, `plan` (RequireUser).
- `LoadUser` (soft middleware) is attached to `me`, `catalog`, `cart*` — so prices/gating are computed
  for the current user. `checkout` — `RequireUser`.

**Payments:** `POST /payment/webhook`, `GET /payment/success|fail`, `GET|POST /payment/mock/{orderID}`.

**Admin (`/admin`, HTML, `RequireAdminToken`):** `login/logout`, CRUD `services` / `news` /
`trainings` / `merch`, `users` + `users/{id}` (+ `entry-fee`, subscriptions: add/edit/extend/delete),
`payments` (+ `{id}/refund`), `settings` (+ `settings/reminders`), `notifications/send`.

**Plans (`/coach/plans` and `/admin/plans`, HTML, `RequirePanel(admin, coach)`):** list / `new` /
`{id}/edit` / `{id}` (save) / `{id}/publish` / `{id}/unpublish` / `{id}/notify` / `{id}/delete`.
Plus `/coach/login` + `/coach/logout`.

---

## 7. Frontend (`web/frontend/`)

- **Routes** (`src/App.tsx`): `/`, `/runners` (catalog), `/news`, `/schedule`, `/merch`, `/cart`,
  `/me`, `/plan` (weekly plan, subscribers only), `/auth/telegram`, `/legal/offer`, `/legal/privacy`, `/payment/success|fail`, `*` (404).
- **API client:** `src/api/client.ts` (`api.*`), types — `src/api/types.ts`.
- **Contexts:** `CartContext` (add/remove/refresh; handles errors — e.g. shows a toast "subscription
  already in cart"), `AuthContext`, `UIContext` (toasts, login modal).
- **Service card** `components/cards/ServiceCard.tsx`: shows `effective_price_kop`, struck-through old
  price + `−%` badge, `locked`/`owned` states (button disabled).
- **Schedule:** `components/schedule/ScheduleBoard.tsx` (weekly board, computes the current week from
  `new Date()`), used both in the homepage section (`ScheduleCalendar`) and the `/schedule` page.
- **Hero video:** `components/HeroVideo.tsx` — resilient loading: poster shown immediately, auto-retry
  of `play()`, recovery on `stalled`/`error`. Assets: `public/hero-run.mp4` + `public/hero-run-poster.jpg`
  (optimized with ffmpeg, `+faststart`). The master source is `the_runnish_main_video.mp4` (don't touch).
- **`ScrollToHash`** (in Layout): correct scroll to `#anchor` after an SPA route change.
- **Styles:** `src/styles/*` (tokens/components), brand variables (`--runish-red`, `--cream`,
  `--font-display`/Anton, `.speedlines`, etc.).

---

## 8. Gotchas / what to watch

- **Single source of price** — `pricing.go`. Add any new price/availability logic there, otherwise
  catalog/cart/checkout diverge (there was a bug: the cart computed the base price).
- **BASE_URL** must be publicly reachable (for the webhook and redirects). On the demo tunnel it changes.
- **T-Bank `PaymentId`**: string in `/Init`, number in the webhook → only via `flexID`.
- **The receipt** is not built without the buyer's phone (a warning is logged, payment not blocked).
- **Catalog seeding is not a migration** — it's `scripts/seed_catalog_v2.sql` (raw INSERT).
- **A new admin template** → don't forget to add it to the list in `cmd/web/main.go`
  (`internal/handlers/templates_test.go` parse-checks every file in `web/templates/`).
- **Deleting entities with FKs** (services, etc.) is soft (`is_active=false`), since orders/
  subscriptions reference them; a hard delete fails.
- The public site is **React**, not Go templates; storefront changes go in `web/frontend/src`, rebuild the image.

---

## 9. Common checks (how to test)

```bash
go build ./...                       # backend
(cd web/frontend && npm run build)   # frontend (tsc + vite)
docker compose build web worker && docker compose up -d --force-recreate web worker

# catalog as guest/user (prices, locked/owned, promo)
curl -s localhost:8080/api/catalog | python3 -m json.tool

# admin: login and pages
curl -s -c /tmp/j -X POST localhost:8080/admin/login --data-urlencode login=test --data-urlencode password=1111
curl -s -b /tmp/j localhost:8080/admin/users/<id>

# payments/webhooks are easiest to exercise with signed requests to /payment/webhook (see repo history)
```
