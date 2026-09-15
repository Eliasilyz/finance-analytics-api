package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/service"
)

func (h *Handler) analytics(c *gin.Context) {
	res, err := h.qs.GetAnalytics(c.Request.Context(), c.Param("symbol"))
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	writeData(c.Writer, http.StatusOK, gin.H{
		"symbol":  res.Symbol,
		"price":   res.Price,
		"metrics": metricsJSON(res.Metrics),
	})
}

func metricsJSON(m service.AnalyticsMetrics) gin.H {
	return gin.H{
		"pe":              m.PERatio,
		"pb":              m.PriceToBook,
		"roe":             m.ROE,
		"roa":             m.ROA,
		"net_margin":      m.NetMargin,
		"current_ratio":   m.CurrentRatio,
		"debt_to_equity":  m.DebtToEquity,
		"revenue_growth":  m.RevenueGrowth,
		"earnings_growth": m.EarningsGrowth,
	}
}

func (h *Handler) technical(c *gin.Context) {
	res, err := h.qs.GetTechnical(c.Request.Context(), c.Param("symbol"))
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	writeData(c.Writer, http.StatusOK, gin.H{
		"symbol":    res.Symbol,
		"last_date": res.LastDate.Format(dateFormat),
		"values": gin.H{
			"daily_return":     res.Values.DailyReturn,
			"sma20":            res.Values.SMA20,
			"sma50":            res.Values.SMA50,
			"sma200":           res.Values.SMA200,
			"ema20":            res.Values.EMA20,
			"rsi14":            res.Values.RSI14,
			"volatility20":     res.Values.Volatility20,
			"average_volume20": res.Values.AverageVolume20,
		},
	})
}
