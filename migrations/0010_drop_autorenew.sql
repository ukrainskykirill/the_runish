-- Автопродление убрано из продукта целиком.
ALTER TABLE users DROP COLUMN IF EXISTS sub_autorenew;
