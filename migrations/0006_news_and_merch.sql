-- Клубные новости и мерч.

CREATE TABLE news (
    id            BIGSERIAL PRIMARY KEY,
    title         TEXT NOT NULL,
    content       TEXT NOT NULL,
    sort_order    INT NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    published_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE merch (
    id            BIGSERIAL PRIMARY KEY,
    title         TEXT NOT NULL,
    description   TEXT,
    price_kop     BIGINT NOT NULL,
    sort_order    INT NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);