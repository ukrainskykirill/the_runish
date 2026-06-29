-- Переработка системы подписок: вступительный взнос, скидки, льгота «первых 30».
-- Данные каталога здесь НЕ сидим — продукты вставляются отдельным прямым INSERT
-- (scripts/seed_catalog_v2.sql). Здесь только схема.

-- Взнос: флаг хранится у юзера навсегда. sub_autorenew — согласие на автопродление
-- (биллинг/рекуррентка — отдельной задачей).
ALTER TABLE users ADD COLUMN entry_fee_paid BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN sub_autorenew  BOOLEAN NOT NULL DEFAULT false;

-- Скидки на услуге: цена «при подписке» и льготная цена «первых 30». NULL = не применяется.
ALTER TABLE services ADD COLUMN price_with_sub_kop BIGINT;
ALTER TABLE services ADD COLUMN promo_price_kop    BIGINT;

-- Простой key-value для настроек (переключатель акции «первых 30» и т.п.).
CREATE TABLE app_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO app_settings (key, value) VALUES ('first30_promo_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
