package analytics

// ROE returns return on equity = net income / total shareholder equity.
// Equity <= 0 is undefined. Negative net income yields negative ROE (valid).
func ROE(netIncome, equity float64) (float64, error) {
	return ratio(netIncome, equity, false)
}

// ROA returns return on assets = net income / total assets.
// Total assets <= 0 is undefined.
func ROA(netIncome, totalAssets float64) (float64, error) {
	return ratio(netIncome, totalAssets, false)
}

// NetMargin returns net margin = net income / total revenue.
// Revenue == 0 is undefined. Negative values on either side are valid.
func NetMargin(netIncome, revenue float64) (float64, error) {
	return ratio(netIncome, revenue, true)
}
