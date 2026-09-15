package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/provider"
)

// DataStore is the persistence surface the ingestion pipeline needs. The
// repository package implements it; tests use an in-memory fake.
type DataStore interface {
	UpsertCompany(ctx context.Context, c models.Company) (int64, error)
	InsertPrices(ctx context.Context, companyID int64, prices []models.DailyPrice) (int64, error)
	InsertIncomeStatements(ctx context.Context, companyID int64, stmts []models.IncomeStatement) (int64, error)
	InsertBalanceSheets(ctx context.Context, companyID int64, sheets []models.BalanceSheet) (int64, error)
	InsertCashFlows(ctx context.Context, companyID int64, flows []models.CashFlow) (int64, error)
	RecordRun(ctx context.Context, run models.IngestionRun) error
}

// Errors surfaced to callers of Ingest.
var (
	ErrInvalidSymbol = errors.New("ingestion: invalid symbol")
	ErrNoPrices      = errors.New("ingestion: no valid price data in provider response")
)

var symbolRe = regexp.MustCompile(`^[A-Z0-9.]{1,10}$`)

// IsValidSymbol reports whether s (already trimmed/uppercased) is an
// acceptable stock symbol. Shared by ingestion and the REST API handlers so
// validation is defined once, not duplicated per layer.
func IsValidSymbol(s string) bool { return symbolRe.MatchString(s) }

// IngestionService executes the fetch -> validate -> normalize -> persist
// pipeline and records every attempt (success or failure) in ingestion_runs.
type IngestionService struct {
	provider provider.FinancialDataProvider
	store    DataStore
	cache    Cache
	log      *slog.Logger
}

// NewIngestionService wires the pipeline. A cache is optional: when present,
// a successful Ingest deletes that symbol's cached analytics/technical
// results so the API never serves stale numbers after fresh data lands.
func NewIngestionService(p provider.FinancialDataProvider, store DataStore, logger *slog.Logger, cache ...Cache) *IngestionService {
	s := &IngestionService{provider: p, store: store, log: logger}
	if len(cache) > 0 {
		s.cache = cache[0]
	}
	return s
}

// Ingest pulls a company snapshot for symbol, validates and normalizes it, then
// persists it idempotently. Re-ingesting the same symbol is a no-op that still
// records a successful run, satisfying the "data already stored" case.
func (s *IngestionService) Ingest(ctx context.Context, symbol string) (models.IngestionRun, error) {
	sym, err := normalizeSymbol(symbol)
	if err != nil {
		return models.IngestionRun{}, err
	}

	started := time.Now()
	run := models.IngestionRun{Symbol: sym, Status: "failure", StartedAt: started}
	fail := func(kind string, insertErr error) (models.IngestionRun, error) {
		run.Status = kind
		if insertErr != nil {
			run.ErrorMessage = insertErr.Error()
		}
		run.FinishedAt = time.Now()
		if recErr := s.store.RecordRun(ctx, run); recErr != nil {
			// Audit must not mask the original ingestion error.
			s.log.Error("record run failed", "symbol", sym, "error", recErr)
		}
		return run, insertErr
	}

	snap, fetchErr := s.provider.FetchCompanySnapshot(ctx, sym)
	if fetchErr != nil {
		return fail("failure", fmt.Errorf("fetch snapshot: %w", fetchErr))
	}

	companyID, upsErr := s.store.UpsertCompany(ctx, snap.Company)
	if upsErr != nil {
		return fail("failure", fmt.Errorf("upsert company: %w", upsErr))
	}
	run.CompaniesInserted = 1

	prices := validPrices(snap.Prices)
	if len(prices) == 0 {
		return fail("failure", ErrNoPrices)
	}
	if n, pErr := s.store.InsertPrices(ctx, companyID, prices); pErr != nil {
		return fail("failure", fmt.Errorf("insert prices: %w", pErr))
	} else {
		run.PricesInserted = int(n)
	}

	income := validIncomeStatements(snap.Income)
	if len(income) > 0 {
		if n, iErr := s.store.InsertIncomeStatements(ctx, companyID, income); iErr != nil {
			return fail("failure", fmt.Errorf("insert income statements: %w", iErr))
		} else {
			run.IncomeInserted = int(n)
		}
	}

	balance := validBalanceSheets(snap.Balance)
	if len(balance) > 0 {
		if n, bErr := s.store.InsertBalanceSheets(ctx, companyID, balance); bErr != nil {
			return fail("failure", fmt.Errorf("insert balance sheets: %w", bErr))
		} else {
			run.BalanceInserted = int(n)
		}
	}

	cashFlow := validCashFlows(snap.CashFlow)
	if len(cashFlow) > 0 {
		if n, cfErr := s.store.InsertCashFlows(ctx, companyID, cashFlow); cfErr != nil {
			return fail("failure", fmt.Errorf("insert cash flow statements: %w", cfErr))
		} else {
			run.CashFlowInserted = int(n)
		}
	}

	run.Status = "success"
	run.FinishedAt = time.Now()
	if recErr := s.store.RecordRun(ctx, run); recErr != nil {
		return run, fmt.Errorf("ingestion succeeded but audit failed: %w", recErr)
	}
	// Fresh data invalidates the read-through cache for this symbol; a stale
	// analytics/technical response after an ingestion would be a correctness
	// regression, so deletion is best-effort but logged when it fails.
	if s.cache != nil {
		for _, k := range []string{"analytics:" + sym, "technical:" + sym} {
			if err := s.cache.Del(ctx, k); err != nil {
				s.log.Warn("cache invalidation failed", "symbol", sym, "key", k, "error", err)
			}
		}
	}
	s.log.Info("ingestion complete",
		"symbol", sym,
		"prices", run.PricesInserted,
		"income", run.IncomeInserted,
		"balance", run.BalanceInserted,
		"cash_flow", run.CashFlowInserted,
	)
	return run, nil
}

// normalizeSymbol uppercases and validates an input symbol.
func normalizeSymbol(symbol string) (string, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if !IsValidSymbol(sym) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSymbol, symbol)
	}
	return sym, nil
}

// validPrices drops malformed rows (zero/negative prices, inverted high/low,
// negative volume, missing date). The provider is an external untrusted source,
// so every row is validated at the trust boundary.
func validPrices(prices []models.DailyPrice) []models.DailyPrice {
	out := make([]models.DailyPrice, 0, len(prices))
	for _, p := range prices {
		if p.Date.IsZero() || p.Open <= 0 || p.High <= 0 || p.Low <= 0 || p.Close <= 0 {
			continue
		}
		if p.High < p.Open || p.High < p.Close || p.Low > p.Open || p.Low > p.Close {
			continue
		}
		if p.Volume < 0 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// validIncomeStatements drops rows with a missing fiscal date or unknown period.
func validIncomeStatements(stmts []models.IncomeStatement) []models.IncomeStatement {
	out := make([]models.IncomeStatement, 0, len(stmts))
	for _, s := range stmts {
		if s.FiscalDate.IsZero() || !isValidPeriod(s.Period) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// validBalanceSheets drops rows with a missing fiscal date or unknown period.
func validBalanceSheets(sheets []models.BalanceSheet) []models.BalanceSheet {
	out := make([]models.BalanceSheet, 0, len(sheets))
	for _, s := range sheets {
		if s.FiscalDate.IsZero() || !isValidPeriod(s.Period) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// validCashFlows drops rows with a missing fiscal date or unknown period.
func validCashFlows(flows []models.CashFlow) []models.CashFlow {
	out := make([]models.CashFlow, 0, len(flows))
	for _, f := range flows {
		if f.FiscalDate.IsZero() || !isValidPeriod(f.Period) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func isValidPeriod(p string) bool { return p == "annual" || p == "quarterly" }
