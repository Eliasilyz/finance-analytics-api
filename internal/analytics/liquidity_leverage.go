package analytics

// CurrentRatio returns current assets / current liabilities.
// Liabilities <= 0 is undefined.
func CurrentRatio(currentAssets, currentLiabilities float64) (float64, error) {
	return ratio(currentAssets, currentLiabilities, false)
}

// DebtToEquity returns total debt / total shareholder equity.
// Equity <= 0 is undefined (the ratio has no meaning with a
// non-positive or zero equity base).
func DebtToEquity(totalDebt, equity float64) (float64, error) {
	return ratio(totalDebt, equity, false)
}
