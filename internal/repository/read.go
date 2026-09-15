package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// GetCompany returns one company by symbol. Returns sql.ErrNoRows when the
// symbol is not present.
func (r *Repository) GetCompany(ctx context.Context, symbol string) (models.Company, error) {
	var c models.Company
	var desc sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT symbol, name, exchange, sector, industry, currency, description
		FROM companies WHERE symbol = $1`,
		symbol,
	).Scan(&c.Symbol, &c.Name, &c.Exchange, &c.Sector, &c.Industry, &c.Currency, &desc)
	if err != nil {
		return models.Company{}, fmt.Errorf("get company: %w", err)
	}
	c.Description = desc.String
	return c, nil
}

// ListCompanies returns all companies ordered by symbol.
func (r *Repository) ListCompanies(ctx context.Context) ([]models.Company, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol, name, exchange, sector, industry, currency, description
		FROM companies ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("list companies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.Company
	for rows.Next() {
		var c models.Company
		var desc sql.NullString
		if err := rows.Scan(&c.Symbol, &c.Name, &c.Exchange, &c.Sector,
			&c.Industry, &c.Currency, &desc); err != nil {
			return nil, fmt.Errorf("scan company: %w", err)
		}
		c.Description = desc.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetPriceHistory returns daily prices for a company, oldest first. limit <= 0
// means "all rows". Returns sql.ErrNoRows when the company has no prices.
func (r *Repository) GetPriceHistory(ctx context.Context, symbol string, limit int) ([]models.DailyPrice, error) {
	query := `
		SELECT date, open, high, low, close, volume
		FROM price_history ph
		JOIN companies c ON c.id = ph.company_id
		WHERE c.symbol = $1
		ORDER BY ph.date ASC`
	args := []any{symbol}
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.DailyPrice
	for rows.Next() {
		var p models.DailyPrice
		if err := rows.Scan(&p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.Volume); err != nil {
			return nil, fmt.Errorf("scan price: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, sql.ErrNoRows
	}
	return out, nil
}

// GetLatestPrice returns the most recent daily price for a company.
func (r *Repository) GetLatestPrice(ctx context.Context, symbol string) (models.DailyPrice, error) {
	var p models.DailyPrice
	err := r.db.QueryRowContext(ctx, `
		SELECT date, open, high, low, close, volume
		FROM price_history ph
		JOIN companies c ON c.id = ph.company_id
		WHERE c.symbol = $1
		ORDER BY ph.date DESC
		LIMIT 1`, symbol,
	).Scan(&p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.Volume)
	if err != nil {
		return models.DailyPrice{}, fmt.Errorf("get latest price: %w", err)
	}
	return p, nil
}

// queryStatements runs a parameterized select against a statement table.
// fields, table and period are fixed constants chosen by the caller (period
// is validated by the handler); only $N placeholders carry user input.
func queryStatements[T any](ctx context.Context, r *Repository, symbol,
	fields, table, period string,
	scan func(*sql.Rows) (T, error),
) ([]T, error) {
	query := fmt.Sprintf("SELECT %s FROM %s st JOIN companies c ON c.id = st.company_id WHERE c.symbol = $1",
		fields, table)
	args := []any{symbol}
	if period != "" && period != "all" {
		query += ` AND st.period = $2`
		args = append(args, period)
	}
	query += " ORDER BY st.fiscal_date DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query statements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, sql.ErrNoRows
	}
	return out, nil
}

// GetIncomeStatements returns income statements for a symbol, newest first.
// period is "", "all", "annual" or "quarterly".
func (r *Repository) GetIncomeStatements(ctx context.Context, symbol, period string) ([]models.IncomeStatement, error) {
	scan := func(row *sql.Rows) (models.IncomeStatement, error) {
		var s models.IncomeStatement
		var fd time.Time
		if err := row.Scan(&s.Period, &fd, &s.Revenue, &s.NetIncome, &s.DilutedEPS); err != nil {
			return models.IncomeStatement{}, fmt.Errorf("scan income statement: %w", err)
		}
		s.FiscalDate = fd
		return s, nil
	}
	return queryStatements(ctx, r, symbol,
		"st.period, st.fiscal_date, st.revenue, st.net_income, st.diluted_eps",
		"income_statements", period, scan)
}

// GetBalanceSheets returns balance sheets for a symbol, newest first.
func (r *Repository) GetBalanceSheets(ctx context.Context, symbol, period string) ([]models.BalanceSheet, error) {
	scan := func(row *sql.Rows) (models.BalanceSheet, error) {
		var s models.BalanceSheet
		var fd time.Time
		if err := row.Scan(&s.Period, &fd, &s.TotalAssets, &s.TotalLiabilities,
			&s.TotalEquity, &s.TotalDebt, &s.CurrentAssets, &s.CurrentLiabilities,
			&s.SharesOutstanding); err != nil {
			return models.BalanceSheet{}, fmt.Errorf("scan balance sheet: %w", err)
		}
		s.FiscalDate = fd
		return s, nil
	}
	return queryStatements(ctx, r, symbol,
		"st.period, st.fiscal_date, st.total_assets, st.total_liabilities, st.total_equity, st.total_debt, st.current_assets, st.current_liabilities, st.shares_outstanding",
		"balance_sheets", period, scan)
}

// GetCashFlows returns cash flow statements for a symbol, newest first.
func (r *Repository) GetCashFlows(ctx context.Context, symbol, period string) ([]models.CashFlow, error) {
	scan := func(row *sql.Rows) (models.CashFlow, error) {
		var f models.CashFlow
		var fd time.Time
		if err := row.Scan(&f.Period, &fd, &f.OperatingCashFlow, &f.InvestingCashFlow,
			&f.FinancingCashFlow, &f.NetChangeInCash); err != nil {
			return models.CashFlow{}, fmt.Errorf("scan cash flow: %w", err)
		}
		f.FiscalDate = fd
		return f, nil
	}
	return queryStatements(ctx, r, symbol,
		"st.period, st.fiscal_date, st.operating_cash_flow, st.investing_cash_flow, st.financing_cash_flow, st.net_change_in_cash",
		"cash_flow_statements", period, scan)
}
