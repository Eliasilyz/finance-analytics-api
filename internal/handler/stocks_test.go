package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

func TestPriceHistory(t *testing.T) {
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	r := newTestRouter(&queryStoreStub{prices: []models.DailyPrice{
		{Date: day, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 100},
	}})

	w := doGet(t, r, "/api/v1/stocks/aapl/history")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"date":"2024-01-02"`) || !strings.Contains(w.Body.String(), `"close":1.5`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	if lw := doGet(t, r, "/api/v1/stocks/aapl/history?limit=0"); lw.Code != http.StatusBadRequest {
		t.Fatalf("limit=0 status = %d, want 400", lw.Code)
	}
	if lw := doGet(t, r, "/api/v1/stocks/aapl/history?limit=9999"); lw.Code != http.StatusBadRequest {
		t.Fatalf("limit=9999 status = %d, want 400", lw.Code)
	}

	empty := newTestRouter(&queryStoreStub{})
	if w := doGet(t, empty, "/api/v1/stocks/msft/history"); w.Code != http.StatusNotFound {
		t.Fatalf("empty history status = %d, want 404", w.Code)
	}
}

func TestStatementsEndpoints(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	r := newTestRouter(&queryStoreStub{
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalLiabilities: 300,
				TotalEquity: 200, TotalDebt: 50, CurrentAssets: 100, CurrentLiabilities: 40, SharesOutstanding: 10},
		},
		flows: []models.CashFlow{
			{Period: "annual", FiscalDate: day, OperatingCashFlow: 30, InvestingCashFlow: -10,
				FinancingCashFlow: -5, NetChangeInCash: 15},
		},
	})

	if w := doGet(t, r, "/api/v1/stocks/aapl/income-statement"); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"net_income":20`) {
		t.Fatalf("income status/body wrong: %d %s", w.Code, w.Body.String())
	}
	if w := doGet(t, r, "/api/v1/stocks/aapl/balance-sheet?period=quarterly"); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"shares_outstanding":10`) {
		t.Fatalf("balance status/body wrong: %d %s", w.Code, w.Body.String())
	}
	if w := doGet(t, r, "/api/v1/stocks/aapl/cash-flow"); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"operating_cash_flow":30`) {
		t.Fatalf("cash-flow status/body wrong: %d %s", w.Code, w.Body.String())
	}

	if w := doGet(t, r, "/api/v1/stocks/aapl/income-statement?period=bogus"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid period status = %d, want 400", w.Code)
	}
}
