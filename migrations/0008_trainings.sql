-- Расписание тренировок: повторяющийся недельный график (по дню недели).
-- weekday по ISO: 1=Пн … 7=Вс. start_time — локальное время клуба (без таймзоны).

CREATE TABLE trainings (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    weekday     SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    start_time  TIME NOT NULL,
    place       TEXT NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_trainings_week ON trainings(weekday, start_time) WHERE is_active;
