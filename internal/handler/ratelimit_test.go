package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/gin-gonic/gin"
)

type fakeLimiter struct {
	allow bool
	err   error
}

func (f *fakeLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return f.allow, f.err
}

func newRateLimitedRouter(limiter *fakeLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := New(service.NewQueryService(&queryStoreStub{}), slog.New(slog.DiscardHandler), limiter)
	e := gin.New()
	h.Routes(e)
	return e
}

func TestRateLimitAllowsWithinWindow(t *testing.T) {
	r := newRateLimitedRouter(&fakeLimiter{allow: true, err: nil})
	w := doGet(t, r, "/api/v1/companies")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRateLimitBlocksAtThreshold(t *testing.T) {
	r := newRateLimitedRouter(&fakeLimiter{allow: false, err: nil})
	w := doGet(t, r, "/api/v1/companies")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"RATE_LIMITED"`) {
		t.Fatalf("expected RATE_LIMITED envelope, got %s", w.Body.String())
	}
}

func TestRateLimitFailsOpenWhenLimiterDown(t *testing.T) {
	r := newRateLimitedRouter(&fakeLimiter{err: errFakeDown})
	w := doGet(t, r, "/api/v1/companies")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fail open)", w.Code)
	}
}

func TestRateLimitSkipsHealth(t *testing.T) {
	r := newRateLimitedRouter(&fakeLimiter{allow: false, err: nil})
	w := doGet(t, r, "/health")
	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200 (must bypass rate limit)", w.Code)
	}
}

var errFakeDown = errors.New("limiter down")
