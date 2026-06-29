-- Журнал отправленных автоматических напоминаний о скором окончании подписки.
-- Позволяет настраивать любые периоды вместо жёстких 7/3/1 и не отправлять
-- одно и то же напоминание повторно.
CREATE TABLE subscription_reminder_logs (
    id              BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    days_before     INTEGER NOT NULL CHECK (days_before >= 0),
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (subscription_id, days_before)
);

CREATE INDEX idx_subscription_reminder_logs_sub
    ON subscription_reminder_logs(subscription_id);

INSERT INTO app_settings (key, value)
VALUES ('subscription_reminder_days', '7,3,1')
ON CONFLICT (key) DO NOTHING;

INSERT INTO subscription_reminder_logs (subscription_id, days_before)
SELECT id, 7 FROM subscriptions WHERE reminded_7d
ON CONFLICT (subscription_id, days_before) DO NOTHING;

INSERT INTO subscription_reminder_logs (subscription_id, days_before)
SELECT id, 3 FROM subscriptions WHERE reminded_3d
ON CONFLICT (subscription_id, days_before) DO NOTHING;

INSERT INTO subscription_reminder_logs (subscription_id, days_before)
SELECT id, 1 FROM subscriptions WHERE reminded_1d
ON CONFLICT (subscription_id, days_before) DO NOTHING;
