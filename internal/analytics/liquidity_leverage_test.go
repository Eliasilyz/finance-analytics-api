package analytics

import (
	"errors"
	"testing"
)

func TestCurrentRatio(t *testing.T) {
	// assets 2M, liabilities 1M -> 2
	got, err := CurrentRatio(2, 1)
	if err != nil || !almostEqual(got, 2, 1e-9) {
		t.Fatalf("CurrentRatio(2,1) = %v,%v; want 2,nil", got, err)
	}

	for _, tc := range []struct {
		name string
		a, l float64
	}{
		{"liabilities zero", 2, 0},
		{"liabilities negative", 2, -1},
	} {
		if _, err := CurrentRatio(tc.a, tc.l); !errors.Is(err, ErrUndefined) {
			t.Fatalf("CurrentRatio(%s) err = %v; want ErrUndefined", tc.name, err)
		}
	}
}

func TestDebtToEquity(t *testing.T) {
	// debt 600K, equity 800K -> 0.75
	got, err := DebtToEquity(0.6, 0.8)
	if err != nil || !almostEqual(got, 0.75, 1e-9) {
		t.Fatalf("DebtToEquity(0.6,0.8) = %v,%v; want 0.75,nil", got, err)
	}

	for _, tc := range []struct {
		name string
		d, e float64
	}{
		{"equity zero", 0.6, 0},
		{"equity negative", 0.6, -0.8},
	} {
		if _, err := DebtToEquity(tc.d, tc.e); !errors.Is(err, ErrUndefined) {
			t.Fatalf("DebtToEquity(%s) err = %v; want ErrUndefined", tc.name, err)
		}
	}
}
