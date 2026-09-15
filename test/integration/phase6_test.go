//go:build integration

package integration

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/eliasilyz/finance-analytics-api/internal/cache"
	"github.com/eliasilyz/finance-analytics-api/internal/handler"
	"github.com/eliasilyz/finance-analytics-api/internal/models"
	"github.com/eliasilyz/finance-analytics-api/internal/provider"
	"github.com/eliasilyz/finance-analytics-api/internal/rate"
	"github.com/eliasilyz/finance-analytics-api/internal/repository"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/eliasilyz/finance-analytics-api/test/mocks"
)

// seedAAPL inserts the canonical AAPL fixture used across integration tests.
func seedAAPL(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	var cid int64
	if err := db.QueryRowContext(ctx,
		`INSERT INTO companies (symbol,name,exchange,sector,industry,currency)
		 VALUES ('AAPL','Apple Inc.','NASDAQ','Technology','Consumer Electronics','USD') RETURNING id`,
	).Scan(&cid); err != nil {
		t.Fatalf("insert company: %v", err)
	}
	for _, r := range []struct {
		fy  string
		rev float64
		ni  float64
		eps float64
	}{
		{"2024-09-30", 391000000000, 97000000000, 6.30},
		{"2023-09-30", 383000000000, 96900000000, 6.16},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO income_statements (company_id,period,fiscal_date,revenue,net_income,dilated_eps)
			 VALUES ($1,'annual',$2,$3,$4,$5)`, cid, r.fy, r.rev, r.ni, r.eps); err != nil {
			t.Fatalf("insert income: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO balance_sheets (company_id,period,fiscal_date,total_assets,total_liabilities,total_equity,total_debt,current_assets,current_liabilities,shares_outstanding)
		 VALUES ($1,'annual','2024-09-30',352000000000,290000000000,62000000000,100000000000,120000000000,90000000000,16000000000)`,
		cid); err != nil {
		t.Fatalf("insert balance: %v", err)
	}
	start := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		d := start.AddDate(0, 0, i)
		c := 180.0 + float64(i)
		if _, err := db.ExecContext(ctx,
			`INSERT INTO price_history (company_id,date,open,high,low,close,volume)
			 VALUES ($1,$2,$3,$4,$5,$6,1000000)`, cid, d, c-1, c+1, c-1, c); err != nil {
			t.Fatalf("insert price: %v", err)
		}
	}
}

// startRedis boots a Redis container and returns a connected client.
func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(context.Background(),
		testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(context.Background())
	port, _ := c.MappedPort(context.Background(), "6379")
	rdb := redis.NewClient(&redis.Options{
		Addr:         net.JoinHostPort(host, port.Port()),
		DialTimeout:  300 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// countingRepo wraps the real repository and counts expensive read calls so
// tests can verify that cache hits skip recompute.
type countingRepo struct {
	*repository.Repository
	incomeCalls int
	priceCalls  int
	latCalls    int
}

func (c *countingRepo) GetIncomeStatements(ctx context.Context, symbol, period string) ([]models.IncomeStatement, error) {
	c.incomeCalls++
	return c.Repository.GetIncomeStatements(ctx, symbol, period)
}
func (c *countingRepo) GetPriceHistory(ctx context.Context, symbol string, limit int) ([]models.DailyPrice, error) {
	c.priceCalls++
	return c.Repository.GetPriceHistory(ctx, symbol, limit)
}
func (c *countingRepo) GetLatestPrice(ctx context.Context, symbol string) (models.DailyPrice, error) {
	c.latCalls++
	return c.Repository.GetLatestPrice(ctx, symbol)
}

// buildStack wires the real repository, cache, and optional rate limiter into
// a gin router. limit=0 disables the limiter.
func buildStack(t *testing.T, db *sql.DB, rdb *redis.Client, limit int64) (*gin.Engine, *countingRepo, *cache.Redis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	counting := &countingRepo{Repository: repository.NewRepository(db)}
	rcache := cache.NewRedis(rdb, 10*time.Minute)
	qs := service.NewQueryService(counting, rcache)

	var lim *rate.RedisLimiter
	if rdb != nil {
		lim = rate.NewRedis(rdb, limit, time.Hour)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := handler.New(qs, logger, lim)
	r := gin.New()
	h.Routes(r)
	return r, counting, rcache
}

func doRequest(r *gin.Engine, path string) (int, string) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// TestCacheSkipsRecomputeAndInvalidates proves the read-through cache: first
// request computes, second is served from cache (no extra store calls), then
// Ingest invalidates so the next request recomputes with fresh data.
func TestCacheSkipsRecomputeAndInvalidates(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)

	r, counting, rcache := buildStack(t, db, rdb, 100)

	// First request: cache miss, data computed from Postgres.
	code, _ := doRequest(r, "/api/v1/stocks/aapl/analytics")
	if code != http.StatusOK {
		t.Fatalf("analytics warm: %d", code)
	}
	warmIncome := counting.incomeCalls
	if warmIncome == 0 {
		t.Fatal("first request must compute from the store")
	}

	// Second request within TTL: must be served from cache.
	code, _ = doRequest(r, "/api/v1/stocks/aapl/analytics")
	if code != http.StatusOK {
		t.Fatalf("analytics cache: %d", code)
	}
	if counting.incomeCalls != warmIncome {
		t.Fatalf("cache hit should not call the store: %d -> %d", warmIncome, counting.incomeCalls)
	}

	// Ingest fresh data: single new price on a new date → latest close 300,
	// making PE = 300 / 6.30 ≈ 47.619 instead of 239 / 6.30 ≈ 37.937.
	ingest := service.NewIngestionService(
		&mocks.MockProvider{Snapshot: provider.CompanySnapshot{
			Company: models.Company{Symbol: "AAPL", Name: "Apple Inc.", Currency: "USD"},
			Prices: []models.DailyPrice{
				{Date: time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC), Open: 290, High: 305, Low: 288, Close: 300, Volume: 80000000},
			},
		}},
		repository.NewRepository(db),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		rcache,
	)
	if _, err := ingest.Ingest(context.Background(), "aapl"); err != nil {
		t.Fatalf("re-ingest: %v", err)
	}
	if _, err := rdb.Get(context.Background(), "fda:analytics:AAPL").Result(); err != redis.Nil {
		t.Fatalf("analytics key should be gone after ingest, got %v", err)
	}

	// Post-ingest request: store hit again (recompute) with different PE.
	pre := counting.incomeCalls
	code, body := doRequest(r, "/api/v1/stocks/aapl/analytics")
	if code != http.StatusOK {
		t.Fatalf("analytics after ingest: %d %s", code, body)
	}
	if counting.incomeCalls <= pre {
		t.Fatalf("post-ingest must recompute, store calls: %d", counting.incomeCalls)
	}
	if !strings.Contains(body, `"pe":47.619`) {
		t.Fatalf("expected new PE from close=300, got %s", body)
	}
}

// TestRateLimitsAtThreshold proves fixed-window 429 at the real HTTP layer:
// limit=3, 4th request returns RATE_LIMITED envelope.
func TestRateLimitsAtThreshold(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	rdb := startRedis(t)
	seedAAPL(t, db)

	r, _, _ := buildStack(t, db, rdb, 3)
	var lastStatus int
	var lastBody string
	for i := 1; i <= 4; i++ {
		lastStatus, lastBody = doRequest(r, "/api/v1/stocks/aapl")
	}
	if lastStatus != http.StatusTooManyRequests {
		t.Fatalf("4th request: status %d, want 429", lastStatus)
	}
	if !strings.Contains(lastBody, `"code":"RATE_LIMITED"`) {
		t.Fatalf("expected RATE_LIMITED envelope, got %s", lastBody)
	}
}

// TestRedisDownFailsOpen proves that an unreachable Redis does not take the
// API down: cache miss falls back to Postgres, and the rate limiter fails
// open, all without panic or hang.
func TestRedisDownFailsOpen(t *testing.T) {
	m, db := testMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	dead := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  300 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
	})
	t.Cleanup(func() { _ = dead.Close() })
	seedAAPL(t, db)

	r, counting, _ := buildStack(t, db, dead, 3)

	// Cache fail-open: every request recompute against Postgres.
	for i := 0; i < 3; i++ {
		code, _ := doRequest(r, "/api/v1/stocks/aapl/analytics")
		if code != http.StatusOK {
			t.Fatalf("analytics with dead cache: attempt %d status %d", i+1, code)
		}
	}
	if counting.incomeCalls < 3 {
		t.Fatalf("dead cache must recompute every time, store calls = %d", counting.incomeCalls)
	}

	// Rate limiter fail-open: despite limit=3, no request is rejected.
	for i := 1; i <= 5; i++ {
		code, _ := doRequest(r, "/api/v1/stocks/aapl")
		if code == http.StatusTooManyRequests {
			t.Fatalf("dead limiter must fail open, rejected on attempt %d", i)
		}
	}
}
