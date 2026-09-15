package analytics

import (
	"errors"
	"testing"
)

func TestRevenueGrowth(t *testing.T) {
	// prev 100, latest 120 -> +20%
	got, err := RevenueGrowth(100, 120)
	if err != nil || !almostEqual(got, 0.20, 1e-9) {
		t.Fatalf("RevenueGrowth(100,120) = %v,%v; want 0.20,nil", got, err)
	}

	// decline
	got, err = RevenueGrowth(100, 80)
	if err != nil || !almostEqual(got, -0.20, 1e-9) {
		t.Fatalf("RevenueGrowth(100,80) = %v,%v; want -0.20,nil", got, err)
	}

	for _, tc := range []struct {
		name         string
		prev, latest float64
	}{
		{"base zero", 0, 120},
		{"base negative", -100, 120},
		{"base negative to flat", -100, 0},
	} {
		if _, err := RevenueGrowth(tc.prev, tc.latest); !errors.Is(err, ErrUndefined) {
			t.Fatalf("RevenueGrowth(%s) err = %v; want ErrUndefined", tc.name, err)
		}
	}
}

func TestEarningsGrowth(t *testing.T) {
	// prev 50, latest 62 -> +24%
	got, err := EarningsGrowth(50, 62)
	if err != nil || !almostEqual(got, 0.24, 1e-9) {
		t.Fatalf("EarningsGrowth(50,62) = %v,%v; want 0.24,nil", got, err)
	}

	// profitable growth but negative latest -> negative %, valid
	got, err = EarningsGrowth(100, -50)
	if err != nil || !almostEqual(got, -1.5, 1e-9) {
		t.Fatalf("EarningsGrowth(100,-50) = %v,%v; want -1.5,nil", got, err)
	}

	if _, err := EarningsGrowth(0, 10); !errors.Is(err, ErrUndefined) {
		t.Fatalf("EarningsGrowth(0,10) err = %v; want ErrUndefined", err)
	}
}
