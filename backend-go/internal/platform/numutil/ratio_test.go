package numutil

import "testing"

func TestRatio(t *testing.T) {
	tests := []struct {
		name  string
		total int
		count int
		want  float64
	}{
		{name: "positive total", total: 5, count: 3, want: 0.6},
		{name: "zero total", total: 0, count: 3, want: 0},
		{name: "negative total", total: -1, count: 3, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Ratio(tt.total, tt.count); got != tt.want {
				t.Fatalf("Ratio(%d, %d) = %v, want %v", tt.total, tt.count, got, tt.want)
			}
		})
	}
}
