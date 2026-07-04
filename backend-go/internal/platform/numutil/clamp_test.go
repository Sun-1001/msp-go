package numutil

import (
	"math"
	"testing"
)

func TestClampFloat(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		floor   float64
		ceiling float64
		want    float64
	}{
		{name: "below floor", value: -0.1, floor: 0, ceiling: 1, want: 0},
		{name: "above ceiling", value: 1.2, floor: 0, ceiling: 1, want: 1},
		{name: "within range", value: 0.5, floor: 0, ceiling: 1, want: 0.5},
		{name: "at floor", value: 0, floor: 0, ceiling: 1, want: 0},
		{name: "at ceiling", value: 1, floor: 0, ceiling: 1, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClampFloat(tt.value, tt.floor, tt.ceiling); got != tt.want {
				t.Fatalf("ClampFloat(%v, %v, %v) = %v, want %v", tt.value, tt.floor, tt.ceiling, got, tt.want)
			}
		})
	}
}

func TestClampFloatPreservesNaN(t *testing.T) {
	if got := ClampFloat(math.NaN(), 0, 1); !math.IsNaN(got) {
		t.Fatalf("ClampFloat(NaN, 0, 1) = %v, want NaN", got)
	}
}
