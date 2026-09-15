package analytics

// RevenueGrowth returns YoY revenue growth = (latest - prev) / prev.
// A non-positive base (prev <= 0) is mathematically undefined, so it
// returns ErrUndefined instead of a misleading number.
func RevenueGrowth(prev, latest float64) (float64, error) {
	return growth(prev, latest)
}

// EarningsGrowth returns YoY earnings growth = (latest - prev) / prev.
// Same undefined rule as RevenueGrowth.
func EarningsGrowth(prev, latest float64) (float64, error) {
	return growth(prev, latest)
}

func growth(prev, latest float64) (float64, error) {
	if prev <= 0 {
		return 0, ErrUndefined
	}
	return (latest - prev) / prev, nil
}
