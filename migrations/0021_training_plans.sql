-- Кабинет тренера: роль у панельной сессии + недельные планы тренировок.

ALTER TABLE admin_sessions ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'
    CHECK (role IN ('admin', 'coach'));

-- Недельный план. groups/materials хранятся целиком в JSONB: редактор сохраняет
-- всю сетку одним сабмитом, построчные диффы не нужны.
CREATE TABLE training_plans (
    id           BIGSERIAL PRIMARY KEY,
    week_start   DATE NOT NULL UNIQUE,          -- всегда понедельник
    status       TEXT NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft', 'published')),
    groups       JSONB NOT NULL DEFAULT '[]',
    materials    JSONB NOT NULL DEFAULT '[]',
    published_at TIMESTAMPTZ,
    notified_at  TIMESTAMPTZ,
    notify_sent  INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_training_plans_published ON training_plans(week_start DESC)
    WHERE status = 'published';
