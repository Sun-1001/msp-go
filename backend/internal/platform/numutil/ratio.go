package numutil

// Ratio returns count / total, or 0 when total is not positive.
func Ratio(total int, count int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) / float64(total)
}
