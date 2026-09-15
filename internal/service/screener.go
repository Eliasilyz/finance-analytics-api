package service

import (
	"context"
	"fmt"

	"github.com/eliasilyz/finance-analytics-api/internal/models"
)

// Screener filters companies by live fundamental criteria computed in SQL.
// Returns an empty slice (never an error) when no company matches. All
// criteria are already typed and finite at this point (handler parses and
// validates them); nil fields mean "no filter".
func (s *QueryService) Screener(ctx context.Context, filter models.ScreenerFilter) ([]models.ScreenerResult, error) {
	rows, err := s.store.Screener(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("screener: %w", err)
	}
	if rows == nil {
		rows = []models.ScreenerResult{}
	}
	return rows, nil
}
