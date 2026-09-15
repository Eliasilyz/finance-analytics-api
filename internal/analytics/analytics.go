// Package analytics contains pure financial and technical metric functions.
// No I/O allowed here: no database, no HTTP, no filesystem.
//
// Conventions:
//   - fundamental ratios: func X(..., denominator float64) (float64, error)
//   - price-based indicators: func X(prices []float64, window int) ([]float64, error)
//     returning the full series, error when data is insufficient/invalid.
package analytics

// ratio computes num/den with denominator guards.
// allowNegative=true permits negative denominators (only zero is rejected),
// used where a negative denominator still has meaning; otherwise any
// denominator <= 0 makes the ratio undefined.
func ratio(num, den float64, allowNegative bool) (float64, error) {
	if den == 0 || (!allowNegative && den < 0) {
		return 0, ErrUndefined
	}
	return num / den, nil
}

// checkPositive rejects any non-positive price/volume element.
func checkPositive(v []float64) error {
	for _, x := range v {
		if x <= 0 {
			return ErrInvalidInput
		}
	}
	return nil
}
