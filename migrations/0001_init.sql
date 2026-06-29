-- Начальная схема БД therunish.
-- Связи через REFERENCES users(id). Порядок создания учитывает FK.
-- Деньги — в копейках (поля *_kop). Никаких float.

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    telegram_id     BIGINT UNIQUE NOT NULL,
    username        TEXT,
    full_name       TEXT,
    phone           TEXT,
    bot_dialog_open BOOLEAN NOT NULL DEFAULT false,
    is_admin        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

CREATE TABLE orders (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    amount_kop    BIGINT NOT NULL,
    status        TEXT NOT NULL,            -- 'created'|'paid'|'cancelled'
    contact_phone TEXT,
    contact_name  TEXT,
    contact_tg    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_user ON orders(user_id);

CREATE TABLE order_items (
    id         BIGSERIAL PRIMARY KEY,
    order_id   BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES services(id),
    title      TEXT NOT NULL,               -- снимок названия
    price_kop  BIGINT NOT NULL,             -- снимок цены
    qty        INT NOT NULL DEFAULT 1
);
CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE payments (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id),
    order_id         BIGINT NOT NULL REFERENCES orders(id),
    amount_kop       BIGINT NOT NULL,
    status           TEXT NOT NULL,         -- 'new'|'pending'|'confirmed'|'rejected'|'refunded'
    provider         TEXT NOT NULL DEFAULT 'tbank',
    tbank_payment_id TEXT,                  -- PaymentId из Init
    tbank_order_id   TEXT NOT NULL,         -- наш OrderId в Init
    tbank_status     TEXT,
    payment_url      TEXT,
    error_code       TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at          TIMESTAMPTZ,
    CONSTRAINT uq_tbank_order_id UNIQUE (tbank_order_id)
);
CREATE INDEX idx_payments_user  ON payments(user_id);
CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_tbank ON payments(tbank_payment_id);

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

-- Сессии (замена Redis). Корзина живёт прямо в session.cart_json.
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,           -- случайный 256-бит токен
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cart_json  JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_sessions_expiry ON sessions(expires_at);