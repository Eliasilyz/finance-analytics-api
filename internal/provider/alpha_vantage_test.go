package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient returns an AlphaVantage wired to a fake HTTP server that calls
// handler. baseURL is overridden so no real Alpha Vantage traffic happens.
func newTestClient(t *testing.T, handler http.HandlerFunc) *AlphaVantage {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &AlphaVantage{
		baseURL: srv.URL,
		apiKey:  "test-key",
		client:  srv.Client(),
	}
}

func TestFetchCompanySnapshotSuccess(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fn := r.URL.Query().Get("function")
		switch fn {
		case "OVERVIEW":
			_, _ = w.Write([]byte(`{"Symbol":"AAPL","Name":"Apple Inc.","Exchange":"NASDAQ","Sector":"Technology","Industry":"Consumer Electronics","Currency":"USD","Description":"Big tech."}`))
		case "TIME_SERIES_DAILY":
			_, _ = w.Write([]byte(`{"Time Series (Daily)":{
				"2024-01-03":{"1. open":"190.50","2. high":"192.60","3. low":"189.70","4. close":"191.20","5. volume":"71234567"},
				"2024-01-02":{"1. open":"188.40","2. high":"190.20","3. low":"187.90","4. close":"190.00","5. volume":"68451234"}
			}}`))
		case "INCOME_STATEMENT":
			_, _ = w.Write([]byte(`{"annualReports":[{"fiscalDateEnding":"2023-09-30","totalRevenue":"383285000000","grossProfit":"169148000000","operatingIncome":"114301000000","netIncome":"96995000000","dilutedEPS":"6.16"}],"quarterlyReports":[{"fiscalDateEnding":"2024-03-30","totalRevenue":"90753000000","grossProfit":"38959000000","operatingIncome":"27511000000","netIncome":"23636000000","dilutedEPS":"1.53"}]}`))
		case "BALANCE_SHEET":
			_, _ = w.Write([]byte(`{"annualReports":[{"fiscalDateEnding":"2023-09-30","totalAssets":"352583000000","totalLiabilities":"290437000000","totalShareholderEquity":"62146000000","totalDebt":"120820000000","currentAssets":"143566000000","currentLiabilities":"145308000000","commonStockSharesOutstanding":"15550107000"}],"quarterlyReports":[]}`))
		case "CASH_FLOW":
			_, _ = w.Write([]byte(`{"annualReports":[{"fiscalDateEnding":"2023-09-30","operatingCashflow":"110543000000","investingCashflow":"-28431000000","financingCashflow":"-36551000000","changeInCashAndCashEquivalents":"-73300000"}],"quarterlyReports":[]}`))
		default:
			t.Errorf("unexpected function %q", fn)
		}
	})

	snap, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snap.Company.Symbol != "AAPL" || snap.Company.Name != "Apple Inc." {
		t.Errorf("unexpected company: %+v", snap.Company)
	}
	if len(snap.Prices) != 2 {
		t.Fatalf("expected 2 prices, got %d", len(snap.Prices))
	}
	// Sorted ascending by date.
	if snap.Prices[0].Date.Format("2006-01-02") != "2024-01-02" || snap.Prices[1].Date.Format("2006-01-02") != "2024-01-03" {
		t.Errorf("prices not sorted by date: %v %v", snap.Prices[0].Date, snap.Prices[1].Date)
	}
	if snap.Prices[1].Close != 191.20 || snap.Prices[1].Volume != 71234567 {
		t.Errorf("unexpected price record: %+v", snap.Prices[1])
	}
	if len(snap.Income) != 2 || snap.Income[0].Period != "annual" || snap.Income[1].Period != "quarterly" {
		t.Fatalf("unexpected income statements: %+v", snap.Income)
	}
	if snap.Income[0].Revenue != 383285000000 {
		t.Errorf("unexpected annual revenue: %v", snap.Income[0].Revenue)
	}
	if len(snap.Balance) != 1 || snap.Balance[0].TotalAssets != 352583000000 || snap.Balance[0].SharesOutstanding != 15550107000 {
		t.Errorf("unexpected balance sheet: %+v", snap.Balance)
	}
	if len(snap.CashFlow) != 1 || snap.CashFlow[0].Period != "annual" ||
		snap.CashFlow[0].OperatingCashFlow != 110543000000 ||
		snap.CashFlow[0].InvestingCashFlow != -28431000000 ||
		snap.CashFlow[0].FinancingCashFlow != -36551000000 ||
		snap.CashFlow[0].NetChangeInCash != -73300000 {
		t.Errorf("unexpected cash flow: %+v", snap.CashFlow)
	}
}

func TestFetchCompanySnapshotInvalidSymbol(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Error Message":"Invalid API call. Please retry or visit the documentation."}`))
	})
	_, err := av.FetchCompanySnapshot(context.Background(), "NOPE")
	if !errors.Is(err, ErrInvalidSymbol) {
		t.Fatalf("expected ErrInvalidSymbol, got %v", err)
	}
}

func TestFetchCompanySnapshotRateLimitNote(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Note":"Thank you for using Alpha Vantage! Our standard API rate limit is 25 requests per day."}`))
	})
	_, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestFetchCompanySnapshotHTTP429(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestFetchCompanySnapshotHTTPServerError(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("expected ErrProviderFailed, got %v", err)
	}
}

func TestFetchCompanySnapshotInvalidData(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Time Series (Daily)":`)) // truncated -> malformed JSON
	})
	_, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("expected ErrProviderFailed for malformed JSON, got %v", err)
	}
}

func TestFetchCompanySnapshotSkipsMalformedRows(t *testing.T) {
	av := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("function") {
		case "OVERVIEW":
			_, _ = w.Write([]byte(`{"Symbol":"AAPL"}`))
		case "TIME_SERIES_DAILY":
			// Bad date and non-numeric numbers are skipped, good rows kept.
			_, _ = w.Write([]byte(`{"Time Series (Daily)":{
				"not-a-date":{"1. open":"a","2. high":"b","3. low":"c","4. close":"d","5. volume":"e"},
				"2024-01-02":{"1. open":"188.40","2. high":"190.20","3. low":"187.90","4. close":"190.00","5. volume":"68451234"}
			}}`))
		case "INCOME_STATEMENT", "BALANCE_SHEET", "CASH_FLOW":
			_, _ = w.Write([]byte(`{"annualReports":[],"quarterlyReports":[]}`))
		}
	})
	snap, err := av.FetchCompanySnapshot(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Prices) != 1 {
		t.Fatalf("expected 1 valid price, got %d", len(snap.Prices))
	}
	if snap.Prices[0].Close != 190.00 {
		t.Errorf("unexpected close: %v", snap.Prices[0].Close)
	}
}

func TestFetchCompanySnapshotTimeout(t *testing.T) {
	// Provider that never answers within the caller's deadline: server stalls
	// 200ms while the ctx expires at 50ms. Asserts the client surfaces a
	// sentinel-wrapped error well before the server would have responded —
	// proving it times out instead of hanging or panicking.
	const serverDelay = 200 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(serverDelay)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	av := &AlphaVantage{
		baseURL: srv.URL,
		apiKey:  "test-key",
		client:  &http.Client{Timeout: 30 * time.Second}, // generous; ctx is the deadline
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := av.FetchCompanySnapshot(ctx, "AAPL")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	// About late: the failure must be *caused* by the deadline, not some other
	// transport hiccup. Go's http client wraps ctx.Err() into a *url.Error that
	// errors.Is can traverse, and query() preserves the chain via double %w.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	if !errors.Is(err, ErrProviderFailed) {
		t.Errorf("expected wrapped ErrProviderFailed, got %v", err)
	}
	// Timeout must fire before the server would have answered.
	if elapsed >= serverDelay {
		t.Errorf("client hung instead of timing out: returned after %v (server answers at %v)", elapsed, serverDelay)
	}
}
