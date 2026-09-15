-- 000002_ingestion_runs.up.sql
-- Audit trail for ingestion attempts. Every run (success or failure) is recorded
-- so ingestion outcomes are observable without reading application logs.

CREATE TABLE ingestion_runs (
    id                  BIGSERIAL PRIMARY KEY,
    symbol              VARCHAR(10) NOT NULL,
    status              VARCHAR(20) NOT NULL,  -- 'success' | 'failure'
    companies_inserted  INT         NOT NULL DEFAULT 0,
    prices_inserted     INT         NOT NULL DEFAULT 0,
    income_inserted     INT         NOT NULL DEFAULT 0,
    balance_inserted    INT         NOT NULL DEFAULT 0,
    error_message       TEXT,
    started_at          TIMESTAMPTZ NOT NULL,
    finished_at         TIMESTAMPTZ NOT NULL
);