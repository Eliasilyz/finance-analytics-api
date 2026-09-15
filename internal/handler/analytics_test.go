package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

func TestAnalyticsHandler(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	prev := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	r := newTestRouter(&queryStoreStub{
		hasCo:   true,
		company: models.Company{Symbol: "AAPL"},
		prices:  []models.DailyPrice{{Date: day, Close: 50}},
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
			{Period: "annual", FiscalDate: prev, Revenue: 80, NetIncome: 10, DilutedEPS: 1},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalLiabilities: 300,
				TotalEquity: 200, TotalDebt: 50, CurrentAssets: 100, CurrentLiabilities: 40, SharesOutstanding: 10},
		},
	})

	w := doGet(t, r, "/api/v1/stocks/aapl/analytics")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"pe":25`) || !strings.Contains(body, `"pb":2.5`) ||
		!strings.Contains(body, `"roe":0.1`) {
		t.Fatalf("unexpected analytics body: %s", body)
	}

	missing := newTestRouter(&queryStoreStub{})
	if w := doGet(t, missing, "/api/v1/stocks/zzz/analytics"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown symbol status = %d, want 404", w.Code)
	}
}

func TestTechnicalHandlerPartialNull(t *testing.T) {
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	prices := make([]models.DailyPrice, 60)
	for i := range prices {
		prices[i] = models.DailyPrice{Date: day.AddDate(0, 0, i), Close: 100 + float64(i), Volume: 1000}
	}
	r := newTestRouter(&queryStoreStub{
		hasCo: true, company: models.Company{Symbol: "AAPL"}, prices: prices,
	})

	w := doGet(t, r, "/api/v1/stocks/aapl/technical")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"sma200":null`) {
		t.Fatalf("sma200 must be null with 60 rows, body: %s", body)
	}
	if !strings.Contains(body, `"sma20":`) || !strings.Contains(body, `"rsi14":`) {
		t.Fatalf("shorter-window indicators must be filled, body: %s", body)
	}

	// Zero rows -> 422.
	empty := newTestRouter(&queryStoreStub{hasCo: true, company: models.Company{Symbol: "AAPL"}})
	if w := doGet(t, empty, "/api/v1/stocks/aapl/technical"); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("zero rows status = %d, want 422", w.Code)
	}
}
