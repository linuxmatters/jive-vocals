package ui

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jive-vocals/internal/cli"
	"github.com/linuxmatters/jive-vocals/internal/processor"
)

// renderProcessingView renders the header and file queue as one unscrolled
// stack. View() drives the live UI through the viewport instead; this is the
// pre-size fallback (before the first WindowSizeMsg builds the viewport) and the
// section-order reference the layout tests assert against.
func renderProcessingView(m Model) string {
	return renderProcessingHeader(m) + "\n" + renderFileQueue(m, m.progress)
}

// renderProcessingHeader renders the fixed header above the scrollable file
// queue: the title and the overall-progress box. View() keeps this outside the
// viewport so it stays pinned while the file list scrolls. The two trailing
// blank lines reproduce the prior gap between the box and the first file entry.
func renderProcessingHeader(m Model) string {
	var b strings.Builder

	// Header (title only)
	b.WriteString(cli.RenderTitle())
	b.WriteString("\n\n")

	// Overall progress box, directly under the title
	b.WriteString(renderOverallProgress(m))
	b.WriteString("\n")

	return b.String()
}

// renderFileQueue renders the list of files with their status
func renderFileQueue(m Model, prog progress.Model) string {
	return renderFileQueueWithActiveEntries(m, prog, nil)
}

func renderFileQueueWithActiveEntries(m Model, prog progress.Model, activeEntries []string) string {
	var b strings.Builder

	for i := range m.Files {
		if rendered := activeEntryAt(activeEntries, i); rendered != "" && fileActive(m.Files[i].Status) {
			b.WriteString(rendered)
			b.WriteString("\n")
			continue
		}

		// Use the eased meter and progress positions for the active display;
		// fall back to the raw values when no spring slot exists.
		easedLevel, easedProgress, easedPeak := m.displayValues(i)
		b.WriteString(renderFileEntryWithCache(&m.Files[i], m.fileEntryCache(i), prog, easedLevel, easedProgress, easedPeak, m.Width))
		b.WriteString("\n")
	}

	return b.String()
}

func activeEntryAt(activeEntries []string, index int) string {
	if index < 0 || index >= len(activeEntries) {
		return ""
	}
	return activeEntries[index]
}

func (m Model) fileEntryCache(index int) *fileEntryCache {
	if index < 0 || index >= len(m.fileEntryCaches) {
		return nil
	}
	return &m.fileEntryCaches[index]
}

func renderFileEntryWithCache(file *FileProgress, cache *fileEntryCache, prog progress.Model, easedLevel, easedProgress, easedPeak float64, termWidth int) string {
	if cache != nil &&
		cache.valid &&
		cache.status == file.Status &&
		cache.termWidth == termWidth &&
		stableFileEntryStatus(file.Status) {
		return cache.rendered
	}
	return renderFileEntry(file, prog, easedLevel, easedProgress, easedPeak, termWidth)
}

// renderFileEntry renders a single file entry in the queue. termWidth gates the
// side-by-side status boxes: they are dropped on narrow terminals so the Pass box
// never wraps.
func renderFileEntry(file *FileProgress, prog progress.Model, easedLevel, easedProgress, easedPeak float64, termWidth int) string {
	return renderFileEntryWithPassBox(file, prog, easedLevel, easedProgress, easedPeak, termWidth, "")
}

func renderFileEntryWithPassBox(file *FileProgress, prog progress.Model, easedLevel, easedProgress, easedPeak float64, termWidth int, passBox string) string {
	fileName := filepath.Base(file.InputPath)

	switch file.Status {
	case StatusComplete:
		return renderDoneBox(*file)

	case StatusAnalysing, StatusProcessing, StatusNormalising:
		// active file with detailed progress, with the filter-chain status boxes
		// joined to the right of the Pass box.
		icon := lipgloss.NewStyle().Foreground(cli.ColorOrange).Render("∿")
		if passBox == "" {
			passBox = renderFileDetails(file, prog, easedLevel, easedProgress, easedPeak)
		}
		body := joinStatusBoxes(passBox, file, termWidth)
		return fmt.Sprintf(" %s %s\n%s", icon, fileName, body)

	case StatusError:
		// ✗ failed file
		icon := lipgloss.NewStyle().Foreground(cli.ColorRed).Render("✗")
		return fmt.Sprintf(" %s %s\n   Error: %v", icon, fileName, file.Error)

	default:
		// ⧗ queued file
		icon := lipgloss.NewStyle().Foreground(cli.ColorMuted).Render("⧗")
		return fmt.Sprintf(" %s %s\n   Queued...", icon, fileName)
	}
}

// fileDetailsBox is the active-file Pass box frame: a sky-blue rounded border with
// horizontal padding. Its inputs are all compile-time constants, so it is identical
// every frame; hoisting it off renderFileDetails keeps the style allocation off the
// 60fps redraw (one active file rebuilds it per frame otherwise).
var fileDetailsBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(cli.ColorSkyBlue).
	Padding(0, 1)

// renderFileDetails renders detailed progress for the active file. easedLevel is
// the spring-smoothed audio level used for the meter display.
func renderFileDetails(file *FileProgress, prog progress.Model, easedLevel, easedProgress, easedPeak float64) string {
	var content strings.Builder

	// Pass indicator. "Pass N/4" sits in the top border (spliced below, matching
	// the Filter Chain / Analysis boxes); the pass name is the first content row.
	var passName string
	switch file.CurrentPass {
	case processor.PassAnalysis:
		passName = "Analysing Audio" //nolint:gosec // G101 false positive: not a credential
	case processor.PassProcessing:
		passName = "Processing Audio"
	case processor.PassMeasuring:
		passName = "Measuring Levels"
	case processor.PassNormalising:
		passName = "Normalising Audio"
	default:
		passName = "Processing"
	}
	fmt.Fprintf(&content, "%s\n", passName)

	// Progress bar (spring-eased fill for smooth motion)
	content.WriteString(prog.ViewAs(easedProgress))
	content.WriteString("\n\n")

	// Time block: elapsed clock, mini dot timeline, projected total clock, and a
	// realtime-speed badge.
	content.WriteString(renderTimeline(*file))
	content.WriteByte('\n')

	// Audio level visualization. Both the displayed level and the peak marker ease
	// toward their targets via springs; the critically-damped peak spring keeps the
	// eased peak from ever exceeding the measured peak-hold value.
	if file.HasLevel {
		content.WriteString("\n")
		content.WriteString(renderAudioLevelMeterWithLevels(file.CurrentLevel, easedLevel, easedPeak, file.ElapsedTime))
	}

	title := fmt.Sprintf("Pass %d/4", file.CurrentPass)
	return overlayBorderTitle(fileDetailsBox.Render(content.String()), title, cli.ColorSkyBlue)
}

// timelineWidth is the cell count of the mini dot timeline in the Time block.
// Kept small (8) so the whole "MM:SS ▰… MM:SS · ⚡ N×" line stays within the
// meterWidth-cell box inner width.
const timelineWidth = 8

// renderTimeline renders the Time block: an elapsed clock, a mini dot timeline
// filled to the pass progress, a projected total-pass clock, and a realtime
// speed badge. The whole line stays within the box inner width (~meterWidth).
func renderTimeline(file FileProgress) string {
	elapsed := file.ElapsedTime
	elapsedSecs := elapsed.Seconds()

	// Projected total pass time = elapsed / progress (consistent with the prior
	// ETA derivation). Show placeholder until progress is meaningful.
	rightClock := "--:--"
	if file.Progress > 0 {
		rightClock = formatElapsed(time.Duration(elapsedSecs / file.Progress * float64(time.Second)))
	}

	// Mini dot timeline filled to progress. Filled dots muted, empty dots use the
	// meter empty-track colour, so the timeline reads as secondary to the main
	// gradient bar above.
	filled := filledCells(file.Progress, timelineWidth, 0)
	filledStyle := lipgloss.NewStyle().Foreground(cli.ColorMuted)
	emptyStyle := lipgloss.NewStyle().Foreground(cli.ColorFill)
	timeline := filledStyle.Render(strings.Repeat("▰", filled)) +
		emptyStyle.Render(strings.Repeat("▱", timelineWidth-filled))

	// Realtime speed badge: (speedFraction * duration) / elapsed. The fraction
	// un-scales Pass 1's capped bar progress to true decode throughput. Guards
	// reject start-up garbage and a missing duration.
	badge := "⚡ —×"
	if file.Duration > 0 && file.Progress > 0.02 && elapsedSecs > 0.3 {
		rt := (speedFraction(file.CurrentPass, file.Progress) * file.Duration) / elapsedSecs
		badge = fmt.Sprintf("⚡ %.1f×", rt)
	}

	muted := lipgloss.NewStyle().Foreground(cli.ColorMuted)
	return fmt.Sprintf("%s %s %s  %s  %s",
		formatElapsed(elapsed),
		timeline,
		rightClock,
		muted.Render("·"),
		muted.Render(badge))
}

// speedFraction returns the audio fraction the realtime-speed badge should use
// for a given pass. Pass 1 caps its bar progress at processor.BandPhaseProgressStart
// to reserve headroom for the band phase, so the raw progress under-reports decode
// throughput; un-scale it (clamped to 1.0 for the brief band span where decode is
// already done). Other passes report a true fraction already, so pass it through.
func speedFraction(pass processor.PassNumber, progress float64) float64 {
	if pass != processor.PassAnalysis {
		return progress
	}
	return min(1.0, progress/processor.BandPhaseProgressStart)
}

// superscriptValue converts a numeric peak value to Unicode superscript so the
// peak label collapses onto a single marker line. The "Level:" header already
// names the unit, so the marker shows only the value. The mapping is: '-' → '⁻'
// (U+207B), digits 0-9 → ⁰¹²³⁴⁵⁶⁷⁸⁹ (U+2070, U+00B9, U+00B2, U+00B3,
// U+2074-U+2079; 1/2/3 are the Latin-1 superscripts, the rest the Superscripts
// and Subscripts block), and '.' → '·' (U+00B7 middle dot).
func superscriptValue(value string) string {
	const supDigits = "⁰¹²³⁴⁵⁶⁷⁸⁹"
	digits := []rune(supDigits)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '-':
			b.WriteRune('⁻')
		case r == '.':
			b.WriteRune('·')
		case r >= '0' && r <= '9':
			b.WriteRune(digits[r-'0'])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// meterRamp builds the meterWidth-cell green→yellow→orange→red VU ramp once on
// first call and caches it. Every input is a compile-time constant (meterWidth,
// meterFloorDB, the -16 dB green-zone threshold, and the package-level palette
// colours resolved from the terminal background detected once at startup), so the
// ramp is identical every frame; building it per render wasted two Blend1D calls
// and a fresh slice on the 60fps path for every active file. Lazy so the first
// call happens after terminal detection completes; thread-safe because both TUIs
// render from goroutines.
//
// Real VU meters keep green dominant across the low range and compress the warm
// colours into the hot end, so the ramp is two piecewise Blend1D segments keyed
// to the -16 dB threshold: green→yellow fills the low zone, then yellow→orange→red
// is squeezed into the top ~16 dB.
var meterRamp = sync.OnceValue(func() []color.Color {
	width := meterWidth
	minDB := meterFloorDB
	maxDB := 0.0
	greenZone := int((((-16.0) - minDB) / (maxDB - minDB)) * float64(width))
	greenZone = max(0, min(greenZone, width))

	ramp := make([]color.Color, 0, width)
	ramp = append(ramp, lipgloss.Blend1D(greenZone, cli.ColorGreen, cli.ColorYellow)...)
	ramp = append(ramp, lipgloss.Blend1D(width-greenZone, cli.ColorYellow, cli.ColorOrange, cli.ColorRed)...)
	return ramp
})

// meterRampStyles caches one lipgloss.Style per meterRamp colour so the meter
// flush indexes a pre-built style instead of allocating lipgloss.NewStyle() per
// colour run on the 60fps redraw. It derives its colours from meterRamp() inside
// the same closure, so the ramp is the single source of truth (the styles and the
// ramp can never drift). Lazy and thread-safe for the same reasons as meterRamp.
var meterRampStyles = sync.OnceValue(func() []lipgloss.Style {
	ramp := meterRamp()
	styles := make([]lipgloss.Style, len(ramp))
	for i, c := range ramp {
		styles[i] = lipgloss.NewStyle().Foreground(c)
	}
	return styles
})

// meterOffRampStyle is the off-ramp fallback style: cells whose index falls
// outside the cached ramp resolve to cli.ColorRed, matching cellColor in
// renderMeterBar. Built once at package init so the flush never allocates.
var meterOffRampStyle = lipgloss.NewStyle().Foreground(cli.ColorRed)

// renderAudioLevelMeterWithLevels renders a live audio level meter with dB
// visualisation. elapsed drives the gentle pulse of the peak-hold marker; it is
// the file's running elapsed time, advanced once per meter tick, so no second
// tick loop is needed.
func renderAudioLevelMeterWithLevels(displayLevel, currentLevel, peakLevel float64, elapsed time.Duration) string {
	var b strings.Builder

	// Display current level only; the peak value is tethered to its marker below.
	fmt.Fprintf(&b, "Level: %.1f ㏈\n", displayLevel)

	// Create visual meter
	// dB range: -70 dB (silence) to 0 dB (maximum)
	// Map to meterWidth-character width meter
	width := meterWidth
	minDB := meterFloorDB
	maxDB := 0.0

	// Calculate fill position for current level
	currentPos := max(0, min(int(((currentLevel-minDB)/(maxDB-minDB))*float64(width)), width))

	// Calculate column for peak marker. Unlike currentPos (an exclusive fill
	// count, so 0..width), this is a 0-based column index, so it must clamp to
	// width-1: at peakLevel == maxDB the raw ratio is 1.0 and would otherwise
	// place the marker and elbow one cell beyond the bar.
	peakPos := max(0, min(int(((peakLevel-minDB)/(maxDB-minDB))*float64(width)), width-1))

	b.WriteString(renderMeterBar(width, currentPos))

	if marker := renderPeakMarker(peakLevel, peakPos, width, minDB, elapsed); marker != "" {
		b.WriteByte('\n')
		b.WriteString(marker)
	}

	return b.String()
}

// renderMeterBar draws the width-cell level meter: the first currentPos cells use
// the filled glyph, the rest the empty glyph. It does NOT reuse renderFilledBar:
// the ramp colour spans both filled and empty cells (so the bar is coloured along
// its whole length), and contiguous same-colour cells are coalesced into one
// styled run so lipgloss emits a single colour sequence per run rather than per
// rune, keeping it off the 60fps path.
//
// The green→yellow→orange→red colour ramp is built once and cached (see
// meterRamp): its inputs (meterWidth, meterFloorDB, the -16 dB threshold, and the
// package-level colours) are all compile-time constants, so the ramp never varies
// per frame. The matching per-colour styles are cached too (meterRampStyles), so
// the flush indexes a pre-built style rather than allocating one per run.
func renderMeterBar(width, currentPos int) string {
	var b strings.Builder

	ramp := meterRamp()
	rampStyles := meterRampStyles()

	cellColor := func(i int) color.Color {
		if i < 0 || i >= len(ramp) {
			return cli.ColorRed
		}
		return ramp[i]
	}

	meterChar := func(i int) rune {
		if i < currentPos {
			return '▓' // Filled
		}
		return '░' // Empty
	}

	// Build contiguous same-colour runs and style each as one segment. The run is
	// coalesced by colour equality over the cell index, and cellColor maps cell i
	// to ramp position i, so the run's START cell index is its ramp position. Index
	// the cached style by that position; an off-ramp run (position outside the
	// ramp, the cli.ColorRed fallback) uses the package red style.
	var run strings.Builder
	var runColor color.Color
	runStart := 0
	flush := func() {
		if run.Len() == 0 {
			return
		}
		style := meterOffRampStyle
		if runStart >= 0 && runStart < len(rampStyles) {
			style = rampStyles[runStart]
		}
		b.WriteString(style.Render(run.String()))
		run.Reset()
	}

	for i := range width {
		color := cellColor(i)
		if run.Len() > 0 && color != runColor {
			flush()
			runStart = i
		}
		runColor = color
		run.WriteRune(meterChar(i))
	}
	flush()

	return b.String()
}

// renderPeakMarker lays out the peak-hold marker: a single pulsing line beneath
// the bar that tethers the peak value to its column via an up-tip arrow, with the
// value in Unicode superscript so the label and its pointer share one row. The
// "Level:" header already names the unit, so the marker carries only the value. It
// returns "" when there is no meaningful peak yet (peak still at the silence
// floor), so no stray marker sits at column 0. Alignment uses lipgloss.Width
// (display columns), not byte length: every superscript rune is width 1, including
// the '·' decimal separator pinned to width 1 at startup (see main.go), so the
// arrow lands exactly under the peak column.
func renderPeakMarker(peakLevel float64, peakPos, width int, minDB float64, elapsed time.Duration) string {
	if peakLevel <= minDB {
		return ""
	}

	var b strings.Builder

	pulseColor := peakMarkerColor(elapsed)
	arrowStyle := lipgloss.NewStyle().Foreground(pulseColor)
	valueStyle := lipgloss.NewStyle().Foreground(cli.ColorOrange)
	supValue := superscriptValue(fmt.Sprintf("%.1f", peakLevel))

	// Default: arrow leads, value to its right (⬑ value). When that form would
	// overflow the bar, flip so the value leads and the arrow trails (value ⬏),
	// keeping the label within meterWidth. The right form renders as
	// `<peakPos spaces>⬑ <supValue>` = peakPos + 1 (⬑) + 1 (space) +
	// lipgloss.Width(supValue) columns; the arrow sits at the peak column.
	if peakPos+lipgloss.Width(supValue)+2 <= width {
		b.WriteString(strings.Repeat(" ", peakPos))
		b.WriteString(arrowStyle.Render("⬑"))
		b.WriteByte(' ')
		b.WriteString(valueStyle.Render(supValue))
	} else {
		// value then a right-up arrow ⬏ ending under the peak column. The form
		// is `<lead spaces><supValue> ⬏`, so lead + width(supValue) + 1 ==
		// peakPos places the arrow at the peak column.
		lead := max(peakPos-(lipgloss.Width(supValue)+1), 0)
		b.WriteString(strings.Repeat(" ", lead))
		b.WriteString(valueStyle.Render(supValue))
		b.WriteByte(' ')
		b.WriteString(arrowStyle.Render("⬏"))
	}

	return b.String()
}

// peakMarkerDim and peakMarkerBright are the peak-pulse endpoints resolved to
// 8-bit sRGB channels once at package init, so peakMarkerColor interpolates raw
// integers each frame instead of resolving the palette colours per call.
var (
	peakMarkerDimR, peakMarkerDimG, peakMarkerDimB          = rgb8(cli.ColorOrangeDim)
	peakMarkerBrightR, peakMarkerBrightG, peakMarkerBrightB = rgb8(cli.ColorOrange)
)

// peakMarkerColor returns the peak-hold marker colour for the current pulse
// phase. It oscillates gently between a deep orange and the full orange at about
// 1.2 Hz, driven by elapsed wall-clock time so it reuses the existing meter tick
// cadence. The interpolation runs straight in sRGB between two oranges so the
// marker stays a clear orange shade at both ends and never drifts off-hue. The
// channel maths reproduces the former hex-string round-trip exactly: each channel
// is `uint8(dim + phase*(bright-dim) + 0.5)`, the same value the old
// `fmt.Sprintf("%02X", ...)` path formatted, then returned as a color.RGBA struct
// (satisfies color.Color) rather than re-parsed from a hex string.
func peakMarkerColor(elapsed time.Duration) color.Color {
	const pulseHz = 1.2
	// 0.0 at the dim trough, 1.0 at the bright crest.
	phase := 0.5 * (1 + math.Sin(2*math.Pi*pulseHz*elapsed.Seconds()))

	lerp := func(a, b uint8) uint8 {
		return uint8(float64(a) + phase*(float64(b)-float64(a)) + 0.5)
	}
	return color.RGBA{
		R: lerp(peakMarkerDimR, peakMarkerBrightR),
		G: lerp(peakMarkerDimG, peakMarkerBrightG),
		B: lerp(peakMarkerDimB, peakMarkerBrightB),
		A: 0xFF,
	}
}

// gainBarWidth is the cell count of the horizontal gain bar, matching the
// separation bar's grammar (▰ filled / ▱ empty) but wider so the gradient reads
// across width.
const gainBarWidth = 5

// GainBar renders the input true peak as a short horizontal bar that fills and
// colours with the peak, mirroring separationBar's grammar. It is pure
// presentation, derived from the same true peak as GainAdvice, with the fill and
// colour aligned to the advice zones so the bar matches the advice category:
//   - Quiet  (TP < -12)        -> ~1 filled, blue end
//   - Fine   (-12 <= TP <= -1) -> ~3 filled, green centre (the -6 target ≈0.5)
//   - Hot    (TP > -1)         -> ~5 filled, red end
//
// Fill fraction is gainGlyphPosition(inputTP); filled cells = round(position *
// width). Colour is a fixed five-stop ramp, one colour per cell
// (bright-cyan→blue→green→amber→red), so the fill edge sits at the zone colour.
// Empty cells render dim. Styling goes through lipgloss so it auto-strips on a
// non-TTY pipe, leaving the bare ▰▱ runes (which still convey fill in mono).
// Exported so the analysis-only console path (cmd/jive-vocals) reuses one source
// of truth.
func GainBar(inputTP float64) string {
	position := gainGlyphPosition(inputTP)
	// Floor at one cell so the cold/blue end always shows a pip, an empty bar
	// would lose the under-recorded signal. A clipping input (>= 0 dBTP) maxes
	// the bar so the worst case reads as a full, red-tipped run.
	filled := filledCells(position, gainBarWidth, 1)
	if inputTP >= 0 {
		filled = gainBarWidth
	}

	// Five fixed stops, one per cell (width == stop count, so Blend1D samples land
	// exactly on the stops with no muddy interpolation): the fill tip reads its
	// zone colour directly: 3 cells = green (spot on), 4 = amber (hot), 5 = red
	// (clipping).
	ramp := lipgloss.Blend1D(gainBarWidth, cli.ColorCyanBright, cli.ColorBlue, cli.ColorGreen, cli.ColorOrange, cli.ColorRed)
	return renderFilledBar(gainBarWidth, filled, ramp)
}

// filledCells converts a fill fraction into a filled-cell count for a
// width-cell bar: frac is clamped to [0,1], rounded to the nearest cell, and the
// count clamped to [minFilled, width]. minFilled lets a bar pin a minimum
// visible fill (GainBar's one-pip floor); the plain bars pass 0.
func filledCells(frac float64, width, minFilled int) int {
	frac = max(0, min(frac, 1))
	return max(minFilled, min(int(math.Round(frac*float64(width))), width))
}

// renderFilledBar draws a width-cell coloured bar: the first filled cells use the
// solid glyph styled by ramp[i], the rest use the empty glyph in cli.ColorFill.
// Callers own the ramp construction and the filled-count rounding, which differ.
func renderFilledBar(width, filled int, ramp []color.Color) string {
	var b strings.Builder
	for i := range width {
		var c color.Color = cli.ColorFill
		ch := "▱"
		if i < filled {
			c = ramp[i]
			ch = "▰"
		}
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(ch))
	}
	return b.String()
}

// gainGlyphPosition maps an input true peak (dBTP) to a position in [0,1] using
// the GainAdvice zone boundaries, so the bar's fill and colour band coincide with
// the advice category. Each band clamps at its edges.
func gainGlyphPosition(inputTP float64) float64 {
	const (
		quietTP  = -12.0
		hotTP    = -1.0
		quietLo  = -24.0
		hotHi    = 1.0
		quietPos = 0.33
		finePos  = 0.67
	)
	switch {
	case inputTP < quietTP:
		return lerpClamp(inputTP, quietLo, quietTP, 0.0, quietPos)
	case inputTP <= hotTP:
		return lerpClamp(inputTP, quietTP, hotTP, quietPos, finePos)
	default:
		return lerpClamp(inputTP, hotTP, hotHi, finePos, 1.0)
	}
}

// lerpClamp maps v from the input range [inLo,inHi] onto [outLo,outHi], clamping
// the result to the output range at the edges.
func lerpClamp(v, inLo, inHi, outLo, outHi float64) float64 {
	if inHi == inLo {
		return outLo
	}
	t := (v - inLo) / (inHi - inLo)
	t = max(0, min(1, t))
	return outLo + t*(outHi-outLo)
}

// rgb8 resolves a color.Color to 8-bit sRGB channels.
func rgb8(c color.Color) (r, g, b uint8) {
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return rgba.R, rgba.G, rgba.B
}

// renderOverallProgress renders the overall progress footer
func renderOverallProgress(m Model) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.ColorMuted).
		Padding(0, 1)

	content := fmt.Sprintf("Processing %d files, %d complete, %d failed",
		m.TotalFiles, m.CompletedFiles, m.FailedFiles)

	return box.Render(content)
}

// FinalSummary returns the completion-summary string for persisting to the
// normal screen after the alt-screen program exits. Callers gate on Model.Done
// so an early user quit does not print a misleading "complete" summary.
func FinalSummary(m Model) string {
	return renderCompletionSummary(m)
}
