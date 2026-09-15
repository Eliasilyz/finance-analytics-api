package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// compareStore serves multiple symbols with their own data for Compare tests.
type compareStore struct {
	bySymbol map[string]*companyData
}

type companyData struct {
	company  models.Company
	prices   []models.DailyPrice
	income   []models.IncomeStatement
	balances []models.BalanceSheet
}

func (s *compareStore) GetCompany(_ context.Context, symbol string) (models.Company, error) {
	if d := s.bySymbol[symbol]; d != nil {
		return d.company, nil
	}
	return models.Company{}, sql.ErrNoRows
}

func (s *compareStore) ListCompanies(_ context.Context) ([]models.Company, error) {
	return nil, nil
}

func (s *compareStore) GetPriceHistory(_ context.Context, symbol string, _ int) ([]models.DailyPrice, error) {
	if d := s.bySymbol[symbol]; d != nil && len(d.prices) > 0 {
		return d.prices, nil
	}
	return nil, sql.ErrNoRows
}

func (s *compareStore) GetLatestPrice(_ context.Context, symbol string) (models.DailyPrice, error) {
	if d := s.bySymbol[symbol]; d != nil && len(d.prices) > 0 {
		return d.prices[len(d.prices)-1], nil
	}
	return models.DailyPrice{}, sql.ErrNoRows
}

func (s *compareStore) GetIncomeStatements(_ context.Context, symbol string, _ string) ([]models.IncomeStatement, error) {
	if d := s.bySymbol[symbol]; d != nil && len(d.income) > 0 {
		return d.income, nil
	}
	return nil, sql.ErrNoRows
}

func (s *compareStore) GetBalanceSheets(_ context.Context, symbol string, _ string) ([]models.BalanceSheet, error) {
	if d := s.bySymbol[symbol]; d != nil && len(d.balances) > 0 {
		return d.balances, nil
	}
	return nil, sql.ErrNoRows
}

func (s *compareStore) GetCashFlows(_ context.Context, _ string, _ string) ([]models.CashFlow, error) {
	return nil, sql.ErrNoRows
}

func TestCompare(t *testing.T) {
	day := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	store := &compareStore{bySymbol: map[string]*companyData{
		"AAPL": {
			company: models.Company{Symbol: "AAPL"},
			prices:  []models.DailyPrice{{Date: day, Close: 50}},
			income: []models.IncomeStatement{
				{Period: "annual", FiscalDate: day, Revenue: 100, NetIncome: 20, DilutedEPS: 2},
			},
			balances: []models.BalanceSheet{
				{Period: "annual", FiscalDate: day, TotalAssets: 500, TotalLiabilities: 300,
					TotalEquity: 200, TotalDebt: 50, CurrentAssets: 100, CurrentLiabilities: 40, SharesOutstanding: 10},
			},
		},
		"MSFT": {
			company: models.Company{Symbol: "MSFT"},
			prices:  []models.DailyPrice{{Date: day, Close: 10}},
			income: []models.IncomeStatement{
				{Period: "annual", FiscalDate: day, Revenue: 200, NetIncome: 40, DilutedEPS: 4},
			},
			balances: []models.BalanceSheet{
				{Period: "annual", FiscalDate: day, TotalAssets: 100, TotalLiabilities: 60,
					TotalEquity: 40, TotalDebt: 10, CurrentAssets: 20, CurrentLiabilities: 10, SharesOutstanding: 5},
			},
		},
	}}
	s := NewQueryService(store)
	ctx := context.Background()

	t.Run("happy path preserves order", func(t *testing.T) {
		res, err := s.Compare(ctx, []string{"aapl", "MSFT "}, 10)
		if err != nil {
			t.Fatalf("Compare: %v", err)
		}
		if len(res) != 2 || res[0].Symbol != "AAPL" || res[1].Symbol != "MSFT" {
			t.Fatalf("unexpected order: %+v", res)
		}
		if res[0].Metrics.PERatio == nil || *res[0].Metrics.PERatio != 25 {
			t.Errorf("AAPL PE = %v, want 25", res[0].Metrics.PERatio)
		}
	})

	t.Run("invalid symbol fails whole call", func(t *testing.T) {
		if _, err := s.Compare(ctx, []string{"AAPL", "HAS SPACE"}, 10); !errors.Is(err, ErrInvalidSymbol) {
			t.Fatalf("want ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("duplicate fails whole call", func(t *testing.T) {
		if _, err := s.Compare(ctx, []string{"AAPL", "aapl"}, 10); !errors.Is(err, ErrInvalidSymbol) {
			t.Fatalf("want ErrInvalidSymbol, got %v", err)
		}
	})

	t.Run("too many symbols fails whole call", func(t *testing.T) {
		var many []string
		for i := 0; i < 11; i++ {
			many = append(many, "ABCDEFGHIJKLMNOP") // unique so the max check triggers
		}
		if _, err := s.Compare(ctx, many, 10); !errors.Is(err, ErrInvalidSymbol) {
			t.Fatalf("want ErrInvalidSymbol for 11 symbols, got %v", err)
		}
	})

	t.Run("unknown symbol aborts whole call", func(t *testing.T) {
		if _, err := s.Compare(ctx, []string{"AAPL", "ZZZZ"}, 10); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound (all-or-nothing), got %v", err)
		}
	})
}
