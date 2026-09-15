package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

type fakeQueryStore struct {
	company  models.Company
	hasCo    bool
	prices   []models.DailyPrice
	income   []models.IncomeStatement
	balances []models.BalanceSheet
	flows    []models.CashFlow
}

func (f *fakeQueryStore) GetCompany(_ context.Context, symbol string) (models.Company, error) {
	if !f.hasCo || f.company.Symbol != symbol {
		return models.Company{}, sql.ErrNoRows
	}
	return f.company, nil
}

func (f *fakeQueryStore) ListCompanies(_ context.Context) ([]models.Company, error) {
	if !f.hasCo {
		return []models.Company{}, nil
	}
	return []models.Company{f.company}, nil
}

func (f *fakeQueryStore) GetPriceHistory(_ context.Context, _ string, _ int) ([]models.DailyPrice, error) {
	if len(f.prices) == 0 {
		return nil, sql.ErrNoRows
	}
	return f.prices, nil
}

func (f *fakeQueryStore) GetLatestPrice(_ context.Context, _ string) (models.DailyPrice, error) {
	if len(f.prices) == 0 {
		return models.DailyPrice{}, sql.ErrNoRows
	}
	return f.prices[len(f.prices)-1], nil
}

func (f *fakeQueryStore) GetIncomeStatements(_ context.Context, _ string, _ string) ([]models.IncomeStatement, error) {
	if len(f.income) == 0 {
		return nil, sql.ErrNoRows
	}
	return f.income, nil
}

func (f *fakeQueryStore) GetBalanceSheets(_ context.Context, _ string, _ string) ([]models.BalanceSheet, error) {
	if len(f.balances) == 0 {
		return nil, sql.ErrNoRows
	}
	return f.balances, nil
}

func (f *fakeQueryStore) GetCashFlows(_ context.Context, _ string, _ string) ([]models.CashFlow, error) {
	if len(f.flows) == 0 {
		return nil, sql.ErrNoRows
	}
	return f.flows, nil
}

func (f *fakeQueryStore) Screener(_ context.Context, _ models.ScreenerFilter) ([]models.ScreenerResult, error) {
	return nil, nil
}

func TestQueryServiceGetCompany(t *testing.T) {
	s := NewQueryService(&fakeQueryStore{
		hasCo: true,
		company: models.Company{
			Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ",
			Sector: "Technology", Industry: "Consumer Electronics",
			Currency: "USD", Description: "",
		},
	})
	ctx := context.Background()

	t.Run("normalizes lowercase symbol", func(t *testing.T) {
		c, err := s.GetCompany(ctx, " aapl ")
		if err != nil {
			t.Fatalf("GetCompany: %v", err)
		}
		if c.Symbol != "AAPL" {
			t.Errorf("symbol = %q, want AAPL", c.Symbol)
		}
	})

	t.Run("invalid symbol", func(t *testing.T) {
		if _, err := s.GetCompany(ctx, "a very long symbol name !"); !errors.Is(err, ErrInvalidSymbol) {
			t.Fatalf("want ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("unknown symbol maps to ErrNotFound", func(t *testing.T) {
		s2 := NewQueryService(&fakeQueryStore{})
		if _, err := s2.GetCompany(ctx, "MSFT"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestQueryServiceGetPriceHistory(t *testing.T) {
	day := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	s := NewQueryService(&fakeQueryStore{prices: []models.DailyPrice{
		{Date: day, Open: 1, High: 2, Low: 1, Close: 1.5, Volume: 100},
		{Date: day.AddDate(0, 0, 1), Open: 1.5, High: 2.5, Low: 1.4, Close: 2.4, Volume: 200},
	}})
	ctx := context.Background()

	rows, err := s.GetPriceHistory(ctx, "aapl", 0)
	if err != nil {
		t.Fatalf("GetPriceHistory: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	empty := NewQueryService(&fakeQueryStore{})
	if _, err := empty.GetPriceHistory(ctx, "msft", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for empty history, got %v", err)
	}
}

func TestQueryServiceStatements(t *testing.T) {
	s := NewQueryService(&fakeQueryStore{income: []models.IncomeStatement{
		{Period: "annual", FiscalDate: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), Revenue: 100, NetIncome: 20, DilutedEPS: 2},
	}})
	ctx := context.Background()

	got, err := s.GetIncomeStatements(ctx, "aapl", "annual")
	if err != nil {
		t.Fatalf("GetIncomeStatements: %v", err)
	}
	if len(got) != 1 || got[0].Revenue != 100 {
		t.Fatalf("unexpected income statements: %+v", got)
	}

	if _, err := s.GetBalanceSheets(ctx, "aapl", "annual"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for empty balance sheets, got %v", err)
	}
}
