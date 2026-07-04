package numutil

// ClampFloat clamps value between floor and ceiling.
func ClampFloat(value float64, floor float64, ceiling float64) float64 {
	if value < floor {
		return floor
	}
	if value > ceiling {
		return ceiling
	}
	return value
}
