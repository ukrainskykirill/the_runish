-- Запросы на вход через Telegram deep-link (фолбэк для Login Widget).
-- Сайт создаёт nonce, бот подтверждает его при получении /start <nonce>.

CREATE TABLE login_requests (
    nonce      TEXT PRIMARY KEY,
    user_id    BIGINT REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'confirmed'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_login_requests_expiry ON login_requests(expires_at);
