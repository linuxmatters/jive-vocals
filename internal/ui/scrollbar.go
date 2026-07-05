package ui

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jive-vocals/internal/cli"
)

// scrollbarThumbGlyph and scrollbarTrackGlyph are the scrollbar cells: a solid
// block for the thumb (the visible slice) and a thin vertical for the track.
const (
	scrollbarThumbGlyph = "█"
	scrollbarTrackGlyph = "│"
)

// scrollHintText is the dim one-line footer shown only while the file queue
// overflows the viewport. It names the scroll inputs in plain British English.
const scrollHintText = "↑/↓ · PgUp/PgDn · scroll to navigate"

// buildScrollbar builds the vertical scrollbar strip for the file-queue
// viewport: vpHeight rows tall, one column wide, a thumb sized to the visible
// fraction of the content and positioned by the scroll percent. It is pure (no
// viewport handle, no styling) so the thumb maths is table-testable; the caller
// styles and joins the returned rows. scrollPercent is the viewport's 0..1
// scroll position. The returned slice always has exactly vpHeight rows (each a
// single glyph), so it joins cleanly to the right of the viewport view.
func buildScrollbar(vpHeight, totalLines, visibleLines int, scrollPercent float64) []string {
	if vpHeight <= 0 {
		return nil
	}

	// Visible fraction of the whole content, clamped to (0,1]. A non-positive
	// total degrades to a full-height thumb so the strip never renders empty.
	visibleFraction := 1.0
	if totalLines > 0 {
		visibleFraction = float64(visibleLines) / float64(totalLines)
	}
	visibleFraction = max(0, min(1, visibleFraction))

	thumbHeight := int(math.Round(float64(vpHeight) * visibleFraction))
	thumbHeight = max(1, min(thumbHeight, vpHeight))

	scrollPercent = max(0, min(1, scrollPercent))
	thumbTop := int(math.Round(float64(vpHeight-thumbHeight) * scrollPercent))
	thumbTop = max(0, min(thumbTop, vpHeight-thumbHeight))

	rows := make([]string, vpHeight)
	for i := range rows {
		if i >= thumbTop && i < thumbTop+thumbHeight {
			rows[i] = scrollbarThumbGlyph
		} else {
			rows[i] = scrollbarTrackGlyph
		}
	}
	return rows
}

// renderScrollbar styles the buildScrollbar rows into a single column block: the
// thumb in the muted fill colour (the same dim tone the empty meter track uses),
// the track in the dimmer ColorFill so the thumb stands out against it.
func renderScrollbar(vpHeight, totalLines, visibleLines int, scrollPercent float64) string {
	rows := buildScrollbar(vpHeight, totalLines, visibleLines, scrollPercent)
	thumbStyle := lipgloss.NewStyle().Foreground(cli.ColorMuted)
	trackStyle := lipgloss.NewStyle().Foreground(cli.ColorFill)

	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		if row == scrollbarThumbGlyph {
			b.WriteString(thumbStyle.Render(row))
		} else {
			b.WriteString(trackStyle.Render(row))
		}
	}
	return b.String()
}

// renderScrollHint renders the dim scroll-hint footer line. The caller decides
// whether to fill it (overflow) or leave it blank (fits); this always returns the
// styled text so the gating lives in one place upstream.
func renderScrollHint() string {
	return lipgloss.NewStyle().Foreground(cli.ColorMuted).Render(scrollHintText)
}
