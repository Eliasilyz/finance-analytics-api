package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// AlphaVantage implements FinancialDataProvider against the Alpha Vantage API.
// Free-tier rate limits are enforced upstream; a Note envelope maps to ErrRateLimited.
type AlphaVantage struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAlphaVantage returns a client for the given API key.
func NewAlphaVantage(apiKey string) *AlphaVantage {
	return &AlphaVantage{
		baseURL: "https://www.alphavantage.co/query",
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchCompanySnapshot fetches company overview, daily prices, and the income,
// balance, and cash flow statements for a symbol. Alpha Vantage reports
// per-endpoint; a failure in any endpoint fails the whole snapshot.
func (a *AlphaVantage) FetchCompanySnapshot(ctx context.Context, symbol string) (CompanySnapshot, error) {
	var (
		snapshot CompanySnapshot
		err      error
	)
	if snapshot.Company, err = a.fetchCompany(ctx, symbol); err != nil {
		return CompanySnapshot{}, err
	}
	if snapshot.Prices, err = a.fetchDailyPrices(ctx, symbol); err != nil {
		return CompanySnapshot{}, err
	}
	if snapshot.Income, err = fetchStatements(ctx, a, symbol, "INCOME_STATEMENT", parseIncome); err != nil {
		return CompanySnapshot{}, err
	}
	if snapshot.Balance, err = fetchStatements(ctx, a, symbol, "BALANCE_SHEET", parseBalance); err != nil {
		return CompanySnapshot{}, err
	}
	if snapshot.CashFlow, err = fetchStatements(ctx, a, symbol, "CASH_FLOW", parseCashFlow); err != nil {
		return CompanySnapshot{}, err
	}
	return snapshot, nil
}

// query performs a GET on the Alpha Vantage query endpoint, detects the error
// envelope Alpha Vantage returns inside HTTP 200 bodies, and decodes payloads.
func (a *AlphaVantage) query(ctx context.Context, params url.Values, out any) error {
	params.Set("apikey", a.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("%w: build request: %v", ErrProviderFailed, err)
	}
	res, err := a.client.Do(req)
	if err != nil {
		// Double %w keeps both sentinels in the unwrap chain: callers can match
		// ErrProviderFailed (provider contract) AND the real cause (e.g. a
		// context deadline) via errors.Is.
		return fmt.Errorf("%w: %w", ErrProviderFailed, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", ErrProviderFailed, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrProviderFailed, res.StatusCode)
	}

	var env struct {
		ErrorMessage string `json:"Error Message"`
		Note         string `json:"Note"`
		Information  string `json:"Information"`
	}
	// A well-formed success payload still unmarshals into env with zero fields,
	// so the envelope check only fires on the actual error shapes.
	if err := json.Unmarshal(body, &env); err == nil {
		switch {
		case env.Note != "":
			return ErrRateLimited
		case env.ErrorMessage != "":
			return fmt.Errorf("%w: %s", ErrInvalidSymbol, env.ErrorMessage)
		case env.Information != "" && strings.Contains(env.Information, "premium"):
			return fmt.Errorf("%w: %s", ErrProviderFailed, env.Information)
		}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrProviderFailed, err)
	}
	return nil
}

type companyOverview struct {
	Symbol      string `json:"Symbol"`
	Name        string `json:"Name"`
	Exchange    string `json:"Exchange"`
	Sector      string `json:"Sector"`
	Industry    string `json:"Industry"`
	Currency    string `json:"Currency"`
	Description string `json:"Description"`
}

func (a *AlphaVantage) fetchCompany(ctx context.Context, symbol string) (models.Company, error) {
	var raw companyOverview
	err := a.query(ctx, url.Values{
		"function": {"OVERVIEW"},
		"symbol":   {symbol},
	}, &raw)
	if err != nil {
		return models.Company{}, err
	}
	return models.Company{
		Symbol:      strings.ToUpper(strings.TrimSpace(symbol)),
		Name:        raw.Name,
		Exchange:    raw.Exchange,
		Sector:      raw.Sector,
		Industry:    raw.Industry,
		Currency:    raw.Currency,
		Description: raw.Description,
	}, nil
}

type dailyResponse struct {
	TimeSeries map[string]priceRecord `json:"Time Series (Daily)"`
}

type priceRecord struct {
	Open   string `json:"1. open"`
	High   string `json:"2. high"`
	Low    string `json:"3. low"`
	Close  string `json:"4. close"`
	Volume string `json:"5. volume"`
}

func (a *AlphaVantage) fetchDailyPrices(ctx context.Context, symbol string) ([]models.DailyPrice, error) {
	var raw dailyResponse
	err := a.query(ctx, url.Values{
		"function": {"TIME_SERIES_DAILY"},
		"symbol":   {symbol},
	}, &raw)
	if err != nil {
		return nil, err
	}
	prices := make([]models.DailyPrice, 0, len(raw.TimeSeries))
	for dateStr, rec := range raw.TimeSeries {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		prices = append(prices, models.DailyPrice{
			Date:   date,
			Open:   parseNum(rec.Open),
			High:   parseNum(rec.High),
			Low:    parseNum(rec.Low),
			Close:  parseNum(rec.Close),
			Volume: int64(parseNum(rec.Volume)),
		})
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Date.Before(prices[j].Date) })
	return prices, nil
}

type statementRecord struct {
	FiscalDate string `json:"fiscalDateEnding"`
}

type incomeRaw struct {
	statementRecord
	Revenue         string `json:"totalRevenue"`
	GrossProfit     string `json:"grossProfit"`
	OperatingIncome string `json:"operatingIncome"`
	NetIncome       string `json:"netIncome"`
	DilutedEPS      string `json:"dilutedEPS"`
}

type balanceRaw struct {
	statementRecord
	TotalAssets        string `json:"totalAssets"`
	TotalLiabilities   string `json:"totalLiabilities"`
	TotalEquity        string `json:"totalShareholderEquity"`
	TotalDebt          string `json:"totalDebt"`
	CurrentAssets      string `json:"currentAssets"`
	CurrentLiabilities string `json:"currentLiabilities"`
	SharesOutstanding  string `json:"commonStockSharesOutstanding"`
}

type cashFlowRaw struct {
	statementRecord
	OperatingCashFlow string `json:"operatingCashflow"`
	InvestingCashFlow string `json:"investingCashflow"`
	FinancingCashFlow string `json:"financingCashflow"`
	NetChangeInCash   string `json:"changeInCashAndCashEquivalents"`
}

type statementsResponse struct {
	Annual    []json.RawMessage `json:"annualReports"`
	Quarterly []json.RawMessage `json:"quarterlyReports"`
}

func parseIncome(raw json.RawMessage, period string) (models.IncomeStatement, bool) {
	var inc incomeRaw
	if err := json.Unmarshal(raw, &inc); err != nil {
		return models.IncomeStatement{}, false
	}
	date, err := time.Parse("2006-01-02", inc.FiscalDate)
	if err != nil {
		return models.IncomeStatement{}, false
	}
	return models.IncomeStatement{
		Period:          period,
		FiscalDate:      date,
		Revenue:         parseNum(inc.Revenue),
		GrossProfit:     parseNum(inc.GrossProfit),
		OperatingIncome: parseNum(inc.OperatingIncome),
		NetIncome:       parseNum(inc.NetIncome),
		DilutedEPS:      parseNum(inc.DilutedEPS),
	}, true
}

func parseBalance(raw json.RawMessage, period string) (models.BalanceSheet, bool) {
	var bal balanceRaw
	if err := json.Unmarshal(raw, &bal); err != nil {
		return models.BalanceSheet{}, false
	}
	date, err := time.Parse("2006-01-02", bal.FiscalDate)
	if err != nil {
		return models.BalanceSheet{}, false
	}
	return models.BalanceSheet{
		Period:             period,
		FiscalDate:         date,
		TotalAssets:        parseNum(bal.TotalAssets),
		TotalLiabilities:   parseNum(bal.TotalLiabilities),
		TotalEquity:        parseNum(bal.TotalEquity),
		TotalDebt:          parseNum(bal.TotalDebt),
		CurrentAssets:      parseNum(bal.CurrentAssets),
		CurrentLiabilities: parseNum(bal.CurrentLiabilities),
		SharesOutstanding:  parseNum(bal.SharesOutstanding),
	}, true
}

func parseCashFlow(raw json.RawMessage, period string) (models.CashFlow, bool) {
	var cf cashFlowRaw
	if err := json.Unmarshal(raw, &cf); err != nil {
		return models.CashFlow{}, false
	}
	date, err := time.Parse("2006-01-02", cf.FiscalDate)
	if err != nil {
		return models.CashFlow{}, false
	}
	return models.CashFlow{
		Period:            period,
		FiscalDate:        date,
		OperatingCashFlow: parseNum(cf.OperatingCashFlow),
		InvestingCashFlow: parseNum(cf.InvestingCashFlow),
		FinancingCashFlow: parseNum(cf.FinancingCashFlow),
		NetChangeInCash:   parseNum(cf.NetChangeInCash),
	}, true
}

// fetchStatements fetches a report endpoint and returns normalized records. Each
// record carries a "period" tag so annual and quarterly rows are distinguishable.
func fetchStatements[T any](
	ctx context.Context, a *AlphaVantage, symbol, function string,
	parse func(json.RawMessage, string) (T, bool),
) ([]T, error) {
	var raw statementsResponse
	err := a.query(ctx, url.Values{
		"function": {function},
		"symbol":   {symbol},
	}, &raw)
	if err != nil {
		return nil, err
	}

	out := make([]T, 0, len(raw.Annual)+len(raw.Quarterly))
	for _, rec := range raw.Annual {
		if v, ok := parse(rec, "annual"); ok {
			out = append(out, v)
		}
	}
	for _, rec := range raw.Quarterly {
		if v, ok := parse(rec, "quarterly"); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// parseNum parses Alpha Vantage's string-encoded numbers (may contain commas).
func parseNum(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
