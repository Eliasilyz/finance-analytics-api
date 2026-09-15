package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

func TestGetAnalyticsPartialNull(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	p := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	store := &fakeQueryStore{
		company: models.Company{Symbol: "AAPL"},
		hasCo:   true,
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
			{Period: "annual", FiscalDate: p, Revenue: 80, NetIncome: 10, DilutedEPS: 1},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalLiabilities: 300,
				TotalEquity: 200, TotalDebt: 50, CurrentAssets: 100, CurrentLiabilities: 40, SharesOutstanding: 10},
		},
		prices: []models.DailyPrice{
			{Date: day, Close: 50},
		},
	}
	s := NewQueryService(store)
	ctx := context.Background()

	res, err := s.GetAnalytics(ctx, "aapl")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}
	// PE = 50/2 = 25; PB = 50/(200/10) = 2.5; ROE = 20/200 = 0.10
	if res.Metrics.PERatio == nil || *res.Metrics.PERatio != 25 {
		t.Errorf("PE = %v, want 25", res.Metrics.PERatio)
	}
	if res.Metrics.PriceToBook == nil || *res.Metrics.PriceToBook != 2.5 {
		t.Errorf("PB = %v, want 2.5", res.Metrics.PriceToBook)
	}
	if res.Metrics.RevenueGrowth == nil || *res.Metrics.RevenueGrowth != 0.25 {
		t.Errorf("revenue growth = %v, want 0.25", res.Metrics.RevenueGrowth)
	}

	// Single income statement -> growth metrics stay null (partial-null).
	store.income = store.income[:1]
	res, err = s.GetAnalytics(ctx, "aapl")
	if err != nil {
		t.Fatalf("GetAnalytics: %v", err)
	}
	if res.Metrics.RevenueGrowth != nil || res.Metrics.EarningsGrowth != nil {
		t.Errorf("growth should be null with one period: %+v", res.Metrics)
	}
	if res.Metrics.PERatio == nil {
		t.Errorf("PE should still be filled: %+v", res.Metrics)
	}

	// No data at all -> ErrInsufficientData.
	empty := NewQueryService(&fakeQueryStore{company: models.Company{Symbol: "AAPL"}, hasCo: true})
	if _, err := empty.GetAnalytics(ctx, "aapl"); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("want ErrInsufficientData, got %v", err)
	}

	// Unknown symbol -> ErrNotFound.
	noc := NewQueryService(&fakeQueryStore{})
	if _, err := noc.GetAnalytics(ctx, "aapl"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetTechnicalPartialNull(t *testing.T) {
	// 60 daily closes: enough for SMA20/50, RSI14, EMA20, volatility, but
	// NOT for SMA200.
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	prices := make([]models.DailyPrice, 60)
	for i := range prices {
		prices[i] = models.DailyPrice{Date: day.AddDate(0, 0, i), Close: 100 + float64(i), Volume: 1000}
	}
	store := &fakeQueryStore{
		company: models.Company{Symbol: "AAPL"}, hasCo: true, prices: prices,
	}
	s := NewQueryService(store)
	ctx := context.Background()

	res, err := s.GetTechnical(ctx, "aapl")
	if err != nil {
		t.Fatalf("GetTechnical: %v", err)
	}
	if res.Values.SMA200 != nil {
		t.Errorf("SMA200 should be null with 60 rows (need 200), got %v", *res.Values.SMA200)
	}
	if res.Values.DailyReturn == nil || res.Values.SMA20 == nil || res.Values.SMA50 == nil ||
		res.Values.EMA20 == nil || res.Values.RSI14 == nil || res.Values.Volatility20 == nil ||
		res.Values.AverageVolume20 == nil {
		t.Errorf("indicators computable from 60 rows must be filled: %+v", res.Values)
	}

	// Zero rows -> ErrInsufficientData, never partial.
	empty := NewQueryService(&fakeQueryStore{company: models.Company{Symbol: "AAPL"}, hasCo: true})
	if _, err := empty.GetTechnical(ctx, "aapl"); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("want ErrInsufficientData, got %v", err)
	}
}
