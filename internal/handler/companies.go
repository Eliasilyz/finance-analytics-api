package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// listCompanies returns every company as an array (never null).
func (h *Handler) listCompanies(c *gin.Context) {
	companies, err := h.qs.ListCompanies(c.Request.Context())
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	if companies == nil {
		companies = []models.Company{}
	}
	writeData(c.Writer, http.StatusOK, companyListJSON(companies))
}

// company returns a single company by symbol.
func (h *Handler) company(c *gin.Context) {
	company, err := h.qs.GetCompany(c.Request.Context(), c.Param("symbol"))
	if err != nil {
		writeServiceError(c.Writer, h.log, err)
		return
	}
	writeData(c.Writer, http.StatusOK, companyJSON(company))
}

func companyListJSON(companies []models.Company) []gin.H {
	out := make([]gin.H, 0, len(companies))
	for _, cmp := range companies {
		out = append(out, gin.H{
			"symbol":   cmp.Symbol,
			"name":     cmp.Name,
			"exchange": cmp.Exchange,
			"sector":   cmp.Sector,
			"industry": cmp.Industry,
			"currency": cmp.Currency,
		})
	}
	return out
}

func companyJSON(cmp models.Company) gin.H {
	return gin.H{
		"symbol":      cmp.Symbol,
		"name":        cmp.Name,
		"exchange":    cmp.Exchange,
		"sector":      cmp.Sector,
		"industry":    cmp.Industry,
		"currency":    cmp.Currency,
		"description": cmp.Description,
	}
}
