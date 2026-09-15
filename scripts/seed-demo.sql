-- scripts/seed-demo.sql
-- Manual demo seed: a few well-known companies with 60 trading days of OHLCV
-- and one fiscal year of statements, so dashboard/visualization endpoints have
-- data to render without consuming Alpha Vantage quota.
-- Run: docker compose exec -T postgres psql -U postgres -d finance_db -f /tmp/seed-demo.sql
-- The table `companies`/rows are NOT ingestion-audited (no ingestion_runs row).

BEGIN;

INSERT INTO companies (symbol, name, exchange, sector, industry, currency, description) VALUES
  ('AAPL', 'Apple Inc.',               'NASDAQ', 'Technology', 'Consumer Electronics',  'USD', 'Designs and sells devices, software, and services.'),
  ('MSFT', 'Microsoft Corporation',    'NASDAQ', 'Technology', 'Software - Infrastructure', 'USD', 'Develops software, cloud, and hardware products.'),
  ('NVDA', 'NVIDIA Corporation',       'NASDAQ', 'Technology', 'Semiconductors',       'USD', 'Designs graphics and accelerated computing processors.')
ON CONFLICT (symbol) DO NOTHING;

-- ~60 trading days ending 2026-09-15, deterministic pseudo-random drift.
WITH c AS (
    SELECT id, symbol,
           CASE symbol WHEN 'AAPL' THEN 215.0 WHEN 'MSFT' THEN 430.0 ELSE 135.0 END AS base,
           row_number() OVER (ORDER BY symbol) - 1 AS idx
    FROM companies
    WHERE symbol IN ('AAPL', 'MSFT', 'NVDA')
),
price_days AS (
    SELECT d::date AS date,
           row_number() OVER (ORDER BY d::date) - 1 AS g
    FROM generate_series(DATE '2026-07-01', DATE '2026-09-15', interval '1 day') d
    WHERE extract(dow FROM d) BETWEEN 1 AND 5
)
INSERT INTO price_history (company_id, date, open, high, low, close, volume)
SELECT c.id, pd.date,
       round((c.base + pd.g*0.13 + mod(c.idx*3 + pd.g*5, 17) - 8)::numeric, 2),
       round((c.base + pd.g*0.13 + mod(c.idx*3 + pd.g*5, 17) - 7 + mod(pd.g, 4)*0.1)::numeric, 2),
       round((c.base + pd.g*0.13 + mod(c.idx*3 + pd.g*5, 17) - 9 - mod(pd.g, 3)*0.1)::numeric, 2),
       round((c.base + pd.g*0.13 + mod(c.idx*3 + pd.g*5, 17) - 8)::numeric, 2),
       (4000000 + mod(pd.g*98765 + c.idx*31, 3500000))::bigint
FROM c, price_days pd;

INSERT INTO income_statements (company_id, period, fiscal_date, revenue, gross_profit, operating_income, net_income, diluted_eps) VALUES
  -- annual FY2024
  ((SELECT id FROM companies WHERE symbol='AAPL'), 'annual', '2024-09-30', 391035000000, 169148000000, 115752000000,  93736000000, 6.08),
  ((SELECT id FROM companies WHERE symbol='MSFT'), 'annual', '2024-09-30', 245122000000, 153228000000, 109433000000,  88136000000, 11.86),
  ((SELECT id FROM companies WHERE symbol='NVDA'), 'annual', '2024-09-30',  60922000000,  33835000000,  16653000000,  29760000000, 1.20),
  -- annual FY2023 (for growth metrics)
  ((SELECT id FROM companies WHERE symbol='AAPL'), 'annual', '2023-09-30', 383285000000, 169148000000, 114301000000,  96995000000, 6.16),
  ((SELECT id FROM companies WHERE symbol='MSFT'), 'annual', '2023-09-30', 211915000000, 135062000000,  88523000000,  72361000000, 9.74),
  ((SELECT id FROM companies WHERE symbol='NVDA'), 'annual', '2023-09-30',  26974000000,  15334000000,   8214000000,   4368000000, 0.38),
  -- latest quarter
  ((SELECT id FROM companies WHERE symbol='AAPL'), 'quarterly', '2025-06-30',  90753000000,  38959000000,  27511000000,  23636000000, 1.53),
  ((SELECT id FROM companies WHERE symbol='MSFT'), 'quarterly', '2025-06-30',  64700000000,  44773000000,  30186000000,  23774000000, 3.20),
  ((SELECT id FROM companies WHERE symbol='NVDA'), 'quarterly', '2025-04-30',  35209000000,  24210000000,  18647000000,  19530000000, 0.79);

INSERT INTO balance_sheets (company_id, period, fiscal_date, total_assets, total_liabilities, total_equity, total_debt, current_assets, current_liabilities, shares_outstanding) VALUES
  ((SELECT id FROM companies WHERE symbol='AAPL'), 'annual', '2024-09-30', 364980000000, 279340000000,  85640000000, 121280000000, 146360000000, 151300000000, 15156060000),
  ((SELECT id FROM companies WHERE symbol='MSFT'), 'annual', '2024-09-30', 512163000000, 253482000000, 258681000000,  74629000000, 143178000000,  90923000000,  7431000000),
  ((SELECT id FROM companies WHERE symbol='NVDA'), 'annual', '2024-09-30', 111849000000,  47244000000,  64605000000,  12508000000,  78662000000,  31305000000, 24452000000);

INSERT INTO cash_flow_statements (company_id, period, fiscal_date, operating_cash_flow, investing_cash_flow, financing_cash_flow, net_change_in_cash) VALUES
  ((SELECT id FROM companies WHERE symbol='AAPL'), 'annual', '2024-09-30', 118254000000, -33328000000, -114147000000,  -2921000000),
  ((SELECT id FROM companies WHERE symbol='MSFT'), 'annual', '2024-09-30', 118539000000, -63779000000,  -48018000000,   6947000000),
  ((SELECT id FROM companies WHERE symbol='NVDA'), 'annual', '2024-09-30',  61464000000, -32391000000,  -24435000000,   4657000000);

COMMIT;