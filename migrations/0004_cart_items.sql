-- Корзина в отдельной таблице вместо JSONB в sessions.
CREATE TABLE cart_items (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    service_id  BIGINT NOT NULL REFERENCES services(id),
    qty         INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_cart_session_service UNIQUE (session_id, service_id)
);
CREATE INDEX idx_cart_items_session ON cart_items(session_id);

-- cart_json больше не нужен.
ALTER TABLE sessions DROP COLUMN IF EXISTS cart_json;