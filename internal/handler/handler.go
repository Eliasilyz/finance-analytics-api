// Package handler contains the HTTP handlers for the REST API.
package handler

import (
	"log/slog"

	"github.com/eliasilyz/finance-analytics-api/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler wires the QueryService behind the REST routes.
type Handler struct {
	qs  *service.QueryService
	log *slog.Logger
}

// New builds a Handler.
func New(qs *service.QueryService, log *slog.Logger) *Handler {
	return &Handler{qs: qs, log: log}
}

// Routes registers every endpoint on the given engine.
func (h *Handler) Routes(r *gin.Engine) *gin.Engine {
	r.GET("/health", func(c *gin.Context) {
		writeData(c.Writer, 200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
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
