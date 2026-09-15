package analytics

import "math"

// DailyReturn computes close-to-close returns: prices[i]/prices[i-1] - 1.
// Input must have >= 2 prices; output has len(prices)-1 elements.
func DailyReturn(prices []float64) ([]float64, error) {
	if len(prices) < 2 {
		return nil, ErrInsufficientData
	}
	if err := checkPositive(prices); err != nil {
		return nil, err
	}
	out := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		out[i-1] = prices[i]/prices[i-1] - 1
	}
	return out, nil
}

// SMA computes the simple moving average with the given window.
// Output has len(prices)-window+1 elements, aligned so that element i
// corresponds to prices[i+window-1] being the newest close in the window.
func SMA(prices []float64, window int) ([]float64, error) {
	if window < 1 {
		return nil, ErrInvalidInput
	}
	if len(prices) < window {
		return nil, ErrInsufficientData
	}
	if err := checkPositive(prices); err != nil {
		return nil, err
	}
	out := make([]float64, len(prices)-window+1)
	sum := 0.0
	for i, p := range prices {
		sum += p
		if i >= window {
			sum -= prices[i-window]
		}
		if i >= window-1 {
			out[i-window+1] = sum / float64(window)
		}
	}
	return out, nil
}

// EMA computes the exponential moving average seeded with the SMA of the
// first `window` closes, then EMA_t = close*alpha + EMA_{t-1}*(1-alpha)
// with alpha = 2/(window+1). Output has len(prices)-window+1 elements.
func EMA(prices []float64, window int) ([]float64, error) {
	sma, err := SMA(prices, window)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(sma))
	out[0] = sma[0]
	alpha := 2 / float64(window+1)
	for i := 1; i < len(out); i++ {
		out[i] = prices[i+window-1]*alpha + out[i-1]*(1-alpha)
	}
	return out, nil
}

// RSI computes the Relative Strength Index with the classic Wilder's
// smoothing (period = 14 by convention; caller passes it). The first
// average gain/loss is a simple mean over the first `period` deltas, then
// Wilder smoothing: avg = (prev*(period-1) + cur)/period. Output has
// len(prices)-period elements. Flat prices yield a neutral 50; a period
// with gains only yields 100.
func RSI(prices []float64, period int) ([]float64, error) {
	if period < 1 {
		return nil, ErrInvalidInput
	}
	if len(prices) < period+1 {
		return nil, ErrInsufficientData
	}
	if err := checkPositive(prices); err != nil {
		return nil, err
	}

	deltas := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		deltas[i-1] = prices[i] - prices[i-1]
	}

	avgGain, avgLoss := 0.0, 0.0
	for _, d := range deltas[:period] {
		if d > 0 {
			avgGain += d
		} else {
			avgLoss -= d
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	out := make([]float64, 0, len(deltas)-period+1)
	out = append(out, rsiValue(avgGain, avgLoss))
	for _, d := range deltas[period:] {
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out = append(out, rsiValue(avgGain, avgLoss))
	}
	return out, nil
}

// volatilitySeries computes the sample standard deviation (ddof=1) of
// daily returns over a trailing window. Output has len(prices)-window
// elements aligned so element i covers returns of prices[i]..prices[i+window].
func volatilitySeries(prices []float64, window int) ([]float64, error) {
	if window < 1 {
		return nil, ErrInvalidInput
	}
	if len(prices) < window+1 {
		return nil, ErrInsufficientData
	}
	if err := checkPositive(prices); err != nil {
		return nil, err
	}
	ret, err := DailyReturn(prices)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(ret)-window+1)
	for i := range out {
		mean := 0.0
		for _, r := range ret[i : i+window] {
			mean += r
		}
		mean /= float64(window)
		var sq float64
		for _, r := range ret[i : i+window] {
			d := r - mean
			sq += d * d
		}
		out[i] = math.Sqrt(sq / float64(window-1))
	}
	return out, nil
}

// Volatility returns the daily volatility (sample std of returns, window=20
// by convention via caller) at the most recent window as a single value --
// the non-annualized daily figure. Need >= window+1 prices.
func Volatility(prices []float64, window int) (float64, error) {
	series, err := volatilitySeries(prices, window)
	if err != nil {
		return 0, err
	}
	return series[len(series)-1], nil
}

// AverageVolume computes the rolling mean of volumes over the window.
// Output has len(volumes)-window+1 elements aligned like SMA.
func AverageVolume(volumes []float64, window int) ([]float64, error) {
	if window < 1 {
		return nil, ErrInvalidInput
	}
	if len(volumes) < window {
		return nil, ErrInsufficientData
	}
	if err := checkPositive(volumes); err != nil {
		return nil, err
	}
	out := make([]float64, len(volumes)-window+1)
	sum := 0.0
	for i, v := range volumes {
		sum += v
		if i >= window {
			sum -= volumes[i-window]
		}
		if i >= window-1 {
			out[i-window+1] = sum / float64(window)
		}
	}
	return out, nil
}

// rsiValue converts average gain/loss into the 0..100 RSI reading.
func rsiValue(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // flat prices, neutral
		}
		return 100 // no losses in window
	}
	return 100 - 100/(1+avgGain/avgLoss)
}
