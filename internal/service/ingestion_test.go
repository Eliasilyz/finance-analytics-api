package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/provider"
	"github.com/eliasilyz/finance-analytics-api/test/mocks"
)

// fakeStore is an in-memory DataStore for unit-testing the pipeline.
type fakeStore struct {
	companyID    int64
	runs         []models.IngestionRun
	inserted     map[string]int64 // prices | income | balance | cashflow
	upsertErr    error
	insertErr    map[string]error
	recordRunErr error
	companies    []models.Company
}

func newFakeStore() *fakeStore {
	return &fakeStore{inserted: map[string]int64{}}
}

func (f *fakeStore) UpsertCompany(ctx context.Context, c models.Company) (int64, error) {
	if f.upsertErr != nil {
		return 0, f.upsertErr
	}
	f.companies = append(f.companies, c)
	return f.companyID, nil
}

func (f *fakeStore) InsertPrices(_ context.Context, _ int64, _ []models.DailyPrice) (int64, error) {
	if err := f.insertErr["prices"]; err != nil {
		return 0, err
	}
	return f.inserted["prices"], nil
}

func (f *fakeStore) InsertIncomeStatements(_ context.Context, _ int64, _ []models.IncomeStatement) (int64, error) {
	if err := f.insertErr["income"]; err != nil {
		return 0, err
	}
	return f.inserted["income"], nil
}

func (f *fakeStore) InsertBalanceSheets(_ context.Context, _ int64, _ []models.BalanceSheet) (int64, error) {
	if err := f.insertErr["balance"]; err != nil {
		return 0, err
	}
	return f.inserted["balance"], nil
}

func (f *fakeStore) InsertCashFlows(_ context.Context, _ int64, _ []models.CashFlow) (int64, error) {
	if err := f.insertErr["cashflow"]; err != nil {
		return 0, err
	}
	return f.inserted["cashflow"], nil
}

func (f *fakeStore) RecordRun(_ context.Context, run models.IngestionRun) error {
	if f.recordRunErr != nil {
		return f.recordRunErr
	}
	f.runs = append(f.runs, run)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validSnapshot() provider.CompanySnapshot {
	return provider.CompanySnapshot{
		Company: models.Company{Symbol: "AAPL", Name: "Apple Inc.", Currency: "USD"},
		Prices: []models.DailyPrice{
			{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 180, High: 185, Low: 179, Close: 184, Volume: 50000000},
			{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Open: 184, High: 190, Low: 183, Close: 189, Volume: 60000000},
		},
		Income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), Revenue: 383285000000, NetIncome: 96995000000},
		},
		Balance: []models.BalanceSheet{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), TotalAssets: 352583000000},
		},
		CashFlow: []models.CashFlow{
			{Period: "annual", FiscalDate: time.Date(2023, 9, 30, 0, 0, 0, 0, time.UTC), OperatingCashFlow: 110543000000, NetChangeInCash: -73000000},
		},
	}
}

func TestIngestSuccess(t *testing.T) {
	store := newFakeStore()
	store.companyID = 42
	store.inserted["prices"] = 2
	store.inserted["income"] = 1
	store.inserted["balance"] = 1
	store.inserted["cashflow"] = 1

	svc := NewIngestionService(&mocks.MockProvider{Snapshot: validSnapshot()}, store, testLogger())
	run, err := svc.Ingest(context.Background(), "aapl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "success" {
		t.Errorf("expected success run, got %q", run.Status)
	}
	if run.Symbol != "AAPL" {
		t.Errorf("expected normalized symbol AAPL, got %q", run.Symbol)
	}
	if run.PricesInserted != 2 || run.IncomeInserted != 1 || run.BalanceInserted != 1 || run.CashFlowInserted != 1 {
		t.Errorf("unexpected insert counts: %+v", run)
	}
	if len(store.companies) != 1 || store.companies[0].Symbol != "AAPL" {
		t.Errorf("company not upserted: %+v", store.companies)
	}
}

func TestIngestProviderError(t *testing.T) {
	store := newFakeStore()
	mp := &mocks.MockProvider{Err: errors.New("connection refused")}
	svc := NewIngestionService(mp, store, testLogger())

	run, err := svc.Ingest(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if run.Status != "failure" {
		t.Errorf("expected failure run, got %q", run.Status)
	}
	if run.ErrorMessage == "" {
		t.Error("expected error message on failed run")
	}
	if len(store.runs) != 1 || store.runs[0].Status != "failure" {
		t.Errorf("expected 1 recorded failure run: %+v", store.runs)
	}
}

func TestIngestProviderTimeout(t *testing.T) {
	store := newFakeStore()
	mp := &mocks.MockProvider{Err: context.DeadlineExceeded}
	svc := NewIngestionService(mp, store, testLogger())

	_, err := svc.Ingest(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded to be wrapped, got %v", err)
	}
}

func TestIngestRateLimit(t *testing.T) {
	store := newFakeStore()
	mp := &mocks.MockProvider{Err: provider.ErrRateLimited}
	svc := NewIngestionService(mp, store, testLogger())

	_, err := svc.Ingest(context.Background(), "AAPL")
	if !errors.Is(err, provider.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if len(store.runs) != 1 || store.runs[0].Status != "failure" {
		t.Errorf("rate limit should record a failure run: %+v", store.runs)
	}
}

func TestIngestInvalidData(t *testing.T) {
	store := newFakeStore()
	bad := validSnapshot()
	// All prices malformed: forced to zero, negative volume, inverted high/low.
	bad.Prices = []models.DailyPrice{
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 0, High: 10, Low: 1, Close: 5, Volume: 5},
		{Date: time.Time{}, Open: 180, High: 185, Low: 179, Close: 184, Volume: -5},
	}
	svc := NewIngestionService(&mocks.MockProvider{Snapshot: bad}, store, testLogger())

	run, err := svc.Ingest(context.Background(), "AAPL")
	if !errors.Is(err, ErrNoPrices) {
		t.Fatalf("expected ErrNoPrices, got %v", err)
	}
	if run.Status != "failure" {
		t.Errorf("expected failure for invalid data, got %q", run.Status)
	}
}

func TestIngestNormalizesInvalidRows(t *testing.T) {
	store := newFakeStore()
	store.inserted["prices"] = 1
	store.inserted["income"] = 1
	store.inserted["balance"] = 1
	store.inserted["cashflow"] = 1
	snap := validSnapshot()
	// One good row + one malformed row; malformed must be dropped, not fail.
	snap.Prices = append(snap.Prices, models.DailyPrice{
		Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 0, High: 0, Low: 0, Close: 0, Volume: 0,
	})
	snap.Income = append(snap.Income, models.IncomeStatement{Period: "bogus", FiscalDate: time.Time{}})
	snap.CashFlow = append(snap.CashFlow, models.CashFlow{Period: "bogus", FiscalDate: time.Time{}})

	svc := NewIngestionService(&mocks.MockProvider{Snapshot: snap}, store, testLogger())
	run, err := svc.Ingest(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "success" {
		t.Errorf("expected success, got %q", run.Status)
	}
	if run.PricesInserted != 1 {
		t.Errorf("expected valid rows to persist, prices_inserted=%d", run.PricesInserted)
	}
}

func TestIngestDuplicateIsIdempotent(t *testing.T) {
	store := newFakeStore()
	store.companyID = 7
	store.inserted["prices"] = 0 // previously stored -> nothing new on re-run
	svc := NewIngestionService(&mocks.MockProvider{Snapshot: validSnapshot()}, store, testLogger())

	run, err := svc.Ingest(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "success" {
		t.Errorf("re-ingesting stored data should succeed, got %q", run.Status)
	}
	if run.PricesInserted != 0 {
		t.Errorf("expected 0 new prices on duplicate ingest, got %d", run.PricesInserted)
	}
	if len(store.runs) != 1 {
		t.Errorf("expected 1 recorded run, got %d", len(store.runs))
	}
}

func TestIngestInvalidSymbol(t *testing.T) {
	store := newFakeStore()
	svc := NewIngestionService(&mocks.MockProvider{}, store, testLogger())
	for _, bad := range []string{"", "AAPL TOO LONG", "AA$PL", "AAPL;DROP"} {
		if _, err := svc.Ingest(context.Background(), bad); !errors.Is(err, ErrInvalidSymbol) {
			t.Errorf("symbol %q: expected ErrInvalidSymbol, got %v", bad, err)
		}
	}
}

func TestIngestDatabaseFailure(t *testing.T) {
	store := newFakeStore()
	store.insertErr = map[string]error{"prices": errors.New("connection lost")}
	svc := NewIngestionService(&mocks.MockProvider{Snapshot: validSnapshot()}, store, testLogger())

	run, err := svc.Ingest(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("expected database error to propagate")
	}
	if run.Status != "failure" {
		t.Errorf("expected failure run on db error, got %q", run.Status)
	}
}
