// Package repository persists application data to PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// Repository provides storage for the ingestion pipeline. All writes are
// idempotent via ON CONFLICT DO NOTHING, so re-running an ingestion never
// creates duplicate rows.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps a database connection.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// UpsertCompany inserts a company or updates it in place. Returns the row id.
func (r *Repository) UpsertCompany(ctx context.Context, c models.Company) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO companies (symbol, name, exchange, sector, industry, currency, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (symbol) DO UPDATE SET
			name = EXCLUDED.name,
			exchange = EXCLUDED.exchange,
			sector = EXCLUDED.sector,
			industry = EXCLUDED.industry,
			currency = EXCLUDED.currency,
			description = EXCLUDED.description,
			updated_at = now()
		RETURNING id`,
		c.Symbol, c.Name, c.Exchange, c.Sector, c.Industry, c.Currency, c.Description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert company: %w", err)
	}
	return id, nil
}

// InsertPrices bulk-inserts daily prices, skipping conflicts. Returns the number
// of rows actually inserted.
func (r *Repository) InsertPrices(ctx context.Context, companyID int64, prices []models.DailyPrice) (int64, error) {
	if len(prices) == 0 {
		return 0, nil
	}
	const cols = 7
	vals := make([]string, 0, len(prices))
	args := make([]any, 0, len(prices)*cols)
	for i, p := range prices {
		base := i * cols
		vals = append(vals, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7))
		args = append(args, companyID, p.Date, p.Open, p.High, p.Low, p.Close, p.Volume)
	}
	query := `
		INSERT INTO price_history (company_id, date, open, high, low, close, volume)
		VALUES ` + strings.Join(vals, ",") + `
		ON CONFLICT (company_id, date) DO NOTHING`
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("insert prices: %w", err)
	}
	return res.RowsAffected()
}

// InsertIncomeStatements bulk-inserts income statements, skipping conflicts.
func (r *Repository) InsertIncomeStatements(ctx context.Context, companyID int64, stmts []models.IncomeStatement) (int64, error) {
	if len(stmts) == 0 {
		return 0, nil
	}
	const cols = 6
	vals := make([]string, 0, len(stmts))
	args := make([]any, 0, len(stmts)*cols)
	for i, s := range stmts {
		base := i * cols
		vals = append(vals, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6))
		args = append(args, companyID, s.Period, s.FiscalDate, s.Revenue, s.NetIncome, s.DilutedEPS)
	}
	query := `
		INSERT INTO income_statements (company_id, period, fiscal_date, revenue, net_income, diluted_eps)
		VALUES ` + strings.Join(vals, ",") + `
		ON CONFLICT (company_id, period, fiscal_date) DO NOTHING`
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("insert income statements: %w", err)
	}
	return res.RowsAffected()
}

// InsertBalanceSheets bulk-inserts balance sheets, skipping conflicts.
func (r *Repository) InsertBalanceSheets(ctx context.Context, companyID int64, sheets []models.BalanceSheet) (int64, error) {
	if len(sheets) == 0 {
		return 0, nil
	}
	const cols = 10
	vals := make([]string, 0, len(sheets))
	args := make([]any, 0, len(sheets)*cols)
	for i, s := range sheets {
		base := i * cols
		vals = append(vals, fmt.Sprintf(`($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)`,
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10))
		args = append(args, companyID, s.Period, s.FiscalDate,
			s.TotalAssets, s.TotalLiabilities, s.TotalEquity,
			s.TotalDebt, s.CurrentAssets, s.CurrentLiabilities, s.SharesOutstanding)
	}
	query := `
		INSERT INTO balance_sheets (company_id, period, fiscal_date, total_assets, total_liabilities, total_equity, total_debt, current_assets, current_liabilities, shares_outstanding)
		VALUES ` + strings.Join(vals, ",") + `
		ON CONFLICT (company_id, period, fiscal_date) DO NOTHING`
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("insert balance sheets: %w", err)
	}
	return res.RowsAffected()
}

// InsertCashFlows bulk-inserts cash flow statements, skipping conflicts.
func (r *Repository) InsertCashFlows(ctx context.Context, companyID int64, flows []models.CashFlow) (int64, error) {
	if len(flows) == 0 {
		return 0, nil
	}
	const cols = 7
	vals := make([]string, 0, len(flows))
	args := make([]any, 0, len(flows)*cols)
	for i, f := range flows {
		base := i * cols
		vals = append(vals, fmt.Sprintf(`($%d,$%d,$%d,$%d,$%d,$%d,$%d)`,
			base+1, base+2, base+3, base+4, base+5, base+6, base+7))
		args = append(args, companyID, f.Period, f.FiscalDate,
			f.OperatingCashFlow, f.InvestingCashFlow, f.FinancingCashFlow, f.NetChangeInCash)
	}
	query := `
		INSERT INTO cash_flow_statements (company_id, period, fiscal_date, operating_cash_flow, investing_cash_flow, financing_cash_flow, net_change_in_cash)
		VALUES ` + strings.Join(vals, ",") + `
		ON CONFLICT (company_id, period, fiscal_date) DO NOTHING`
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("insert cash flow statements: %w", err)
	}
	return res.RowsAffected()
}

// RecordRun appends an ingestion run audit record.
func (r *Repository) RecordRun(ctx context.Context, run models.IngestionRun) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ingestion_runs (symbol, status, companies_inserted, prices_inserted, income_inserted, balance_inserted, error_message, started_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		run.Symbol, run.Status,
		run.CompaniesInserted, run.PricesInserted, run.IncomeInserted, run.BalanceInserted,
		run.ErrorMessage, run.StartedAt, run.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("record ingestion run: %w", err)
	}
	return nil
}
