package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) analytics(c *gin.Context) {
	res, err := h.qs.GetAnalytics(c.Request.Context(), c.Param("symbol"))
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	writeData(c.Writer, http.StatusOK, gin.H{
		"symbol": res.Symbol,
		"price":  res.Price,
		"metrics": gin.H{
			"pe":              res.Metrics.PERatio,
			"pb":              res.Metrics.PriceToBook,
			"roe":             res.Metrics.ROE,
			"roa":             res.Metrics.ROA,
			"net_margin":      res.Metrics.NetMargin,
			"current_ratio":   res.Metrics.CurrentRatio,
			"debt_to_equity":  res.Metrics.DebtToEquity,
			"revenue_growth":  res.Metrics.RevenueGrowth,
			"earnings_growth": res.Metrics.EarningsGrowth,
		},
	})
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
