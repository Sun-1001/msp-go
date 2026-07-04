package numutil

import "math"

// Percent returns count / total * 100, or 0 when total is not positive.
func Percent(total int, count int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

// RoundPlaces rounds value to the requested number of decimal places.
func RoundPlaces(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
