package analytics

import (
	"errors"
	"testing"
)

func TestDailyReturn(t *testing.T) {
	got, err := DailyReturn([]float64{10, 11, 10.5})
	if err != nil {
		t.Fatalf("DailyReturn err = %v", err)
	}
	want := []float64{0.1, -0.045454545454545456}
	if len(got) != len(want) {
		t.Fatalf("DailyReturn len = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if !almostEqual(got[i], want[i], 1e-9) {
			t.Fatalf("DailyReturn[%d] = %v; want %v", i, got[i], want[i])
		}
	}

	if _, err := DailyReturn([]float64{10}); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("DailyReturn single err = %v; want ErrInsufficientData", err)
	}
}

func TestSMA(t *testing.T) {
	got, err := SMA([]float64{1, 2, 3, 4, 5}, 3)
	if err != nil {
		t.Fatalf("SMA err = %v", err)
	}
	want := []float64{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("SMA len = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if !almostEqual(got[i], want[i], 1e-9) {
			t.Fatalf("SMA[%d] = %v; want %v", i, got[i], want[i])
		}
	}

	// SMA200 on 50 days -> ErrInsufficientData, no panic/misleading value
	if _, err := SMA(make([]float64, 50), 200); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("SMA200 on 50 pts err = %v; want ErrInsufficientData", err)
	}
	if _, err := SMA(nil, 20); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("SMA nil err = %v; want ErrInsufficientData", err)
	}
	if _, err := SMA([]float64{1, 2, 3}, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SMA window 0 err = %v; want ErrInvalidInput", err)
	}
	if _, err := SMA([]float64{1, 0, 3}, 2); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SMA non-positive price err = %v; want ErrInvalidInput", err)
	}
}

func TestEMA(t *testing.T) {
	// seed = SMA3([12,13,14]) = 13; alpha=2/4=0.5
	// EMA = [13, 15*0.5+13*0.5=14, 16*0.5+14*0.5=15]
	got, err := EMA([]float64{12, 13, 14, 15, 16}, 3)
	if err != nil {
		t.Fatalf("EMA err = %v", err)
	}
	want := []float64{13, 14, 15}
	if len(got) != len(want) {
		t.Fatalf("EMA len = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if !almostEqual(got[i], want[i], 1e-9) {
			t.Fatalf("EMA[%d] = %v; want %v", i, got[i], want[i])
		}
	}
	if _, err := EMA([]float64{1, 2, 3}, 5); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("EMA short data err = %v; want ErrInsufficientData", err)
	}
}

func TestRSI(t *testing.T) {
	// Classic Wilder 14-period example series; known first RSI ~70.46
	prices := []float64{44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10,
		45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	got, err := RSI(prices, 14)
	if err != nil {
		t.Fatalf("RSI err = %v", err)
	}
	wantFirst := 70.46411904891177
	if len(got) != 1 {
		t.Fatalf("RSI len = %d; want 1", len(got))
	}
	if !almostEqual(got[0], wantFirst, 1e-3) {
		t.Fatalf("RSI[0] = %v; want ~%v", got[0], wantFirst)
	}

	// flat prices -> neutral 50
	flat := make([]float64, 20)
	for i := range flat {
		flat[i] = 100
	}
	got, err = RSI(flat, 14)
	if err != nil || got[0] != 50 {
		t.Fatalf("RSI flat = %v,%v; want 50,nil", got, err)
	}

	// monotonically rising -> 100
	rising := make([]float64, 20)
	for i := range rising {
		rising[i] = float64(i + 1)
	}
	got, err = RSI(rising, 14)
	if err != nil || got[0] != 100 {
		t.Fatalf("RSI rising = %v,%v; want 100,nil", got, err)
	}

	// needs at least period+1 prices
	if _, err := RSI(make([]float64, 14), 14); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("RSI short data err = %v; want ErrInsufficientData", err)
	}
}

func TestVolatility(t *testing.T) {
	prices := []float64{10, 11, 12, 13, 14}
	got, err := Volatility(prices, 3)
	if err != nil {
		t.Fatalf("Volatility err = %v", err)
	}
	// sample std of last 3 returns [0.0909, 0.0833, 0.0769] ~ 0.0070011
	if !almostEqual(got, 0.007001096072626301, 1e-9) {
		t.Fatalf("Volatility = %v; want 0.0070011", got)
	}

	// need window+1 prices
	if _, err := Volatility(prices, 5); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("Volatility short data err = %v; want ErrInsufficientData", err)
	}
	if _, err := Volatility(prices, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Volatility window 0 err = %v; want ErrInvalidInput", err)
	}
}

func TestAverageVolume(t *testing.T) {
	got, err := AverageVolume([]float64{100, 110, 120, 130}, 3)
	if err != nil {
		t.Fatalf("AverageVolume err = %v", err)
	}
	want := []float64{110, 120}
	if len(got) != len(want) {
		t.Fatalf("AverageVolume len = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if !almostEqual(got[i], want[i], 1e-9) {
			t.Fatalf("AverageVolume[%d] = %v; want %v", i, got[i], want[i])
		}
	}
	if _, err := AverageVolume([]float64{100}, 3); !errors.Is(err, ErrInsufficientData) {
		t.Fatalf("AverageVolume short err = %v; want ErrInsufficientData", err)
	}
}
