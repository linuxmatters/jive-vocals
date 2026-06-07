package ui

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// fillColors extracts the distinct RGB foreground triples (38;2;r;g;b) from a
// rendered bar, in order of first appearance, ignoring the empty-track colour.
func fillColors(s string) [][3]int {
	var out [][3]int
	seen := map[[3]int]bool{}
	for seg := range strings.SplitSeq(s, "\x1b[") {
		if !strings.HasPrefix(seg, "38;2;") {
			continue
		}
		body, _, _ := strings.Cut(seg, "m")
		parts := strings.Split(body, ";")
		if len(parts) < 5 {
			continue
		}
		r, err1 := strconv.Atoi(parts[2])
		g, err2 := strconv.Atoi(parts[3])
		b, err3 := strconv.Atoi(parts[4])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		c := [3]int{r, g, b}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// hasColor reports whether the rendered bar contains a given RGB foreground.
func hasColor(s string, r, g, b int) bool {
	return slices.Contains(fillColors(s), [3]int{r, g, b})
}

// durSec returns a time.Duration for a fractional second count.
func durSec(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// triangleColor extracts the RGB foreground applied to the '▲' peak marker, or
// nil if no triangle is present.
func triangleColor(s string) *[3]int {
	for seg := range strings.SplitSeq(s, "\x1b[") {
		if !strings.HasPrefix(seg, "38;2;") {
			continue
		}
		head, rest, _ := strings.Cut(seg, "m")
		if !strings.ContainsRune(rest, '▲') {
			continue
		}
		parts := strings.Split(head, ";")
		if len(parts) < 5 {
			continue
		}
		r, err1 := strconv.Atoi(parts[2])
		g, err2 := strconv.Atoi(parts[3])
		b, err3 := strconv.Atoi(parts[4])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		return &[3]int{r, g, b}
	}
	return nil
}

// TestProgressFillIsGradient asserts the fill is a multi-colour gradient that
// starts at the cyan accent, ends at the violet accent, never uses the red brand
// colour, and has no muddy/grey midpoint.
func TestProgressFillIsGradient(t *testing.T) {
	p := newProgressModel()
	p.SetWidth(meterWidth)
	out := p.ViewAs(0.5)

	colors := fillColors(out)
	// Drop the trailing empty-track colour: it is the dark fill (#444444 dark /
	// #CCCCCC light). The gradient fill must still carry multiple stops.
	if len(colors) < 3 {
		t.Fatalf("expected a multi-colour gradient fill, got %d colours: %v", len(colors), colors)
	}

	// Brand red (#A40000) must not appear anywhere in the fill.
	if hasColor(out, 164, 0, 0) {
		t.Errorf("progress fill contains brand red 38;2;164;0;0:\n%q", out)
	}

	// Start endpoint: bright cyan #00D4FF must appear exactly.
	if !hasColor(out, 0, 212, 255) {
		t.Errorf("progress fill missing cyan start #00D4FF (0,212,255):\n%v", colors)
	}
	// End endpoint: at a partial fill the last cell approaches but need not equal
	// the violet stop #9D4EDD (157,78,221). Assert the final fill colour is close
	// (each channel within 12) and clearly violet (blue dominant, low green).
	fill := colors[:len(colors)-1] // drop trailing empty-track colour
	last := fill[len(fill)-1]
	near := abs(last[0]-157) <= 12 && abs(last[1]-78) <= 12 && abs(last[2]-221) <= 12
	if !near {
		t.Errorf("final fill colour %v not near violet end (157,78,221)", last)
	}

	// Midpoint sanity: at least one mid-gradient colour must stay vivid (channel
	// spread > 40) rather than collapsing to a muddy near-grey.
	vivid := false
	for _, c := range colors {
		lo := min(c[0], min(c[1], c[2]))
		hi := max(c[0], max(c[1], c[2]))
		if hi-lo > 40 {
			vivid = true
			break
		}
	}
	if !vivid {
		t.Errorf("gradient looks muddy (no vivid colour found): %v", colors)
	}
}

// TestMeterIsGradient asserts the audio level meter colours its filled cells as
// a smooth green→yellow→orange→red ramp rather than three flat zones: green-ish
// at the first cell, red-ish at the last, and more than 3 distinct fill colours.
func TestMeterIsGradient(t *testing.T) {
	// Drive the level to the hot end so every cell is filled and coloured.
	out := renderAudioLevelMeter(-1.0, 0.0, 0)

	colors := fillColors(out)
	if len(colors) <= 3 {
		t.Fatalf("expected a gradient (>3 distinct fill colours), got %d: %v", len(colors), colors)
	}

	first := colors[0]
	last := colors[len(colors)-1]

	// First cell green-ish: green channel dominant over red and blue.
	greenDominant := first[1] > first[0] && first[1] > first[2]
	if !greenDominant {
		t.Errorf("first cell %v is not green-dominant", first)
	}
	// Last cell red-ish: red channel dominant over green and blue.
	redDominant := last[0] > last[1] && last[0] > last[2]
	if !redDominant {
		t.Errorf("last cell %v is not red-dominant", last)
	}

	// No muddy/grey midpoint: at least one mid colour stays vivid.
	vivid := false
	for _, c := range colors {
		lo := min(c[0], min(c[1], c[2]))
		hi := max(c[0], max(c[1], c[2]))
		if hi-lo > 40 {
			vivid = true
			break
		}
	}
	if !vivid {
		t.Errorf("meter gradient looks muddy (no vivid colour): %v", colors)
	}
}

// TestMeterHasNoInBarPeakGlyph asserts the peak marker is no longer overlaid
// inside the bar: the bar cells are pure filled/empty gradient with no '|'.
func TestMeterHasNoInBarPeakGlyph(t *testing.T) {
	out := renderAudioLevelMeter(-20.0, -10.0, 0)
	bar := ansi.Strip(out)
	// Take the bar line: the line that contains the gradient cells.
	var barLine string
	for line := range strings.SplitSeq(bar, "\n") {
		if strings.ContainsRune(line, '▓') || strings.ContainsRune(line, '░') {
			barLine = line
			break
		}
	}
	if strings.ContainsRune(barLine, '|') {
		t.Errorf("bar line still contains in-bar peak glyph '|':\n%q", barLine)
	}
}

// TestMeterPeakTriangleAlignsBeneathBar asserts a '▲' marker appears on its own
// line beneath the bar with exactly peakPos leading spaces, for two peak levels.
func TestMeterPeakTriangleAlignsBeneathBar(t *testing.T) {
	cases := []struct {
		peak    float64
		peakPos int
	}{
		{-10.0, 34}, // ((-10+70)/70)*40 = 34.3 -> 34
		{-30.0, 22}, // ((-30+70)/70)*40 = 22.9 -> 22
	}
	for _, tc := range cases {
		out := renderAudioLevelMeter(-40.0, tc.peak, 0)
		plain := ansi.Strip(out)
		var markerLine string
		for line := range strings.SplitSeq(plain, "\n") {
			if strings.ContainsRune(line, '▲') {
				markerLine = line
				break
			}
		}
		if markerLine == "" {
			t.Fatalf("peak=%g: no '▲' marker line found in:\n%q", tc.peak, plain)
		}
		lead := len(markerLine) - len(strings.TrimLeft(markerLine, " "))
		if lead != tc.peakPos {
			t.Errorf("peak=%g: marker leading spaces %d, want peakPos %d\n%q",
				tc.peak, lead, tc.peakPos, markerLine)
		}
		if strings.TrimLeft(markerLine, " ") != "▲" {
			t.Errorf("peak=%g: marker line is not a lone triangle: %q", tc.peak, markerLine)
		}
	}
}

// TestMeterPeakTriangleIsOrange asserts the marker triangle is styled in an
// orange shade (red > green > blue, with a substantial green component so it
// reads as orange rather than pure red).
func TestMeterPeakTriangleIsOrange(t *testing.T) {
	out := renderAudioLevelMeter(-40.0, -10.0, 0)
	c := triangleColor(out)
	if c == nil {
		t.Fatalf("no triangle colour found in:\n%q", out)
	}
	if c[0] <= c[1] || c[1] <= c[2] {
		t.Errorf("triangle colour %v is not an orange shade (want r>g>b)", c)
	}
}

// TestMeterPeakTrianglePulses asserts the marker oscillates between two distinct
// orange shades across pulse phases.
func TestMeterPeakTrianglePulses(t *testing.T) {
	// Trough and crest of the 1.2 Hz sine: t=0 sits mid-rise; pick phases that
	// land near the dim trough and the bright peak.
	// durSec(0.625): sin = -1 -> dim trough. durSec(0.208): sin ≈ +1 -> bright crest.
	dim := triangleColor(renderAudioLevelMeter(-40.0, -10.0, durSec(0.625)))
	bright := triangleColor(renderAudioLevelMeter(-40.0, -10.0, durSec(0.208)))
	if dim == nil || bright == nil {
		t.Fatalf("missing triangle colour: dim=%v bright=%v", dim, bright)
	}
	if *dim == *bright {
		t.Errorf("triangle colour did not change across pulse phases: %v", *dim)
	}
	// Both endpoints must stay clearly orange so the marker never vanishes.
	for _, c := range []*[3]int{dim, bright} {
		if c[0] <= c[1] || c[1] <= c[2] {
			t.Errorf("pulse shade %v is not an orange shade (want r>g>b)", *c)
		}
	}
}

func TestProcessingProgressWidthFitsTerminal(t *testing.T) {
	for _, term := range []int{20, 40, 80, 120, 200} {
		m := NewModel([]string{"a.wav"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: term, Height: 24})
		m = updated.(Model)

		w := m.progress.Width()
		if w < minProgressWidth || w > maxProgressWidth {
			t.Errorf("term=%d progress width %d out of [%d,%d]", term, w, minProgressWidth, maxProgressWidth)
		}

		// Box outer width = bar width + border(2) + padding(2). It must not
		// exceed the terminal unless the bar hit its minimum floor on a narrow
		// terminal.
		box := w + 4
		if box > term && w > minProgressWidth {
			t.Errorf("term=%d box width %d overflows terminal", term, box)
		}
	}
}

// TestProgressWidthCapsAtMeterWidth locks the bar to the meter width on wide
// terminals so its right edge aligns with the audio level meter.
func TestProgressWidthCapsAtMeterWidth(t *testing.T) {
	for _, term := range []int{80, 120, 200} {
		m := NewModel([]string{"a.wav"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: term, Height: 24})
		m = updated.(Model)

		if w := m.progress.Width(); w != meterWidth {
			t.Errorf("term=%d progress width %d, want %d (meterWidth)", term, w, meterWidth)
		}
	}
}

// TestProgressBarAlignsWithMeter renders both the eased bar line and the meter
// line at a normal width and asserts their rendered cell widths match.
func TestProgressBarAlignsWithMeter(t *testing.T) {
	m := NewModel([]string{"recording.wav"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m = updated.(Model)

	barLine := m.progress.ViewAs(0.5)
	barW := ansi.StringWidth(barLine)
	if barW != meterWidth {
		t.Errorf("bar rendered width %d, want %d", barW, meterWidth)
	}

	meter := renderAudioLevelMeter(-20.0, -10.0, 0)
	// The meter's second line is the bar cells; take the widest non-label line.
	var meterW int
	for line := range strings.SplitSeq(meter, "\n") {
		if w := ansi.StringWidth(line); w > meterW {
			meterW = w
		}
	}
	if meterW != meterWidth {
		t.Errorf("meter bar width %d, want %d", meterW, meterWidth)
	}
}

func TestAnalysisProgressWidthFitsTerminal(t *testing.T) {
	for _, term := range []int{20, 40, 80, 120, 200} {
		m := NewAnalysisModel([]string{"a.wav"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: term, Height: 24})
		m = updated.(AnalysisModel)

		w := m.progress.Width()
		if w < minProgressWidth || w > maxProgressWidth {
			t.Errorf("term=%d analysis progress width %d out of [%d,%d]", term, w, minProgressWidth, maxProgressWidth)
		}
	}
}

// TestProcessingRowFitsTerminal renders the full active file detail box and
// asserts no line exceeds the terminal width.
func TestProcessingRowFitsTerminal(t *testing.T) {
	for _, term := range []int{40, 80, 120} {
		m := NewModel([]string{"recording.wav"})
		updated, _ := m.Update(tea.WindowSizeMsg{Width: term, Height: 24})
		m = updated.(Model)
		updated, _ = m.Update(ProgressMsg{FileIndex: 0, Pass: 2, PassName: "Processing", Progress: 0.5})
		m = updated.(Model)

		row := renderFileDetails(m.Files[0], m.progress, -20.0, 0.5)
		for line := range strings.SplitSeq(row, "\n") {
			if w := ansi.StringWidth(line); w > term {
				t.Errorf("term=%d line width %d overflows:\n%q", term, w, line)
			}
		}
	}
}

// TestProgressSpringEases asserts the bar fill eases toward a higher target
// after one tick (moves, but does not snap), and that ticking stops once all
// files complete.
func TestProgressSpringEases(t *testing.T) {
	m := NewModel([]string{"a.wav"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	// Activate the file and set a target progress well ahead of the eased pos.
	updated, _ = m.Update(ProgressMsg{FileIndex: 0, Pass: 2, PassName: "Processing", Progress: 0.8})
	m = updated.(Model)

	const start = 0.0
	const target = 0.8
	if got := m.meters[0].progPos; got != start {
		t.Fatalf("initial progPos = %v, want %v", got, start)
	}

	updated, cmd := m.Update(meterTickMsg{})
	m = updated.(Model)
	if cmd == nil {
		t.Error("tick returned nil cmd while a file is active; loop must continue")
	}

	eased := m.meters[0].progPos
	if !(start < eased && eased < target) {
		t.Errorf("eased progPos %v not strictly between start %v and target %v", eased, start, target)
	}

	// After AllCompleteMsg the model is Done and the tick must not reschedule.
	updated, _ = m.Update(AllCompleteMsg{})
	m = updated.(Model)
	_, cmd = m.Update(meterTickMsg{})
	if cmd != nil {
		t.Error("tick rescheduled after AllCompleteMsg; loop must terminate")
	}
}

// TestProgressSpringIgnoresOutOfRange asserts out-of-range progress messages do
// not panic or disturb spring state.
func TestProgressSpringIgnoresOutOfRange(t *testing.T) {
	m := NewModel([]string{"a.wav"})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	before := m.meters[0].progPos
	updated, _ = m.Update(ProgressMsg{FileIndex: 5, Pass: 2, Progress: 0.9})
	m = updated.(Model)
	updated, _ = m.Update(ProgressMsg{FileIndex: -1, Pass: 2, Progress: 0.9})
	m = updated.(Model)

	if got := m.meters[0].progPos; got != before {
		t.Errorf("out-of-range message disturbed spring state: %v != %v", got, before)
	}
}
