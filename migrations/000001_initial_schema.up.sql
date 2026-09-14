-- 000001_initial_schema.up.sql
-- Core entities for financial data. Columns chosen to match actual Alpha Vantage
-- response fields (annualReports + quarterlyReports) and the metrics consumed by
-- the analytics engine. Nothing speculative.

CREATE TABLE companies (
    id           BIGSERIAL PRIMARY KEY,
    symbol       VARCHAR(10)  NOT NULL UNIQUE,
    name         VARCHAR(255) NOT NULL,
    exchange     VARCHAR(50),
    sector       VARCHAR(100),
    industry     VARCHAR(100),
    currency     VARCHAR(10)  NOT NULL DEFAULT 'USD',
    description  TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE price_history (
    id          BIGSERIAL PRIMARY KEY,
    company_id  BIGINT       NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    date        DATE         NOT NULL,
    open        NUMERIC(18,4),
    high        NUMERIC(18,4),
    low         NUMERIC(18,4),
    close       NUMERIC(18,4),
    volume      BIGINT,
    UNIQUE (company_id, date)
);

CREATE INDEX idx_price_history_company_date ON price_history (company_id, date DESC);

CREATE TABLE income_statements (
    id               BIGSERIAL PRIMARY KEY,
    company_id       BIGINT      NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    period           VARCHAR(10) NOT NULL,              -- 'annual' | 'quarterly'
    fiscal_date      DATE        NOT NULL,
    revenue          NUMERIC(20,2),
    gross_profit     NUMERIC(20,2),
    operating_income NUMERIC(20,2),
    net_income       NUMERIC(20,2),
    diluted_eps      NUMERIC(18,4),
    UNIQUE (company_id, period, fiscal_date)
);

CREATE TABLE balance_sheets (
    id                   BIGSERIAL PRIMARY KEY,
    company_id           BIGINT      NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    period               VARCHAR(10) NOT NULL,          -- 'annual' | 'quarterly'
    fiscal_date          DATE        NOT NULL,
    total_assets         NUMERIC(20,2),
    total_liabilities    NUMERIC(20,2),
    total_equity         NUMERIC(20,2),
    total_debt           NUMERIC(20,2),
    current_assets       NUMERIC(20,2),
    current_liabilities  NUMERIC(20,2),
    shares_outstanding   NUMERIC(20,2),
    UNIQUE (company_id, period, fiscal_date)
);

CREATE TABLE cash_flow_statements (
    id                    BIGSERIAL PRIMARY KEY,
    company_id            BIGINT      NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    period                VARCHAR(10) NOT NULL,          -- 'annual' | 'quarterly'
    fiscal_date           DATE        NOT NULL,
    operating_cash_flow   NUMERIC(20,2),
    investing_cash_flow   NUMERIC(20,2),
    financing_cash_flow   NUMERIC(20,2),
    net_change_in_cash    NUMERIC(20,2),
    UNIQUE (company_id, period, fiscal_date)
);