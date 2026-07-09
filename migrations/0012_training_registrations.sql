-- Запись на тренировки: конкретные датированные занятия, лимит мест, гранты доступа
-- (подписка-курс / безлимит / разовая тренировка) и сами регистрации.

-- 1. Лимит участников на занятие. NULL = без лимита.
ALTER TABLE trainings ADD COLUMN IF NOT EXISTS capacity INT
    CHECK (capacity IS NULL OR capacity > 0);

-- 2. Сколько занятий включает услуга. NULL = безлимит. Настраивается в админке.
ALTER TABLE services ADD COLUMN IF NOT EXISTS trainings_quota INT
    CHECK (trainings_quota IS NULL OR trainings_quota >= 0);

-- 3. Какие тренировки покрывает услуга. Пусто для услуги = доступ КО ВСЕМ тренировкам.
CREATE TABLE service_trainings (
    service_id  BIGINT NOT NULL REFERENCES services(id)  ON DELETE CASCADE,
    training_id BIGINT NOT NULL REFERENCES trainings(id) ON DELETE CASCADE,
    PRIMARY KEY (service_id, training_id)
);
CREATE INDEX idx_service_trainings_training ON service_trainings(training_id);

-- 4. Грант доступа — то, с чего списываются записи. Унифицирует подписку и разовую.
--    Один действующий грант на подписку (refresh при продлении: used=0, valid_until=expires_at).
CREATE TABLE training_entitlements (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id      BIGINT NOT NULL REFERENCES services(id),
    subscription_id BIGINT REFERENCES subscriptions(id) ON DELETE CASCADE,
    payment_id      BIGINT REFERENCES payments(id),
    quota           INT,                       -- NULL = безлимит
    used            INT NOT NULL DEFAULT 0,
    valid_until     TIMESTAMPTZ,               -- NULL = бессрочно (разовая)
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'exhausted', 'expired')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_training_entitlements_sub
    ON training_entitlements(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX idx_training_entitlements_user ON training_entitlements(user_id, status);

-- 5. Конкретное датированное занятие (создаётся лениво при первой записи).
CREATE TABLE training_sessions (
    id           BIGSERIAL PRIMARY KEY,
    training_id  BIGINT NOT NULL REFERENCES trainings(id) ON DELETE CASCADE,
    session_date DATE NOT NULL,
    status       TEXT NOT NULL DEFAULT 'scheduled'
                 CHECK (status IN ('scheduled', 'cancelled')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_training_sessions_training_date
    ON training_sessions(training_id, session_date);

-- 6. Запись пользователя на занятие.
CREATE TABLE training_registrations (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id       BIGINT NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
    entitlement_id   BIGINT NOT NULL REFERENCES training_entitlements(id),
    status           TEXT NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active', 'cancelled')),
    reminder_sent_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at     TIMESTAMPTZ
);
-- Одна активная запись на (юзер, занятие).
CREATE UNIQUE INDEX idx_training_regs_user_session
    ON training_registrations(user_id, session_id) WHERE status = 'active';
-- Для воркера напоминаний.
CREATE INDEX idx_training_regs_reminder
    ON training_registrations(session_id)
    WHERE status = 'active' AND reminder_sent_at IS NULL;
