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
