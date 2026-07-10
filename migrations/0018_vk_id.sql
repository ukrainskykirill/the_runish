-- Вход через ВКонтакте: пользователь может быть привязан к Telegram, к ВК или к обоим.
ALTER TABLE users ADD COLUMN vk_id BIGINT UNIQUE;
ALTER TABLE users ALTER COLUMN telegram_id DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_identity_chk
    CHECK (telegram_id IS NOT NULL OR vk_id IS NOT NULL);
