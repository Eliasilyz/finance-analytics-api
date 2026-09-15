package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxCompareSymbols = 10

func (h *Handler) compare(c *gin.Context) {
	raw := strings.Split(c.Query("symbols"), ",")
	results, err := h.qs.Compare(c.Request.Context(), raw, maxCompareSymbols)
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	out := make([]gin.H, 0, len(results))
	for _, res := range results {
		out = append(out, gin.H{
			"symbol":  res.Symbol,
			"metrics": metricsJSON(res.Metrics),
		})
	}
	writeData(c.Writer, http.StatusOK, out)
}
