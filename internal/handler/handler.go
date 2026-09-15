// Package handler contains the HTTP handlers for the REST API.
package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/eliasilyz/finance-analytics-api/internal/auth"
	"github.com/eliasilyz/finance-analytics-api/internal/rate"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/gin-gonic/gin"
)

// keyIDContext is the gin context key holding the authenticated key's database
// id, set by authRequire and consumed by rateLimit for per-API-key buckets.
const keyIDContext = "api_key_id"

// Options carries optional middleware wiring. Every field left nil disables
// that middleware: Limiter disables rate limiting, Auth disables authentication,
// AuthGuard disables the per-IP auth-failure guard.
type Options struct {
	// Auth validates the X-API-Key header on every /api/v1 request. Leaving it
	// nil serves the API unauthenticated; production MUST wire it. Routes logs
	// a warning when the handler is built without it.
	Auth *auth.Service
	// Limiter is the per-API-key rate limiter. Every request that passes auth
	// is counted against the authenticated key's bucket.
	Limiter rate.Limiter
	// AuthGuard is a loose per-IP counter for failed-authentication floods. It
	// is NOT general rate limiting: it only counts auth failures per client IP
	// and returns 429 when that IP exceeds the (deliberately high) threshold.
	// Nil disables the guard.
	AuthGuard rate.Limiter
}

// Handler wires the QueryService behind the REST routes.
type Handler struct {
	qs        *service.QueryService
	auth      *auth.Service
	limiter   rate.Limiter
	authGuard rate.Limiter
	log       *slog.Logger
}

// New builds a Handler. Options are optional (zero value works).
func New(qs *service.QueryService, log *slog.Logger, opts ...Options) *Handler {
	h := &Handler{qs: qs, log: log}
	if len(opts) > 0 {
		h.auth = opts[0].Auth
		h.limiter = opts[0].Limiter
		h.authGuard = opts[0].AuthGuard
	}
	if h.auth == nil {
		log.Warn("/api/v1 is running WITHOUT API key authentication; wire Options.Auth in production")
	}
	return h
}

// authRequire validates the X-API-Key header before the handler runs. All
// rejection causes (missing, malformed, unknown, expired, revoked) collapse
// into a uniform 401 so callers cannot probe which condition failed.
func (h *Handler) authRequire(c *gin.Context) {
	if h.auth == nil {
		c.Next()
		return
	}
	key, err := h.auth.Validate(c.Request.Context(), c.GetHeader("X-API-Key"))
	if err != nil {
		if h.blockedByAuthFloodGuard(c) {
			return
		}
		writeError(c.Writer, http.StatusUnauthorized, codeUnauthorized, "missing or invalid API key")
		c.Abort()
		return
	}
	c.Set(keyIDContext, key.ID)
	c.Next()
}

// blockedByAuthFloodGuard reports whether the client IP has tripped the
// auth-failure flood guard and writes a 429 when so. The guard is a jaring
// pengaman against brute-force/DoS on the auth path: a per-IP counter with a
// deliberately loose threshold, hit only on auth failure. It fails OPEN when
// the guard itself is unavailable (Redis down), in line with the Phase 6
// rate-limit policy — a flood-guard outage must never mask the 401 that auth
// failure already produces.
func (h *Handler) blockedByAuthFloodGuard(c *gin.Context) bool {
	if h.authGuard == nil {
		return false
	}
	ok, err := h.authGuard.Allow(c.Request.Context(), "authfail:"+c.ClientIP())
	if err != nil {
		h.log.Warn("auth-failure guard unavailable, failing open", "error", err)
		return false
	}
	if !ok {
		writeError(c.Writer, http.StatusTooManyRequests, codeRateLimited, "too many failed auth attempts")
		c.Abort()
		return true
	}
	return false
}

// rateLimit guards /api/v1 per authenticated API key. Policy: every request
// that reaches here is counted against the authenticated key's bucket (Phase 7:
// per-IP bucketing was retired for data endpoints because a NAT/multi-client
// deployment should share one key's quota), and it fails open when the limiter
// is unreachable so a Redis outage can never take the whole API down. /health
// is outside the group and unaffected.
func (h *Handler) rateLimit(c *gin.Context) {
	if h.limiter == nil {
		c.Next()
		return
	}
	key := "ip:" + c.ClientIP()
	if id, ok := c.Get(keyIDContext); ok {
		key = fmt.Sprintf("key:%d", id)
	}
	ok, err := h.limiter.Allow(c.Request.Context(), key)
	if err != nil {
		h.log.Warn("rate limiter unavailable, failing open", "error", err)
		c.Next()
		return
	}
	if !ok {
		writeError(c.Writer, http.StatusTooManyRequests, codeRateLimited, "rate limit exceeded")
		c.Abort()
		return
	}
	c.Next()
}

// Routes registers every endpoint on the given engine.
func (h *Handler) Routes(r *gin.Engine) *gin.Engine {
	r.GET("/health", func(c *gin.Context) {
		writeData(c.Writer, 200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1", h.authRequire, h.rateLimit)
	{
		api.GET("/companies", h.listCompanies)
		api.GET("/companies/:symbol", h.company)

		api.GET("/stocks/:symbol/history", h.priceHistory)
		api.GET("/stocks/:symbol/income-statement", h.incomeStatements)
		api.GET("/stocks/:symbol/balance-sheet", h.balanceSheets)
		api.GET("/stocks/:symbol/cash-flow", h.cashFlows)

		api.GET("/stocks/:symbol/analytics", h.analytics)
		api.GET("/stocks/:symbol/technical", h.technical)

		api.GET("/compare", h.compare)
		api.GET("/screener", h.screener)
	}
	return r
}
