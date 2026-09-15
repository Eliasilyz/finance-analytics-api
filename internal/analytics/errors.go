// Package-level sentinel errors shared by all analytics functions.
package analytics

import "errors"

var (
	// ErrInsufficientData means the input has fewer points than the
	// window/period the metric requires.
	ErrInsufficientData = errors.New("analytics: insufficient data")

	// ErrInvalidInput means a parameter or a price/volume value is not a
	// usable number (e.g. window < 1, non-positive price).
	ErrInvalidInput = errors.New("analytics: invalid input")

	// ErrUndefined means the ratio has no mathematical meaning for the
	// given inputs (division by zero, non-positive denominator).
	ErrUndefined = errors.New("analytics: undefined ratio")
)
