package analytics

import (
	"errors"
	"testing"
)

func TestROE(t *testing.T) {
	// net income 10M, equity 50M -> 0.20 (20%)
	got, err := ROE(10, 50)
	if err != nil || !almostEqual(got, 0.20, 1e-9) {
		t.Fatalf("ROE(10,50) = %v,%v; want 0.20,nil", got, err)
	}

	// negative net income -> negative ROE, valid
	got, err = ROE(-5, 50)
	if err != nil || !almostEqual(got, -0.10, 1e-9) {
		t.Fatalf("ROE(-5,50) = %v,%v; want -0.10,nil", got, err)
	}

	for _, tc := range []struct {
		name   string
		ni, eq float64
	}{
		{"equity zero", 10, 0},
		{"equity negative", 10, -50},
	} {
		if _, err := ROE(tc.ni, tc.eq); !errors.Is(err, ErrUndefined) {
			t.Fatalf("ROE(%s) err = %v; want ErrUndefined", tc.name, err)
		}
	}
}

func TestROA(t *testing.T) {
	// net income 3M, assets 40M -> 0.075
	got, err := ROA(3, 40)
	if err != nil || !almostEqual(got, 0.075, 1e-9) {
		t.Fatalf("ROA(3,40) = %v,%v; want 0.075,nil", got, err)
	}

	for _, tc := range []struct {
		name  string
		ni, a float64
	}{
		{"assets zero", 3, 0},
		{"assets negative", 3, -40},
	} {
		if _, err := ROA(tc.ni, tc.a); !errors.Is(err, ErrUndefined) {
			t.Fatalf("ROA(%s) err = %v; want ErrUndefined", tc.name, err)
		}
	}
}

func TestNetMargin(t *testing.T) {
	// net income 5, revenue 100 -> 0.05
	got, err := NetMargin(5, 100)
	if err != nil || !almostEqual(got, 0.05, 1e-9) {
		t.Fatalf("NetMargin(5,100) = %v,%v; want 0.05,nil", got, err)
	}

	// negative net income -> negative margin, valid
	got, err = NetMargin(-5, 100)
	if err != nil || !almostEqual(got, -0.05, 1e-9) {
		t.Fatalf("NetMargin(-5,100) = %v,%v; want -0.05,nil", got, err)
	}

	if _, err = NetMargin(5, 0); !errors.Is(err, ErrUndefined) {
		t.Fatalf("NetMargin(5,0) err = %v; want ErrUndefined", err)
	}
}
