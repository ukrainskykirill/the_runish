ALTER TABLE subscription_reminder_logs ALTER COLUMN days_before DROP NOT NULL;
ALTER TABLE subscription_reminder_logs ADD COLUMN IF NOT EXISTS hours_before INTEGER;
ALTER TABLE subscription_reminder_logs
    ADD CONSTRAINT uq_sub_reminder_hours UNIQUE (subscription_id, hours_before);

INSERT INTO app_settings (key, value) VALUES
    ('subscription_reminder_hours', '168,72,24'),
    ('training_reminder_hours', '24'),
    ('notify_quiet_from', '22'),
    ('notify_quiet_to', '9')
ON CONFLICT (key) DO NOTHING;
