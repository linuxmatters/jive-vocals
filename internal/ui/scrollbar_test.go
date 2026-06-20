package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestBuildScrollbarThumbMaths table-tests the pure scrollbar helper: thumb
// height tracks the visible fraction and thumb position tracks the scroll
// percent. A tall content (small visible fraction) yields a short thumb.
func TestBuildScrollbarThumbMaths(t *testing.T) {
	const vpHeight = 10

	cases := []struct {
		name          string
		totalLines    int
		visibleLines  int
		scrollPercent float64
		wantThumbH    int
		wantThumbTop  int
	}{
		// Half-visible content: thumb is half the strip.
		{"half top", 20, 10, 0.0, 5, 0},
		{"half middle", 20, 10, 0.5, 5, 3}, // round((10-5)*0.5)=3
		{"half bottom", 20, 10, 1.0, 5, 5}, // thumb pinned to the bottom
		// Tall content: a small visible fraction gives a short (floored) thumb.
		{"tall short thumb top", 100, 10, 0.0, 1, 0},
		{"tall short thumb bottom", 100, 10, 1.0, 1, 9}, // (10-1)*1.0
		// Just-overflowing content keeps the thumb near full height.
		{"barely over top", 11, 10, 0.0, 9, 0},    // round(10*10/11)=9
		{"barely over bottom", 11, 10, 1.0, 9, 1}, // (10-9)*1.0
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := buildScrollbar(vpHeight, tc.totalLines, tc.visibleLines, tc.scrollPercent)
			if len(rows) != vpHeight {
				t.Fatalf("strip height = %d, want %d", len(rows), vpHeight)
			}

			// Thumb is the contiguous run of thumb glyphs; locate its top and length.
			top, height := -1, 0
			for i, r := range rows {
				if r == scrollbarThumbGlyph {
					if top == -1 {
						top = i
					}
					height++
				}
			}
			if height != tc.wantThumbH {
				t.Errorf("thumb height = %d, want %d (rows %v)", height, tc.wantThumbH, rows)
			}
			if top != tc.wantThumbTop {
				t.Errorf("thumb top = %d, want %d (rows %v)", top, tc.wantThumbTop, rows)
			}

			// Every non-thumb row is a track glyph; no empty cells.
			for i, r := range rows {
				if r != scrollbarThumbGlyph && r != scrollbarTrackGlyph {
					t.Errorf("row %d = %q, want thumb or track glyph", i, r)
				}
			}
		})
	}
}

// TestBuildScrollbarTallContentShortThumb confirms the thumb shrinks as content
// grows relative to a fixed viewport: more total lines means a shorter thumb.
func TestBuildScrollbarTallContentShortThumb(t *testing.T) {
	const vpHeight = 20

	thumbHeight := func(total int) int {
		rows := buildScrollbar(vpHeight, total, vpHeight, 0.0)
		h := 0
		for _, r := range rows {
			if r == scrollbarThumbGlyph {
				h++
			}
		}
		return h
	}

	short := thumbHeight(40)    // half visible -> ~10
	shorter := thumbHeight(200) // tenth visible -> ~2
	if short <= shorter {
		t.Errorf("taller content should yield a shorter thumb: 40-line=%d, 200-line=%d", short, shorter)
	}
	if shorter < 1 {
		t.Errorf("thumb must floor at 1, got %d", shorter)
	}
}

// TestScrollbarAndHintAbsentWhenContentFits confirms NEITHER the scrollbar thumb
// nor the scroll hint render when the file queue fits the viewport (few files,
// tall terminal).
func TestScrollbarAndHintAbsentWhenContentFits(t *testing.T) {
	m := NewModel([]string{"a.wav", "b.wav"})
	m.Width = 120
	m.Height = 200 // very tall: two queued files cannot overflow
	m.sizeViewport()
	m.refreshViewportContent()

	if m.vp.TotalLineCount() > m.vp.Height() {
		t.Fatalf("test setup: content overflows (%d > %d); expected it to fit",
			m.vp.TotalLineCount(), m.vp.Height())
	}

	plain := ansi.Strip(m.renderScrollingView())

	if strings.Contains(plain, scrollbarThumbGlyph) {
		t.Errorf("scrollbar thumb present when content fits:\n%s", plain)
	}
	if strings.Contains(plain, "to navigate") {
		t.Errorf("scroll hint present when content fits:\n%s", plain)
	}
}

// TestScrollbarAndHintPresentWhenContentOverflows confirms BOTH the scrollbar
// thumb and the scroll hint render when the file queue overflows the viewport
// (many files, short terminal).
func TestScrollbarAndHintPresentWhenContentOverflows(t *testing.T) {
	files := make([]string, 40)
	for i := range files {
		files[i] = "file.wav"
	}
	m := NewModel(files)
	m.Width = 120
	m.Height = 12 // short terminal: 40 queued files overflow
	m.sizeViewport()
	m.refreshViewportContent()

	if m.vp.TotalLineCount() <= m.vp.Height() {
		t.Fatalf("test setup: content fits (%d <= %d); expected it to overflow",
			m.vp.TotalLineCount(), m.vp.Height())
	}

	plain := ansi.Strip(m.renderScrollingView())

	if !strings.Contains(plain, scrollbarThumbGlyph) {
		t.Errorf("scrollbar thumb absent when content overflows:\n%s", plain)
	}
	if !strings.Contains(plain, scrollHintText) {
		t.Errorf("scroll hint absent when content overflows:\n%s", plain)
	}
}

// TestScrollingViewWidthStableAcrossOverflow confirms the rendered width does not
// change between the fits and overflows cases: the scrollbar column and hint row
// are reserved unconditionally, so the file boxes never reflow when they toggle.
func TestScrollingViewWidthStableAcrossOverflow(t *testing.T) {
	width, height := 120, 14

	fits := NewModel([]string{"a.wav", "b.wav"})
	fits.Width, fits.Height = width, height
	fits.sizeViewport()
	fits.refreshViewportContent()
	if fits.vp.TotalLineCount() > fits.vp.Height() {
		t.Fatalf("test setup: fits case overflows")
	}

	manyFiles := make([]string, 40)
	for i := range manyFiles {
		manyFiles[i] = "file.wav"
	}
	overflows := NewModel(manyFiles)
	overflows.Width, overflows.Height = width, height
	overflows.sizeViewport()
	overflows.refreshViewportContent()
	if overflows.vp.TotalLineCount() <= overflows.vp.Height() {
		t.Fatalf("test setup: overflow case fits")
	}

	// Both viewports were sized from the same terminal width, so the reserved
	// scrollbar column gives them identical viewport widths regardless of overflow.
	if fits.vp.Width() != overflows.vp.Width() {
		t.Errorf("viewport width differs across overflow: fits=%d overflows=%d",
			fits.vp.Width(), overflows.vp.Width())
	}
}
