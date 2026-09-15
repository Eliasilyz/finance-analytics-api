// Package models defines the shared domain types used across the application layers.
package models

import "time"

// Company is the normalized company information used by the ingestion pipeline.
type Company struct {
	Symbol      string
	Name        string
	Exchange    string
	Sector      string
	Industry    string
	Currency    string
	Description string
}

// DailyPrice is a single trading-day OHLCV record.
type DailyPrice struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// IncomeStatement is one fiscal-period income statement.
type IncomeStatement struct {
	Period          string // "annual" | "quarterly"
	FiscalDate      time.Time
	Revenue         float64
	GrossProfit     float64
	OperatingIncome float64
	NetIncome       float64
	DilutedEPS      float64
}

// BalanceSheet is one fiscal-period balance sheet.
type BalanceSheet struct {
	Period             string // "annual" | "quarterly"
	FiscalDate         time.Time
	TotalAssets        float64
	TotalLiabilities   float64
	TotalEquity        float64
	TotalDebt          float64
	CurrentAssets      float64
	CurrentLiabilities float64
	SharesOutstanding  float64
}

// CashFlow is one fiscal-period cash flow statement.
type CashFlow struct {
	Period            string // "annual" | "quarterly"
	FiscalDate        time.Time
	OperatingCashFlow float64
	InvestingCashFlow float64
	FinancingCashFlow float64
	NetChangeInCash   float64
}

// ScreenerFilter is the set of optional /screener criteria. Nil fields mean
// "no filter". All values are always bound as query parameters, never
// concatenated into SQL.
type ScreenerFilter struct {
	Sector           *string
	MinROE           *float64
	MaxPE            *float64
	MinRevenueGrowth *float64
}

// ScreenerResult is one /screener row. ROE, PE and RevenueGrowth are computed
// from the most recent annual reports and the latest price in SQL; pointers
// stay null when the metric is mathematically undefined for that company.
type ScreenerResult struct {
	Symbol        string
	Name          string
	Sector        string
	ROE           *float64
	PE            *float64
	RevenueGrowth *float64
}

// APIKey authenticates /api/v1 requests via the X-API-Key header. Only the
// SHA-256 digest (KeyHash) and the lookup prefix (KeyPrefix) are persisted; the
// raw token is shown exactly once at creation time.
type APIKey struct {
	ID        int64
	KeyPrefix string
	KeyHash   string
	Label     string
	CreatedAt time.Time
	ExpiresAt *time.Time
	RevokedAt *time.Time
}

// IngestionRun records one ingestion attempt so every run is auditable.
type IngestionRun struct {
	ID                int64
	Symbol            string
	Status            string // "success" | "failure"
	CompaniesInserted int
	PricesInserted    int
	IncomeInserted    int
	BalanceInserted   int
	CashFlowInserted  int
	ErrorMessage      string
	StartedAt         time.Time
	FinishedAt        time.Time
}
