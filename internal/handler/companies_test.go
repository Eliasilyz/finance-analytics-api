package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

// queryStoreStub implements service.QueryStore for handler tests.
type queryStoreStub struct {
	company  models.Company
	hasCo    bool
	prices   []models.DailyPrice
	income   []models.IncomeStatement
	balances []models.BalanceSheet
	flows    []models.CashFlow
}

func (s *queryStoreStub) GetCompany(_ context.Context, symbol string) (models.Company, error) {
	if !s.hasCo || s.company.Symbol != symbol {
		return models.Company{}, sql.ErrNoRows
	}
	return s.company, nil
}

func (s *queryStoreStub) ListCompanies(_ context.Context) ([]models.Company, error) {
	if !s.hasCo {
		return nil, nil
	}
	return []models.Company{s.company}, nil
}

func (s *queryStoreStub) GetPriceHistory(_ context.Context, _ string, _ int) ([]models.DailyPrice, error) {
	if len(s.prices) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.prices, nil
}

func (s *queryStoreStub) GetLatestPrice(_ context.Context, _ string) (models.DailyPrice, error) {
	if len(s.prices) == 0 {
		return models.DailyPrice{}, sql.ErrNoRows
	}
	return s.prices[len(s.prices)-1], nil
}

func (s *queryStoreStub) GetIncomeStatements(_ context.Context, _ string, _ string) ([]models.IncomeStatement, error) {
	if len(s.income) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.income, nil
}

func (s *queryStoreStub) GetBalanceSheets(_ context.Context, _ string, _ string) ([]models.BalanceSheet, error) {
	if len(s.balances) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.balances, nil
}

func (s *queryStoreStub) GetCashFlows(_ context.Context, _ string, _ string) ([]models.CashFlow, error) {
	if len(s.flows) == 0 {
		return nil, sql.ErrNoRows
	}
	return s.flows, nil
}

func (s *queryStoreStub) Screener(_ context.Context, _ models.ScreenerFilter) ([]models.ScreenerResult, error) {
	return nil, nil
}

func newTestRouter(store service.QueryStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := New(service.NewQueryService(store), slog.New(slog.DiscardHandler))
	e := gin.New()
	h.Routes(e)
	return e
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestHealth(t *testing.T) {
	r := newTestRouter(&queryStoreStub{})
	w := doGet(t, r, "/health")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestListCompanies(t *testing.T) {
	r := newTestRouter(&queryStoreStub{
		hasCo: true,
		company: models.Company{
			Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ",
			Sector: "Technology", Industry: "Consumer Electronics", Currency: "USD",
		},
	})
	w := doGet(t, r, "/api/v1/companies")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	got := strings.TrimSpace(w.Body.String())
	want := `{"data":[{"currency":"USD","exchange":"NASDAQ","industry":"Consumer Electronics","name":"Apple Inc.","sector":"Technology","symbol":"AAPL"}]}`
	if got != want {
		t.Fatalf("body = %s\nwant %s", got, want)
	}
}

func TestGetCompany(t *testing.T) {
	r := newTestRouter(&queryStoreStub{
		hasCo: true,
		company: models.Company{
			Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ",
			Sector: "Technology", Industry: "Consumer Electronics", Currency: "USD", Description: "hi",
		},
	})
	w := doGet(t, r, "/api/v1/companies/aapl")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	missing := doGet(t, r, "/api/v1/companies/msft")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", missing.Code)
	}
}
