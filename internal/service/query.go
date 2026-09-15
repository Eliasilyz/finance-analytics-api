package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// Errors surfaced by QueryService methods. Handlers map these to status codes.
// ErrInvalidSymbol is shared with ingestion (declared in ingestion.go).
var (
	// ErrNotFound is returned when a company or its data does not exist.
	ErrNotFound = errors.New("query: not found")
	// ErrInsufficientData is returned when the resource exists but has no
	// data needed for a computation.
	ErrInsufficientData = errors.New("query: insufficient data")
)

// QueryStore is the read side of the repository, implemented by
// repository.Repository and faked in service tests.
type QueryStore interface {
	GetCompany(ctx context.Context, symbol string) (models.Company, error)
	ListCompanies(ctx context.Context) ([]models.Company, error)
	GetPriceHistory(ctx context.Context, symbol string, limit int) ([]models.DailyPrice, error)
	GetLatestPrice(ctx context.Context, symbol string) (models.DailyPrice, error)
	GetIncomeStatements(ctx context.Context, symbol, period string) ([]models.IncomeStatement, error)
	GetBalanceSheets(ctx context.Context, symbol, period string) ([]models.BalanceSheet, error)
	GetCashFlows(ctx context.Context, symbol, period string) ([]models.CashFlow, error)
}

// QueryService is the read-side service behind the REST API: it validates
// input, reads data through QueryStore and (for computed endpoints) feeds it
// as parameters to the pure analytics functions.
type QueryService struct {
	store QueryStore
}

// NewQueryService builds a QueryService backed by store.
func NewQueryService(store QueryStore) *QueryService {
	return &QueryService{store: store}
}

// normalizeSymbol validates and uppercases an input symbol using the same
// rule as ingestion.
func (s *QueryService) normalizeSymbol(symbol string) (string, error) {
	return normalizeSymbol(symbol)
}

// ListCompanies returns all companies ordered by symbol.
func (s *QueryService) ListCompanies(ctx context.Context) ([]models.Company, error) {
	return s.store.ListCompanies(ctx)
}

// GetCompany returns one company or ErrNotFound.
func (s *QueryService) GetCompany(ctx context.Context, symbol string) (models.Company, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return models.Company{}, err
	}
	c, err := s.store.GetCompany(ctx, sym)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Company{}, s.wrapNotFound(err)
	}
	return c, err
}

// GetPriceHistory returns daily prices, oldest first, capped at limit when
// limit > 0. Returns ErrNotFound for an unknown symbol or missing prices.
func (s *QueryService) GetPriceHistory(ctx context.Context, symbol string, limit int) ([]models.DailyPrice, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.GetPriceHistory(ctx, sym, limit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, s.wrapNotFound(err)
	}
	return rows, err
}

// GetIncomeStatements returns the income statements for symbol ordered newest
// first, filtered by period ("", "all", "annual", "quarterly").
func (s *QueryService) GetIncomeStatements(ctx context.Context, symbol, period string) ([]models.IncomeStatement, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.GetIncomeStatements(ctx, sym, period)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, s.wrapNotFound(err)
	}
	return rows, err
}

// GetBalanceSheets returns the balance sheets for symbol ordered newest first.
func (s *QueryService) GetBalanceSheets(ctx context.Context, symbol, period string) ([]models.BalanceSheet, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.GetBalanceSheets(ctx, sym, period)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, s.wrapNotFound(err)
	}
	return rows, err
}

// GetCashFlows returns the cash flow statements for symbol ordered newest first.
func (s *QueryService) GetCashFlows(ctx context.Context, symbol, period string) ([]models.CashFlow, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.GetCashFlows(ctx, sym, period)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, s.wrapNotFound(err)
	}
	return rows, err
}

// wrapNotFound wraps store errors caused by absent data in ErrNotFound while
// keeping the original cause reachable.
func (s *QueryService) wrapNotFound(err error) error {
	return fmt.Errorf("%w: %v", ErrNotFound, err)
}

// Compare returns the analytics metrics for each symbol, computed through the
// same path as GetAnalytics. ALL-OR-NOTHING (per the API decision): any
// syntactically invalid, duplicate, or out-of-range symbol fails the whole
// call with ErrInvalidSymbol; the first symbol that is unknown (ErrNotFound)
// or data-starved (ErrInsufficientData) likewise aborts the whole call.
func (s *QueryService) Compare(ctx context.Context, raw []string, max int) ([]AnalyticsResult, error) {
	symbols := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		sym, err := normalizeSymbol(item)
		if err != nil {
			return nil, err
		}
		if len(symbols) >= max {
			return nil, fmt.Errorf("%w: at most %d symbols", ErrInvalidSymbol, max)
		}
		if seen[sym] {
			return nil, fmt.Errorf("%w: duplicate symbol %q", ErrInvalidSymbol, sym)
		}
		seen[sym] = true
		symbols = append(symbols, sym)
	}
	if len(symbols) == 0 {
		return nil, fmt.Errorf("%w: at least one symbol is required", ErrInvalidSymbol)
	}

	out := make([]AnalyticsResult, 0, len(symbols))
	for _, sym := range symbols {
		res, err := s.GetAnalytics(ctx, sym)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}
