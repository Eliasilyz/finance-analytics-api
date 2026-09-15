package analytics

import (
	"errors"
	"math"
	"testing"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestPERatio(t *testing.T) {
	got, err := PERatio(150, 5)
	if err != nil || !almostEqual(got, 30, 1e-9) {
		t.Fatalf("PERatio(150,5) = %v,%v; want 30,nil", got, err)
	}

	// negative EPS -> negative P/E, still valid
	got, err = PERatio(100, -4)
	if err != nil || !almostEqual(got, -25, 1e-9) {
		t.Fatalf("PERatio(100,-4) = %v,%v; want -25,nil", got, err)
	}

	if _, err = PERatio(100, 0); !errors.Is(err, ErrUndefined) {
		t.Fatalf("PERatio(100,0) err = %v; want ErrUndefined", err)
	}
	if _, err = PERatio(0, 5); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PERatio(0,5) err = %v; want ErrInvalidInput", err)
	}
}

func TestPriceToBook(t *testing.T) {
	// BVPS = 1000/100 = 10; P/B = 80/10 = 8
	got, err := PriceToBook(80, 1000, 100)
	if err != nil || !almostEqual(got, 8, 1e-9) {
		t.Fatalf("PriceToBook(80,1000,100) = %v,%v; want 8,nil", got, err)
	}

	cases := []struct {
		name                  string
		price, equity, shares float64
	}{
		{"shares zero", 80, 1000, 0},
		{"shares missing", 80, 1000, 0},
		{"shares negative", 80, 1000, -5},
		{"equity zero", 80, 0, 100},
		{"equity negative", 80, -1000, 100},
	}
	for _, c := range cases {
		if _, err := PriceToBook(c.price, c.equity, c.shares); !errors.Is(err, ErrUndefined) {
			t.Fatalf("PriceToBook %s err = %v; want ErrUndefined", c.name, err)
		}
	}
}
