package ui

import "testing"

// TestFilledCells covers the three call-site parameterisations: the dot
// timeline (timelineWidth, minFilled 0), the separation bar
// (separationBarWidth, minFilled 0), and GainBar (gainBarWidth, minFilled 1,
// its clipping override lives outside the helper).
func TestFilledCells(t *testing.T) {
	tests := []struct {
		name      string
		frac      float64
		width     int
		minFilled int
		want      int
	}{
		// Dot timeline: width 8, minFilled 0.
		{"timeline empty", 0, timelineWidth, 0, 0},
		{"timeline full", 1, timelineWidth, 0, timelineWidth},
		{"timeline mid rounds up", 0.5, timelineWidth, 0, 4},
		{"timeline rounds nearest", 0.3, timelineWidth, 0, 2},
		{"timeline below range clamps", -0.5, timelineWidth, 0, 0},
		{"timeline above range clamps", 1.5, timelineWidth, 0, timelineWidth},

		// Separation bar: width 3, minFilled 0.
		{"separation empty", 0, separationBarWidth, 0, 0},
		{"separation full", 1, separationBarWidth, 0, separationBarWidth},
		{"separation mid", 0.5, separationBarWidth, 0, 2},
		{"separation small fraction rounds down", 0.1, separationBarWidth, 0, 0},
		{"separation above range clamps", 2, separationBarWidth, 0, separationBarWidth},

		// GainBar: width 5, minFilled 1 (one-pip floor).
		{"gain zero pins one pip", 0, gainBarWidth, 1, 1},
		{"gain full", 1, gainBarWidth, 1, gainBarWidth},
		{"gain mid rounds up", 0.5, gainBarWidth, 1, 3},
		{"gain tiny fraction pins one pip", 0.05, gainBarWidth, 1, 1},
		{"gain below range pins one pip", -1, gainBarWidth, 1, 1},
		{"gain above range clamps", 1.2, gainBarWidth, 1, gainBarWidth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filledCells(tc.frac, tc.width, tc.minFilled)
			if got != tc.want {
				t.Fatalf("filledCells(%v, %d, %d) = %d, want %d",
					tc.frac, tc.width, tc.minFilled, got, tc.want)
			}
		})
	}
}
