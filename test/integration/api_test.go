//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/handler"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

// TestAPIEndpoints runs the full HTTP surface against a real Postgres.
func TestAPIEndpoints(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	ctx := context.Background()

	// Seed one company with two annual reports and 60 days of prices.
	var companyID int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO companies (symbol, name, exchange, sector, industry, currency)
		 VALUES ('AAPL', 'Apple Inc.', 'NASDAQ', 'Technology', 'Consumer Electronics', 'USD') RETURNING id`,
	).Scan(&companyID)
	if err != nil {
		t.Fatalf("insert company: %v", err)
	}
	// 60 trading days ending 2024-09-30.
	start := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		fy      string
		revenue float64
		ni      float64
		eps     float64
	}{
		{"2024-09-30", 391000000000, 97000000000, 6.30},
		{"2023-09-30", 383000000000, 96900000000, 6.16},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO income_statements (company_id, period, fiscal_date, revenue, net_income, diluted_eps)
			 VALUES ($1,'annual',$2,$3,$4,$5)`,
			companyID, row.fy, row.revenue, row.ni, row.eps); err != nil {
			t.Fatalf("insert income: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO balance_sheets (company_id, period, fiscal_date, total_assets, total_liabilities, total_equity, total_debt, current_assets, current_liabilities, shares_outstanding)
		 VALUES ($1,'annual','2024-09-30',352000000000,290000000000,62000000000,100000000000,120000000000,90000000000,16000000000)`,
		companyID); err != nil {
		t.Fatalf("insert balance: %v", err)
	}
	// 60 trading days ending 2024-09-30.
	start := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		d := start.AddDate(0, 0, i)
		close := 180.0 + float64(i)
		if _, err := db.ExecContext(ctx,
			`INSERT INTO price_history (company_id, date, open, high, low, close, volume)
			 VALUES ($1,$2,$3,$4,$5,$6,1000000)`,
			companyID, d, close-1, close+1, close-1, close); err != nil {
			t.Fatalf("insert price %d: %v", i, err)
		}
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewRepository(db)
	qs := service.NewQueryService(repo)
	h := handler.New(qs, logger)
	r := gin.New()
	h.Routes(r)

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("health", func(t *testing.T) {
		if w := get("/health"); w.Code != 200 {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("companies list", func(t *testing.T) {
		w := get("/api/v1/companies")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"symbol":"AAPL"`) {
			t.Fatalf("companies: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("analytics PE", func(t *testing.T) {
		w := get("/api/v1/stocks/aapl/analytics")
		if w.Code != 200 {
			t.Fatalf("analytics: %d %s", w.Code, w.Body.String())
		}
		// PE = 239 / 6.30 = 37.9...
		if !strings.Contains(w.Body.String(), `"pe":`) {
			t.Fatalf("analytics body: %s", w.Body.String())
		}
	})

	t.Run("technical partial null", func(t *testing.T) {
		w := get("/api/v1/stocks/aapl/technical")
		if w.Code != 200 {
			t.Fatalf("technical: %d %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		// sma200 needs 200 rows and must stay null; sma20 = avg of last 20
		// closes (220..239) = 229.5 and must be filled.
		if !strings.Contains(body, `"sma200":null`) || !strings.Contains(body, `"sma20":229.5`) {
			t.Fatalf("sma200 null + sma20 filled expected: %s", body)
		}
	})

	t.Run("unknown symbol 404 with error envelope", func(t *testing.T) {
		w := get("/api/v1/stocks/zzzz/analytics")
		if w.Code != 404 || !strings.Contains(w.Body.String(), `"code":"NOT_FOUND"`) {
			t.Fatalf("unknown: %d %s", w.Code, w.Body.String())
		}
	})

	t.Run("screener filter", func(t *testing.T) {
		w := get("/api/v1/screener?min_roe=1&max_pe=100")
		if w.Code != 200 {
			t.Fatalf("screener: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(body.Data) != 1 || body.Data[0]["symbol"] != "AAPL" {
			t.Fatalf("screener data: %+v", body.Data)
		}
	})

	t.Run("compare all-or-nothing", func(t *testing.T) {
		if w := get("/api/v1/compare?symbols=AAPL,BAD$SYM"); w.Code != 400 {
			t.Fatalf("mixed valid+invalid compare: %d, want 400", w.Code)
		}
		if w := get("/api/v1/compare?symbols=ZZZZ,ZZZ1"); w.Code != 404 {
			t.Fatalf("all unknown compare: %d, want 404", w.Code)
		}
	})
}
