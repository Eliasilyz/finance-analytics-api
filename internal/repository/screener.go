package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// Screener runs the screener query. The WHERE clause is assembled on the
// computed columns of a subquery; every criterion value travels through a $N
// placeholder (models.ScreenerFilter), so the dynamic part is injection-safe
// by construction: no user-supplied text is ever spliced into the SQL.
func (r *Repository) Screener(ctx context.Context, f models.ScreenerFilter) ([]models.ScreenerResult, error) {
	const base = `
		WITH base AS (
			SELECT c.symbol, c.name, c.sector,
				CASE WHEN b.total_equity > 0 AND i.net_income IS NOT NULL
					THEN i.net_income / b.total_equity END AS roe,
				CASE WHEN i.diluted_eps > 0 AND p.close IS NOT NULL
					THEN p.close / i.diluted_eps END AS pe,
				CASE WHEN pi.prev_revenue > 0
					THEN (i.revenue - pi.prev_revenue) / pi.prev_revenue END AS revenue_growth
			FROM companies c
			JOIN LATERAL (
				SELECT revenue, net_income, diluted_eps
				FROM income_statements
				WHERE company_id = c.id AND period = 'annual'
				ORDER BY fiscal_date DESC LIMIT 1
			) i ON TRUE
			LEFT JOIN LATERAL (
				SELECT revenue AS prev_revenue
				FROM income_statements
				WHERE company_id = c.id AND period = 'annual'
				  AND fiscal_date < (
						SELECT COALESCE(MAX(fiscal_date), '-infinity')
						FROM income_statements
						WHERE company_id = c.id AND period = 'annual'
					)
				ORDER BY fiscal_date DESC LIMIT 1
			) pi ON TRUE
			LEFT JOIN LATERAL (
				SELECT total_equity
				FROM balance_sheets
				WHERE company_id = c.id AND period = 'annual'
				ORDER BY fiscal_date DESC LIMIT 1
			) b ON TRUE
			LEFT JOIN LATERAL (
				SELECT close
				FROM price_history
				WHERE company_id = c.id
				ORDER BY date DESC LIMIT 1
			) p ON TRUE
		)`

	query := base + ` SELECT symbol, name, sector, roe, pe, revenue_growth FROM base`
	args := []any{}
	where := make([]string, 0)

	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}

	if f.Sector != nil && *f.Sector != "" {
		add("sector = $%d", *f.Sector)
	}
	if f.MinROE != nil {
		add("roe >= $%d", *f.MinROE)
	}
	if f.MaxPE != nil {
		add("pe <= $%d", *f.MaxPE)
	}
	if f.MinRevenueGrowth != nil {
		add("revenue_growth >= $%d", *f.MinRevenueGrowth)
	}

	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY symbol"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("screener: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []models.ScreenerResult{}
	for rows.Next() {
		var s models.ScreenerResult
		if err := rows.Scan(&s.Symbol, &s.Name, &s.Sector, &s.ROE, &s.PE, &s.RevenueGrowth); err != nil {
			return nil, fmt.Errorf("screener scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
