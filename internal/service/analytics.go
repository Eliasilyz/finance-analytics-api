package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/analytics"
	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// AnalyticsResult is the computed snapshot for /analytics. Metrics are
// *float64 so individually undefined metrics serialize as null (partial-null
// policy): one bad metric never fails the whole response.
type AnalyticsResult struct {
	Symbol  string
	Price   float64
	Metrics AnalyticsMetrics
}

// AnalyticsMetrics holds the nine requested ratios in API order.
type AnalyticsMetrics struct {
	PERatio        *float64
	PriceToBook    *float64
	ROE            *float64
	ROA            *float64
	NetMargin      *float64
	CurrentRatio   *float64
	DebtToEquity   *float64
	RevenueGrowth  *float64
	EarningsGrowth *float64
}

// TechnicalResult is the computed snapshot for /technical. Same partial-null
// policy: an indicator that cannot be computed for the available history is
// null while the rest stay filled.
type TechnicalResult struct {
	Symbol   string
	LastDate time.Time
	Values   TechnicalValues
}

type TechnicalValues struct {
	DailyReturn     *float64
	SMA20           *float64
	SMA50           *float64
	SMA200          *float64
	EMA20           *float64
	RSI14           *float64
	Volatility20    *float64
	AverageVolume20 *float64
}

// GetAnalytics computes the nine metrics for symbol using the most recent
// annual statements and the latest price. ErrNotFound when the symbol is not
// in the database; ErrInsufficientData when it exists but has no data at all.
func (s *QueryService) GetAnalytics(ctx context.Context, symbol string) (AnalyticsResult, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return AnalyticsResult{}, err
	}
	if _, err := s.store.GetCompany(ctx, sym); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AnalyticsResult{}, s.wrapNotFound(err)
		}
		return AnalyticsResult{}, err
	}

	incomes, err := s.store.GetIncomeStatements(ctx, sym, "annual")
	if err != nil {
		return AnalyticsResult{}, s.insufficientErr(err)
	}
	balances, err := s.store.GetBalanceSheets(ctx, sym, "annual")
	if err != nil {
		return AnalyticsResult{}, s.insufficientErr(err)
	}
	latest, err := s.store.GetLatestPrice(ctx, sym)
	if err != nil {
		return AnalyticsResult{}, s.insufficientErr(err)
	}

	cur := incomes[0]
	var prev *models.IncomeStatement
	if len(incomes) > 1 {
		prev = &incomes[1]
	}
	b := balances[0]
	price := latest.Close

	m := AnalyticsMetrics{
		PERatio:      ratio(analytics.PERatio(price, cur.DilutedEPS)),
		PriceToBook:  ratio(analytics.PriceToBook(price, b.TotalEquity, b.SharesOutstanding)),
		ROE:          ratio(analytics.ROE(cur.NetIncome, b.TotalEquity)),
		ROA:          ratio(analytics.ROA(cur.NetIncome, b.TotalAssets)),
		NetMargin:    ratio(analytics.NetMargin(cur.NetIncome, cur.Revenue)),
		CurrentRatio: ratio(analytics.CurrentRatio(b.CurrentAssets, b.CurrentLiabilities)),
		DebtToEquity: ratio(analytics.DebtToEquity(b.TotalDebt, b.TotalEquity)),
	}
	if prev != nil {
		m.RevenueGrowth = ratio(analytics.RevenueGrowth(prev.Revenue, cur.Revenue))
		m.EarningsGrowth = ratio(analytics.EarningsGrowth(prev.NetIncome, cur.NetIncome))
	}
	return AnalyticsResult{Symbol: sym, Price: price, Metrics: m}, nil
}

// GetTechnical computes the eight indicators from the full price history,
// each independently (partial-null). ErrNotFound when the symbol is absent;
// ErrInsufficientData only when the company has zero price rows.
func (s *QueryService) GetTechnical(ctx context.Context, symbol string) (TechnicalResult, error) {
	sym, err := s.normalizeSymbol(symbol)
	if err != nil {
		return TechnicalResult{}, err
	}
	if _, err := s.store.GetCompany(ctx, sym); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TechnicalResult{}, s.wrapNotFound(err)
		}
		return TechnicalResult{}, err
	}

	prices, err := s.store.GetPriceHistory(ctx, sym, 0)
	if err != nil {
		return TechnicalResult{}, s.insufficientErr(err)
	}

	closes := make([]float64, len(prices))
	volumes := make([]float64, len(prices))
	for i, p := range prices {
		closes[i] = p.Close
		volumes[i] = float64(p.Volume)
	}

	t := TechnicalResult{
		Symbol:   sym,
		LastDate: prices[len(prices)-1].Date,
	}
	t.Values = TechnicalValues{
		DailyReturn:     last(analytics.DailyReturn(closes)),
		SMA20:           last(analytics.SMA(closes, 20)),
		SMA50:           last(analytics.SMA(closes, 50)),
		SMA200:          last(analytics.SMA(closes, 200)),
		EMA20:           last(analytics.EMA(closes, 20)),
		RSI14:           last(analytics.RSI(closes, 14)),
		Volatility20:    scalar(analytics.Volatility(closes, 20)),
		AverageVolume20: last(analytics.AverageVolume(volumes, 20)),
	}
	return t, nil
}

// ratio converts a (float64, error) analytics call into a nullable metric.
func ratio(v float64, err error) *float64 {
	if err != nil {
		return nil
	}
	return &v
}

// scalar is ratio without the error-value indirection.
func scalar(v float64, err error) *float64 { return ratio(v, err) }

// last takes the most recent element of an indicator series.
func last(vals []float64, err error) *float64 {
	if err != nil || len(vals) == 0 {
		return nil
	}
	return &vals[len(vals)-1]
}

// insufficientErr maps sql.ErrNoRows to ErrInsufficientData.
func (s *QueryService) insufficientErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %v", ErrInsufficientData, err)
	}
	return err
}
