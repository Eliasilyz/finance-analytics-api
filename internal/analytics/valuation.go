package analytics

// PERatio returns price-to-earnings using diluted EPS of the latest fiscal year.
// Negative EPS yields a negative P/E (company loses money), which is valid.
// EPS == 0 makes the ratio undefined.
func PERatio(price, dilutedEPS float64) (float64, error) {
	if price <= 0 {
		return 0, ErrInvalidInput
	}
	return ratio(price, dilutedEPS, true)
}

// PriceToBook returns price-to-book where book value per share =
// totalEquity / sharesOutstanding. SharesOutstanding <= 0 or equity <= 0
// (including missing data surfacing as 0) is undefined.
func PriceToBook(price, totalEquity, sharesOutstanding float64) (float64, error) {
	if price <= 0 {
		return 0, ErrInvalidInput
	}
	if sharesOutstanding <= 0 || totalEquity <= 0 {
		return 0, ErrUndefined
	}
	return price / (totalEquity / sharesOutstanding), nil
}
