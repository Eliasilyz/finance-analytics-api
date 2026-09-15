// Package handler contains the HTTP handlers for the REST API.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/eliasilyz/finance-analytics-api/internal/rate"
	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler wires the QueryService behind the REST routes.
type Handler struct {
	qs      *service.QueryService
	limiter rate.Limiter
	log     *slog.Logger
}

// New builds a Handler. A limiter is optional: when given, every /api/v1
// request is counted against it and exceeding the limit yields 429
// RATE_LIMITED before the handler runs.
func New(qs *service.QueryService, log *slog.Logger, limiter ...rate.Limiter) *Handler {
	h := &Handler{qs: qs, log: log}
	if len(limiter) > 0 {
		h.limiter = limiter[0]
	}
	return h
}

// rateLimit guards the /api/v1 group. Policy: counts EVERY request regardless
// of how the handler answers later (a malformed request still consumes a
// slot), and fails open when the limiter is unreachable so a Redis outage can
// never take the whole API down. /health is outside the group and unaffected.
func (h *Handler) rateLimit(c *gin.Context) {
	if h.limiter == nil {
		c.Next()
		return
	}
	ok, err := h.limiter.Allow(c.Request.Context(), c.ClientIP())
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

	api := r.Group("/api/v1", h.rateLimit)
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
