package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

const dateFormat = "2006-01-02"

// parsePeriod normalizes the ?period parameter: empty means annual (the draft
// default), otherwise it must be one of annual|quarterly|all.
func parsePeriod(v string) (string, bool) {
	if v == "" {
		return "annual", true
	}
	switch v {
	case "annual", "quarterly", "all":
		return v, true
	}
	return "", false
}

// parseLimit parses ?limit=n (1..1000). Absent means "all rows". n<=0 on a
// present param is a validation error; ok=false otherwise.
func parseLimit(v string, absent bool) (int, bool) {
	if absent {
		return 0, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 1000 {
		return 0, false
	}
	return n, true
}

func (h *Handler) priceHistory(c *gin.Context) {
	symbol := c.Param("symbol")
	limit, ok := parseLimit(c.Query("limit"), c.Query("limit") == "")
	if !ok {
		writeError(c.Writer, http.StatusBadRequest, codeValidation, "limit must be an integer in [1,1000]")
		return
	}
	prices, err := h.qs.GetPriceHistory(c.Request.Context(), symbol, limit)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	writeData(c.Writer, http.StatusOK, priceListJSON(prices))
}

func (h *Handler) incomeStatements(c *gin.Context) {
	period, ok := parsePeriod(c.Query("period"))
	if !ok {
		writeError(c.Writer, http.StatusBadRequest, codeValidation, "period must be one of annual|quarterly|all")
		return
	}
	items, err := h.qs.GetIncomeStatements(c.Request.Context(), c.Param("symbol"), period)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"period":      it.Period,
			"fiscal_date": it.FiscalDate.Format(dateFormat),
			"revenue":     it.Revenue,
			"net_income":  it.NetIncome,
			"diluted_eps": it.DilutedEPS,
		})
	}
	writeData(c.Writer, http.StatusOK, out)
}

func (h *Handler) balanceSheets(c *gin.Context) {
	period, ok := parsePeriod(c.Query("period"))
	if !ok {
		writeError(c.Writer, http.StatusBadRequest, codeValidation, "period must be one of annual|quarterly|all")
		return
	}
	items, err := h.qs.GetBalanceSheets(c.Request.Context(), c.Param("symbol"), period)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"period":              it.Period,
			"fiscal_date":         it.FiscalDate.Format(dateFormat),
			"total_assets":        it.TotalAssets,
			"total_liabilities":   it.TotalLiabilities,
			"total_equity":        it.TotalEquity,
			"total_debt":          it.TotalDebt,
			"current_assets":      it.CurrentAssets,
			"current_liabilities": it.CurrentLiabilities,
			"shares_outstanding":  it.SharesOutstanding,
		})
	}
	writeData(c.Writer, http.StatusOK, out)
}

func (h *Handler) cashFlows(c *gin.Context) {
	period, ok := parsePeriod(c.Query("period"))
	if !ok {
		writeError(c.Writer, http.StatusBadRequest, codeValidation, "period must be one of annual|quarterly|all")
		return
	}
	items, err := h.qs.GetCashFlows(c.Request.Context(), c.Param("symbol"), period)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, gin.H{
			"period":              it.Period,
			"fiscal_date":         it.FiscalDate.Format(dateFormat),
			"operating_cash_flow": it.OperatingCashFlow,
			"investing_cash_flow": it.InvestingCashFlow,
			"financing_cash_flow": it.FinancingCashFlow,
			"net_change_in_cash":  it.NetChangeInCash,
		})
	}
	writeData(c.Writer, http.StatusOK, out)
}

func priceListJSON(prices []models.DailyPrice) []gin.H {
	out := make([]gin.H, 0, len(prices))
	for _, p := range prices {
		out = append(out, gin.H{
			"date":   p.Date.Format(dateFormat),
			"open":   p.Open,
			"high":   p.High,
			"low":    p.Low,
			"close":  p.Close,
			"volume": p.Volume,
		})
	}
	return out
}
