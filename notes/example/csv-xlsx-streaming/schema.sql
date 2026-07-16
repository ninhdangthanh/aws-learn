-- Bảng demo. Cột đủ nhiều để 1 dòng ~ 150-200 bytes -> dễ đạt ~1GB với vài triệu dòng.
CREATE TABLE IF NOT EXISTS users (
    id          BIGSERIAL PRIMARY KEY,
    uuid        UUID        NOT NULL,
    first_name  TEXT        NOT NULL,
    last_name   TEXT        NOT NULL,
    email       TEXT        NOT NULL,
    age         INT         NOT NULL,
    country     TEXT        NOT NULL,
    balance     NUMERIC(12,2) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);
