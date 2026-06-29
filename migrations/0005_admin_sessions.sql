-- Admin-сессии (отдельно от пользовательских сессий).
CREATE TABLE admin_sessions (
    token       TEXT PRIMARY KEY,           -- случайный 256-бит токен
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_admin_sessions_expiry ON admin_sessions(expires_at);