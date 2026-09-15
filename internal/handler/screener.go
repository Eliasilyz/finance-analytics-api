package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

func (h *Handler) screener(c *gin.Context) {
	filter := models.ScreenerFilter{}

	if v := c.Query("sector"); v != "" {
		filter.Sector = &v
	}
	for _, pair := range []struct {
		param string
		dst   **float64
	}{
		{"min_roe", &filter.MinROE},
		{"max_pe", &filter.MaxPE},
		{"min_revenue_growth", &filter.MinRevenueGrowth},
	} {
		if raw := c.Query(pair.param); raw != "" {
			f, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				writeError(c.Writer, http.StatusBadRequest, codeValidation,
					pair.param+" must be a finite number")
				return
			}
			*pair.dst = &f
		}
	}

	rows, err := h.qs.Screener(c.Request.Context(), filter)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"symbol":         r.Symbol,
			"name":           r.Name,
			"sector":         r.Sector,
			"roe":            r.ROE,
			"pe":             r.PE,
			"revenue_growth": r.RevenueGrowth,
		})
	}
	writeData(c.Writer, http.StatusOK, out)
}
