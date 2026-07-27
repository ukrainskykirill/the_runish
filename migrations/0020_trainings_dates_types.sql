-- Переход тренировок на конкретные даты вместо еженедельного повтора,
-- плюс тип тренировки (regular | sunday_runish).

ALTER TABLE trainings ADD COLUMN training_date DATE;
ALTER TABLE trainings ADD COLUMN kind TEXT NOT NULL DEFAULT 'regular'
    CHECK (kind IN ('regular', 'sunday_runish'));

-- Бэкфилл: каждой существующей тренировке — ближайшая будущая дата её дня недели
-- (сегодня допускается, если день недели совпадает).
UPDATE trainings
SET training_date = current_date
    + ((weekday - EXTRACT(ISODOW FROM current_date)::int + 7) % 7);

ALTER TABLE trainings ALTER COLUMN training_date SET NOT NULL;

CREATE INDEX idx_trainings_date ON trainings(training_date, start_time) WHERE is_active;

-- Запись без энтайтлмента (первая бесплатная / Sunday Runish) — взнос и подписка не требуются.
ALTER TABLE training_registrations ALTER COLUMN entitlement_id DROP NOT NULL;
