// Package mocks provides fake implementations of app interfaces for tests.
package mocks

import (
	"context"

	"github.com/eliasilyz/finance-analytics-api/internal/provider"
)

// MockProvider is a scriptable FinancialDataProvider. Set Snapshot and/or Err to
// drive each test scenario (success, timeout, rate limit, invalid data, error).
type MockProvider struct {
	Snapshot provider.CompanySnapshot
	Err      error
}

// FetchCompanySnapshot returns the configured snapshot or error.
func (m *MockProvider) FetchCompanySnapshot(ctx context.Context, symbol string) (provider.CompanySnapshot, error) {
	if m.Err != nil {
		return provider.CompanySnapshot{}, m.Err
	}
	return m.Snapshot, nil
}
