//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/provider"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/eliasilyz/finance-analytics-api/test/mocks"
)

func TestIngestPipelineEndToEnd(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	snap := provider.CompanySnapshot{
		Company: models.Company{
			Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ",
			Sector: "Technology", Industry: "Consumer Electronics",
			Currency: "USD", Description: "Designs and sells devices.",
		},
		Prices: []models.DailyPrice{
			{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 188.4, High: 190.2, Low: 187.9, Close: 190.0, Volume: 68451234},
			{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Open: 190.5, High: 192.6, Low: 189.7, Close: 191.2, Volume: 71234567},
		},
		Income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), Revenue: 383285000000, NetIncome: 96995000000, DilutedEPS: 6.16},
		},
		Balance: []models.BalanceSheet{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), TotalAssets: 352583000000, TotalEquity: 62146000000},
		},
		CashFlow: []models.CashFlow{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), OperatingCashFlow: 110543000000, NetChangeInCash: -73300000},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewIngestionService(
		&mocks.MockProvider{Snapshot: snap},
		repository.NewRepository(db),
		logger,
	)

	ctx := context.Background()

	// First run persists everything.
	run, err := svc.Ingest(ctx, "aapl")
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}
	if run.Status != "success" {
		t.Fatalf("expected success run, got %q", run.Status)
	}

	var (
		companyID            int64
		priceCount, incCount int
		cfCount              int
	)
	if err := db.QueryRow(`SELECT id FROM companies WHERE symbol = 'AAPL'`).Scan(&companyID); err != nil {
		t.Fatalf("company not persisted: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM price_history WHERE company_id = $1`, companyID).Scan(&priceCount); err != nil {
		t.Fatalf("count prices: %v", err)
	}
	if priceCount != 2 {
		t.Errorf("expected 2 prices, got %d", priceCount)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM income_statements WHERE company_id = $1`, companyID).Scan(&incCount); err != nil {
		t.Fatalf("count income: %v", err)
	}
	if incCount != 1 {
		t.Errorf("expected 1 income row, got %d", incCount)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM cash_flow_statements WHERE company_id = $1`, companyID).Scan(&cfCount); err != nil {
		t.Fatalf("count cash flow: %v", err)
	}
	if cfCount != 1 {
		t.Errorf("expected 1 cash flow row, got %d", cfCount)
	}

	// Second run is idempotent: same data already stored -> no new rows.
	run2, err := svc.Ingest(ctx, "AAPL")
	if err != nil {
		t.Fatalf("re-ingest failed: %v", err)
	}
	if run2.Status != "success" {
		t.Fatalf("re-ingest expected success, got %q", run2.Status)
	}
	if run2.PricesInserted != 0 {
		t.Errorf("expected 0 new prices on duplicate, got %d", run2.PricesInserted)
	}
	// Company still upserted once; prices total unchanged.
	var totalPriceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM price_history WHERE company_id = $1`, companyID).Scan(&totalPriceCount); err != nil {
		t.Fatalf("recount prices: %v", err)
	}
	if totalPriceCount != 2 {
		t.Errorf("idempotency broken: prices went from 2 to %d", totalPriceCount)
	}

	// Every attempt recorded in ingestion_runs.
	var runCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ingestion_runs WHERE symbol = 'AAPL'`).Scan(&runCount); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runCount != 2 {
		t.Errorf("expected 2 ingestion runs recorded, got %d", runCount)
	}
}
