ALTER TABLE cart_items ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

UPDATE cart_items c
SET user_id = s.user_id
FROM sessions s
WHERE c.session_id = s.id;

DELETE FROM cart_items WHERE user_id IS NULL;

DELETE FROM cart_items a
USING cart_items b
WHERE a.user_id = b.user_id
  AND a.service_id = b.service_id
  AND a.id > b.id;

ALTER TABLE cart_items ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE cart_items DROP CONSTRAINT IF EXISTS uq_cart_session_service;
DROP INDEX IF EXISTS idx_cart_items_session;
ALTER TABLE cart_items DROP COLUMN session_id;
ALTER TABLE cart_items ADD CONSTRAINT uq_cart_user_service UNIQUE (user_id, service_id);
CREATE INDEX idx_cart_items_user ON cart_items(user_id);
