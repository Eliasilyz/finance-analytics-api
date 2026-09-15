package handler

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// compareStub serves a fixed set of symbols for /compare handler tests.
type compareStub struct {
	known    map[string]bool
	prices   []models.DailyPrice
	income   []models.IncomeStatement
	balances []models.BalanceSheet
}

func (s *compareStub) GetCompany(_ context.Context, symbol string) (models.Company, error) {
	if s.known[symbol] {
		return models.Company{Symbol: symbol}, nil
	}
	return models.Company{}, sql.ErrNoRows
}

func (s *compareStub) ListCompanies(_ context.Context) ([]models.Company, error) { return nil, nil }
func (s *compareStub) GetPriceHistory(_ context.Context, _ string, _ int) ([]models.DailyPrice, error) {
	if len(s.prices) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.prices, nil
}
func (s *compareStub) GetLatestPrice(_ context.Context, _ string) (models.DailyPrice, error) {
	if len(s.prices) == 0 {
		return models.DailyPrice{}, sql.ErrNoRows
	}
	return s.prices[len(s.prices)-1], nil
}
func (s *compareStub) GetIncomeStatements(_ context.Context, _ string, _ string) ([]models.IncomeStatement, error) {
	if len(s.income) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.income, nil
}
func (s *compareStub) GetBalanceSheets(_ context.Context, _ string, _ string) ([]models.BalanceSheet, error) {
	if len(s.balances) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.balances, nil
}
func (s *compareStub) GetCashFlows(_ context.Context, _ string, _ string) ([]models.CashFlow, error) {
	return nil, sql.ErrNoRows
}

func TestCompareHandler(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	r := newTestRouter(&compareStub{
		known:  map[string]bool{"AAPL": true, "MSFT": true},
		prices: []models.DailyPrice{{Date: day, Close: 50}},
		income: []models.IncomeStatement{
			{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
		},
		balances: []models.BalanceSheet{
			{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalLiabilities: 300,
				TotalEquity: 200, TotalDebt: 50, CurrentAssets: 100, CurrentLiabilities: 40, SharesOutstanding: 10},
		},
	})

	t.Run("all known symbols", func(t *testing.T) {
		w := doGet(t, r, "/api/v1/compare?symbols=AAPL,MSFT")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !strings.Contains(body, `"symbol":"AAPL"`) || !strings.Contains(body, `"symbol":"MSFT"`) {
			t.Fatalf("each symbol must appear: %s", body)
		}
	})

	t.Run("one unknown symbol fails whole request", func(t *testing.T) {
		w := doGet(t, r, "/api/v1/compare?symbols=AAPL,ZZZZ")
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (all-or-nothing)", w.Code)
		}
	})

	t.Run("invalid symbol returns 400 for whole request", func(t *testing.T) {
		w := doGet(t, r, "/api/v1/compare?symbols=AAPL,BAD$SYM")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
		if !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
			t.Fatalf("body: %s", w.Body.String())
		}
	})

	t.Run("empty symbols returns 400", func(t *testing.T) {
		w := doGet(t, r, "/api/v1/compare?symbols=")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
