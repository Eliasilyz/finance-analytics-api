// Package provider abstracts external financial data sources behind a single interface.
package provider

import (
	"context"
	"errors"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// Sentinel errors returned by provider implementations. The ingestion service
// maps them to run outcomes; callers can match them with errors.Is.
var (
	ErrProviderFailed = errors.New("provider: request failed")
	ErrRateLimited    = errors.New("provider: rate limit exceeded")
	ErrInvalidSymbol  = errors.New("provider: invalid symbol")
)

// CompanySnapshot is everything the ingestion pipeline needs for one symbol.
type CompanySnapshot struct {
	Company  models.Company
	Prices   []models.DailyPrice
	Income   []models.IncomeStatement
	Balance  []models.BalanceSheet
	CashFlow []models.CashFlow
}

// FinancialDataProvider fetches normalized financial data for a symbol.
// Implementations must be safe for concurrent use.
type FinancialDataProvider interface {
	FetchCompanySnapshot(ctx context.Context, symbol string) (CompanySnapshot, error)
}
