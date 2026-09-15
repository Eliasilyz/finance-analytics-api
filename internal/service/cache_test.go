package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

var errCacheMiss = errors.New("cache: miss")

// countingStore wraps a fakeQueryStore and tallies the read methods that
// GetAnalytics/GetTechnical depend on, so a test can prove a cache hit skips
// the recompute path entirely.
type countingStore struct {
	fake  *fakeQueryStore
	calls int
}

func (c *countingStore) GetCompany(ctx context.Context, symbol string) (models.Company, error) {
	return c.fake.GetCompany(ctx, symbol)
}
func (c *countingStore) ListCompanies(ctx context.Context) ([]models.Company, error) {
	return c.fake.ListCompanies(ctx)
}
func (c *countingStore) GetPriceHistory(ctx context.Context, symbol string, limit int) ([]models.DailyPrice, error) {
	return c.fake.GetPriceHistory(ctx, symbol, limit)
}
func (c *countingStore) GetIncomeStatements(ctx context.Context, symbol, period string) ([]models.IncomeStatement, error) {
	c.calls++
	return c.fake.GetIncomeStatements(ctx, symbol, period)
}
func (c *countingStore) GetBalanceSheets(ctx context.Context, symbol, period string) ([]models.BalanceSheet, error) {
	return c.fake.GetBalanceSheets(ctx, symbol, period)
}
func (c *countingStore) GetCashFlows(ctx context.Context, symbol, period string) ([]models.CashFlow, error) {
	return c.fake.GetCashFlows(ctx, symbol, period)
}
func (c *countingStore) GetLatestPrice(ctx context.Context, symbol string) (models.DailyPrice, error) {
	c.calls++
	return c.fake.GetLatestPrice(ctx, symbol)
}
func (c *countingStore) Screener(ctx context.Context, f models.ScreenerFilter) ([]models.ScreenerResult, error) {
	return c.fake.Screener(ctx, f)
}

// fakeCache is an in-memory Cache where errors can be forced to simulate a
// down/unavailable Redis (fail-open behaviour).
type fakeCache struct {
	mu      sync.Mutex
	entries map[string][]byte
	getErr  error
	setErr  error
}

func newFakeCache() *fakeCache { return &fakeCache{entries: map[string][]byte{}} }

func (f *fakeCache) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	if v, ok := f.entries[key]; ok {
		return v, nil
	}
	return nil, errCacheMiss
}

func (f *fakeCache) Set(_ context.Context, key string, val []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.entries[key] = val
	return nil
}

func (f *fakeCache) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.entries, k)
	}
	return nil
}

func TestGetAnalyticsCacheHitSkipsRecompute(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	p := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	counting := &countingStore{fake: &fakeQueryStore{
		company: models.Company{Symbol: "AAPL"}, hasCo: true,
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
			{Period: "annual", FiscalDate: p, Revenue: 80, NetIncome: 10, DilutedEPS: 1},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalEquity: 200, SharesOutstanding: 10},
		},
		prices: []models.DailyPrice{{Date: day, Close: 50}},
	}}

	cache := newFakeCache()
	s := NewQueryService(counting, cache)
	ctx := context.Background()

	// First call computes (store hit), second must be served from cache.
	res1, err := s.GetAnalytics(ctx, "aapl")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}
	wantCalls := counting.calls
	if wantCalls == 0 {
		t.Fatal("first call should touch the store")
	}
	res2, err := s.GetAnalytics(ctx, "aapl")
	if err != nil {
		t.Fatalf("GetAnalytics (hit): %v", err)
	}
	if counting.calls != wantCalls {
		t.Errorf("cache hit should skip recompute: calls %d, want %d", counting.calls, wantCalls)
	}
	jb1, _ := json.Marshal(res1)
	jb2, _ := json.Marshal(res2)
	if string(jb1) != string(jb2) {
		t.Errorf("cached result differs: %s vs %s", jb1, jb2)
	}

	// Cache invalidation forces a recompute.
	if err := cache.Del(ctx, "analytics:AAPL"); err != nil {
		t.Fatalf("del: %v", err)
	}
	if _, err := s.GetAnalytics(ctx, "aapl"); err != nil {
		t.Fatalf("GetAnalytics (after del): %v", err)
	}
	if counting.calls <= wantCalls {
		t.Errorf("after invalidation the store must be touched again: calls %d", counting.calls)
	}
}

func TestCacheUnavailableFailsOpen(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	p := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	counting := &countingStore{fake: &fakeQueryStore{
		company: models.Company{Symbol: "AAPL"}, hasCo: true,
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
			{Period: "annual", FiscalDate: p, Revenue: 80, NetIncome: 10, DilutedEPS: 1},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalEquity: 200, SharesOutstanding: 10},
		},
		prices: []models.DailyPrice{{Date: day, Close: 50}},
	}}

	down := newFakeCache()
	down.getErr = errCacheMiss
	down.setErr = errCacheMiss

	s := NewQueryService(counting, down)
	ctx := context.Background()

	// A dead cache must not break the endpoint: it computes straight from the
	// store every time instead of erroring or hanging.
	for i := 0; i < 3; i++ {
		res, err := s.GetAnalytics(ctx, "aapl")
		if err != nil {
			t.Fatalf("GetAnalytics with dead cache: %v", err)
		}
		if res.Symbol != "AAPL" {
			t.Errorf("symbol = %q, want AAPL", res.Symbol)
		}
	}
	if counting.calls < 3 {
		t.Errorf("dead cache should recompute every call, calls = %d", counting.calls)
	}
}
