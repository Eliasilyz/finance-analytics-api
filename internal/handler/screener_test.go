package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// screenerStub drives /screener handler tests with canned filter results.
type screenerStub struct {
	results  []models.ScreenerResult
	captured *models.ScreenerFilter
}

func (s *screenerStub) GetCompany(_ context.Context, _ string) (models.Company, error) {
	return models.Company{}, nil
}
func (s *screenerStub) ListCompanies(_ context.Context) ([]models.Company, error) { return nil, nil }
func (s *screenerStub) GetPriceHistory(_ context.Context, _ string, _ int) ([]models.DailyPrice, error) {
	return nil, nil
}
func (s *screenerStub) GetLatestPrice(_ context.Context, _ string) (models.DailyPrice, error) {
	return models.DailyPrice{}, nil
}
func (s *screenerStub) GetIncomeStatements(_ context.Context, _ string, _ string) ([]models.IncomeStatement, error) {
	return nil, nil
}
func (s *screenerStub) GetBalanceSheets(_ context.Context, _ string, _ string) ([]models.BalanceSheet, error) {
	return nil, nil
}
func (s *screenerStub) GetCashFlows(_ context.Context, _ string, _ string) ([]models.CashFlow, error) {
	return nil, nil
}
func (s *screenerStub) Screener(_ context.Context, f models.ScreenerFilter) ([]models.ScreenerResult, error) {
	cp := f
	s.captured = &cp
	return s.results, nil
}

func TestScreenerHandler(t *testing.T) {
	roe := 0.20
	pe := 15.0
	stub := &screenerStub{results: []models.ScreenerResult{
		{Symbol: "AAPL", Name: "Apple", Sector: "Technology", ROE: &roe, PE: &pe},
	}}
	r := newTestRouter(stub)

	w := doGet(t, r, "/api/v1/screener?min_roe=0.15&max_pe=30&sector=Technology&min_revenue_growth=0.05")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if stub.captured == nil || stub.captured.MinROE == nil || *stub.captured.MinROE != 0.15 ||
		stub.captured.MaxPE == nil || *stub.captured.MaxPE != 30 ||
		stub.captured.Sector == nil || *stub.captured.Sector != "Technology" ||
		stub.captured.MinRevenueGrowth == nil || *stub.captured.MinRevenueGrowth != 0.05 {
		t.Fatalf("filter not forwarded correctly: %+v", stub.captured)
	}
	if !strings.Contains(w.Body.String(), `"symbol":"AAPL"`) {
		t.Fatalf("body: %s", w.Body.String())
	}

	if w := doGet(t, r, "/api/v1/screener?min_roe=abc"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid numeric status = %d, want 400", w.Code)
	}
}
