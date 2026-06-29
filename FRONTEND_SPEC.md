# The Runish — Technical & Functional Specification

> Purpose: a ready-to-paste spec for an LLM generating front-end mockups.
> Contains only technical/functional facts. No visual, font, or styling guidance.

## 1. Product Overview

**The Runish** is a running-community website (server-rendered on Go). The frontend will be redesigned; the backend API/contract stays as-is.

The site lets visitors browse a catalog of running-related **services** (subscriptions, one-off trainings, free trial runs), read club **news**, and browse **merch**. Logged-in users (Telegram auth) can build a **cart**, **checkout**, and pay via **T-Bank**. Users have a personal **cabinet** with active subscriptions + order history. Admins manage the catalog, news, merch, and view payments through a separate **admin panel**.

### Tech stack
- Go 1.22+ `net/http` ServeMux, `html/template`
- PostgreSQL (pgx v5)
- Server-rendered HTML with a separate **public layout** and **admin layout**
- Auth: Telegram Login Widget + deep-link bot fallback (user); login/password (admin)
- Payments: T-Bank acquiring (Init → redirect → webhook) with a **mock provider** for dev
- Notifications: Telegram bot reminders (7/3/1 days before subscription expiry)

---

## 2. Data model (for UI fields)

### Service (catalog item)
- `kind`: **subscription** | **training** | **free**
- `title`, `description`
- `price_kop` (integer, kopecks; 800000 = 8 000 ₽; **free must be 0**)
- `duration_days` (required for subscription, null for training/free)
- `sort_order` (int, manual ordering)
- `is_active` (soft visibility toggle)

### News
- `title`, `content`, `sort_order`, `is_active`, `published_at`

### Merch
- `title`, `description`, `price_kop`, `sort_order`, `is_active`

### User
- Telegram identity: `telegram_id`, `username`, `full_name`, `phone`
- `is_admin` flag (not used for admin panel auth — admin uses separate login/password)

### Order
- `amount_kop`, `status`: **created** | **paid** | **cancelled**
- Contact snapshot: `contact_name`, `contact_phone`, `contact_tg`
- Items: `service_id`, `title`, `price_kop`, `qty` (price/title snapshotted at purchase)

### Payment
- Statuses: **new → pending → confirmed / rejected / refunded**
- Provider, T-Bank IDs, `payment_url`, `error_code`, `paid_at`

### Subscription
- `status`: **active / expired / cancelled**
- `started_at`, `expires_at`
- Reminders sent flags: `reminded_7d`, `reminded_3d`, `reminded_1d`
- Created/extended on **confirmed** payment (extends expiry if already active)

### Cart
- Per-session in DB (`sessions.cart_json`)
- Each item: `service_id` + `qty`

---

## 3. Public site — routes & pages

| Method | Path | Description |
|---|---|---|
| GET | `/` | Hero landing (3 feature cards: Training, Community, Subscriptions) |
| GET | `/runners` | **Catalog** of services (`/runners` is the canonical route; label may say "Catalog") |
| GET | `/news` | Club news cards |
| GET | `/merch` | Merch cards (currently display-only, no add-to-cart) |
| GET | `/legal/offer` | Public offer (legal text page) |
| GET | `/legal/privacy` | Privacy policy (legal text page) |
| GET | `/cart` | Full cart page (fallback; main UI is the dropdown) |
| GET | `/me` | Personal cabinet (auth required) |
| GET | `/auth/telegram` | Login page (Telegram widget + deep-link) |
| GET | `/payment/success` | Payment success screen |
| GET | `/payment/fail` | Payment failure screen |

### Navigation (header)
- Logo "The Runish"
- Links: **Home / Catalog / News / Merch**
- **Cart** indicator (always visible) — opens a **dropdown cart** (see §6)
- **Auth state**: if logged in → "Cabinet" + "Log out"; if not → "Log in"

### Footer
- © 2024 The Runish
- Links: Public offer, Privacy policy

---

## 4. User / customer journey (happy path)

1. **Landing (`/`)** — hero + 3 feature cards. CTA: "Sign in via Telegram" (or "Browse catalog" if logged in).
2. **Catalog (`/runners`)** — cards grouped/filtered by kind, each with title, description, duration (for subscriptions), price (formatted as "8 000 ₽"), and an **Add to cart** button.
   - If not logged in and clicks "Add to cart" → redirect to `/auth/telegram?reason=login_required` with a friendly prompt.
3. **Cart dropdown (preferred UI)** — small popover near the cart icon showing line items, qty, subtotal, total, and a **Checkout** button. Also accessible as the full `/cart` page.
4. **Checkout (`POST /checkout`, auth required)** — backend:
   - resolves cart items + prices,
   - creates an `order` (status=created) with items snapshot,
   - creates a `payment` (status=new),
   - calls T-Bank `Init` → gets `PaymentURL`,
   - **clears the cart**,
   - redirects user to T-Bank payment page.
5. **Payment** — user pays on T-Bank; on success the browser returns to `/payment/success`, on failure to `/payment/fail`.
6. **Webhook (`POST /payment/webhook`)** — T-Bank sends notification (validated by token). On `CONFIRMED`: idempotently marks payment confirmed, order paid, and **creates or extends subscriptions** for subscription-kind items.
7. **Cabinet (`/me`)** — user sees active subscriptions (with expiry date + "Renew" link) and order history (id, date, amount, status badge).

### Edge states to design for
- Empty catalog ("No services yet")
- Empty cart ("Cart is empty")
- Not-authenticated on cart add (inline notice + login CTA)
- Payment pending / rejected / refunded states (badges)
- Subscription expired / cancelled

---

## 5. Authentication

### User (Telegram)
- **Telegram Login Widget** embedded on `/auth/telegram`.
- **Deep-link fallback**: `https://t.me/<bot>?start=<nonce>` → user clicks Start in bot → backend confirms → page polls `GET /auth/telegram/status?nonce=...` every 2s → on `confirmed`, redirects to `/auth/telegram/complete` → session created.
- Dev login: `/auth/dev?dev=1` (creates a "Dev User", dev-only).
- Sessions stored in Postgres; cookie is **HttpOnly**, `SameSite=Lax`, `Secure` on HTTPS.

### Admin (login/password)
- Separate `/admin/login` form, separate `runish_admin` cookie, 12h TTL, sessions in `admin_sessions` table.
- Constant-time comparison of login/password against env (`ADMIN_LOGIN` / `ADMIN_PASSWORD`).
- All `/admin/*` routes (except login/logout) guarded by middleware → redirect to login if unauthenticated.

---

## 6. Cart (important for frontend redesign)

**Requirement: cart is a small dropdown/popover on the frontend.**

Backend support already exists:
- `POST /cart` (form: `service_id`) — adds to cart; **if `Accept: application/json`**, returns `{ok:true, cart_size:N}` for AJAX (ideal for dropdown updates without reload).
- `POST /cart/remove` (form: `service_id`) — removes line.
- `GET /cart` — full page (fallback).
- Cart is keyed to the session; a logged-in user's cart is tied to their session in DB.

**Recommended dropdown behavior:**
- Cart icon in header shows a **badge** with current item count (or qty total).
- Click toggles a compact popover listing: title, `price × qty`, subtotal, per-line remove (×), total at the bottom, **Checkout** CTA, and "View full cart" link.
- On "Add to cart" click → AJAX `POST /cart` with `Accept: application/json` → update badge + open/highlight dropdown without full page reload.
- Empty state: "Your cart is empty" + "Browse catalog" link.
- If user not authenticated when adding → backend redirects to `/auth/telegram?reason=login_required`; the frontend can detect and show a login prompt/modal instead.

> Note: currently merch has **no add-to-cart** on the backend (display only). If merch should be purchasable, that requires a backend change.

---

## 7. Admin panel

Separate layout (`admin_layout.html`), distinct from the public site.

### Admin nav sections
1. **Services** — `/admin/services`
2. **News** — `/admin/news`
3. **Merch** — `/admin/merch`
4. **Payments** — `/admin/payments`
5. "Open site →" link, "Log out" button

### 7.1 Services CRUD
- **List** (`GET /admin/services`): table — ID, Kind, Title, Price, Duration, Sort order, Active (✅/❌), Actions (Edit / Delete).
- **New** (`GET /admin/services/new`): form.
- **Create** (`POST /admin/services`).
- **Edit** (`GET /admin/services/{id}/edit`): same form prefilled.
- **Update** (`POST /admin/services/{id}`).
- **Delete** (`POST /admin/services/{id}/delete`): **soft delete** (deactivate).

**Form fields:**
- Kind (select: subscription / training / free) — toggles duration field visibility.
- Title* (text, required)
- Description (textarea)
- Price in kopecks* (number, ≥0; helper: "800000 = 8 000 ₽, 0 = free")
- Duration days (number, ≥1) — shown only for subscription, **required** for subscriptions.
- Sort order (number, default 0)
- Active (checkbox) — visible on site.

**Validation rules (Russian messages currently):**
- Invalid kind → "Неверный тип услуги"
- Negative/non-numeric price → "Цена должна быть неотрицательным числом"
- `free` with price ≠ 0 → "Бесплатная услуга должна иметь цену 0"
- Bad duration → "Срок должен быть положительным числом"
- Subscription without duration → "Подписка требует срок (дни)"

### 7.2 News CRUD
- List + New + Edit + Update + soft Delete (deactivate).
- **Fields:** Title* (required), Content* (textarea, required), Sort order, Active checkbox.
- Errors: "Заголовок обязателен", "Текст новости обязателен".

### 7.3 Merch CRUD
- List + New + Edit + Update + soft Delete (deactivate).
- **Fields:** Title* (required), Description, Price in kopecks (≥0), Sort order, Active checkbox.
- Errors: "Название обязательно", "Цена должна быть неотрицательным числом".

### 7.4 Payments (read-only)
- `GET /admin/payments` → last 50 payments.
- Table columns: ID, Date, Amount, **Status badge** (`new/pending/confirmed/rejected/refunded`), Provider, Client (name + @tg), Contact (name + phone), T-Bank Order ID (code), Paid at, Error code.
- No filters / export currently.

---

## 8. Payment & subscription logic (affects UI states)

- Status flow: order `created` → payment `new` → `pending` → `confirmed`/`rejected`.
- On webhook `CONFIRMED` (idempotent, row-locked): payment → confirmed, order → paid, and for each subscription-kind item: **create new subscription** or **extend existing active subscription's expiry by `duration_days`** (reset 7/3/1-day reminder flags).
- Status badge semantics (color-coded in current admin): confirmed (green), pending (yellow), rejected (red), new (indigo), refunded (purple).

---

## 9. Things to flag / decide before generating mockups

1. **Merch in cart?** Currently merch is display-only — adding it needs backend work. Decide if mockups assume merch is purchasable.
2. **Catalog naming**: route is `/runners` but it's the services catalog. Confirm label ("Catalog" vs "Runners").
3. **Filters / pagination** in admin lists (payments especially) — not currently implemented.
4. **Images for services/news/merch** — no image fields today. Mockups should either omit images or plan a backend addition.