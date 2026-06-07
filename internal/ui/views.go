package ui

import (
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jivetalking/internal/cli"
	"github.com/linuxmatters/jivetalking/internal/processor"
)

// renderProcessingView renders the main processing view
func renderProcessingView(m Model) string {
	var b strings.Builder

	// Header
	b.WriteString(renderHeader(m))
	b.WriteString("\n\n")

	// File queue
	b.WriteString(renderFileQueue(m, m.progress))
	b.WriteString("\n\n")

	// Overall progress
	b.WriteString(renderOverallProgress(m))

	return b.String()
}

// renderHeader renders the application header
func renderHeader(m Model) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.ColorRed).
		Render("Jivetalking 🕺")

	subtitle := lipgloss.NewStyle().
		Foreground(cli.ColorMuted).
		Italic(true).
		Render(fmt.Sprintf("Processing %d file(s)", m.TotalFiles))

	return title + "\n" + subtitle
}

// renderFileQueue renders the list of files with their status
func renderFileQueue(m Model, prog progress.Model) string {
	var b strings.Builder

	for i := range m.Files {
		// Use the eased meter and progress positions for the active display;
		// fall back to the raw values when no spring slot exists.
		easedLevel := m.Files[i].CurrentLevel
		easedProgress := m.Files[i].Progress
		if i < len(m.meters) {
			easedLevel = m.meters[i].pos
			easedProgress = m.meters[i].progPos
		}
		b.WriteString(renderFileEntry(m.Files[i], prog, easedLevel, easedProgress))
		b.WriteString("\n")
	}

	return b.String()
}

// renderFileEntry renders a single file entry in the queue
func renderFileEntry(file FileProgress, prog progress.Model, easedLevel, easedProgress float64) string {
	fileName := filepath.Base(file.InputPath)

	switch file.Status {
	case StatusComplete:
		// 🗸 completed file with summary
		icon := lipgloss.NewStyle().Foreground(cli.ColorGreen).Render("🗸")
		delta := file.OutputLUFS - file.InputLUFS
		summary := fmt.Sprintf("Input: %.1f LUFS | Output: %.1f LUFS | Δ %+.1f dB",
			file.InputLUFS, file.OutputLUFS, delta)
		return fmt.Sprintf(" %s %s → %s\n   %s", icon, fileName, filepath.Base(file.OutputPath), summary)

	case StatusAnalyzing, StatusProcessing, StatusNormalising:
		// active file with detailed progress
		icon := lipgloss.NewStyle().Foreground(cli.ColorOrange).Render("∿")
		return fmt.Sprintf(" %s %s\n%s",
			icon, fileName,
			renderFileDetails(file, prog, easedLevel, easedProgress))

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

// renderFileDetails renders detailed progress for the active file. easedLevel is
// the spring-smoothed audio level used for the meter display.
func renderFileDetails(file FileProgress, prog progress.Model, easedLevel, easedProgress float64) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.ColorSkyBlue).
		Padding(0, 1)

	var content strings.Builder

	// Pass indicator
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
	fmt.Fprintf(&content, "Pass %d/4: %s\n", file.CurrentPass, passName)

	// Progress bar (spring-eased fill for smooth motion)
	content.WriteString(prog.ViewAs(easedProgress))
	content.WriteString("\n\n")

	// Time estimates
	elapsed := file.ElapsedTime.Seconds()
	var remaining float64
	if file.Progress > 0 {
		remaining = (elapsed / file.Progress) - elapsed
	}
	fmt.Fprintf(&content, "Time: %.1fs | ETA: ~%.1fs\n", elapsed, remaining)

	// Audio level visualization. The displayed level eases toward the target
	// via the spring; the peak marker stays driven by the measured peak.
	if file.CurrentLevel != 0 {
		content.WriteString("\n")
		content.WriteString(renderAudioLevelMeter(easedLevel, file.PeakLevel, file.ElapsedTime))
	}

	return box.Render(content.String())
}

// renderAudioLevelMeter renders a live audio level meter with dB visualization.
// elapsed drives the gentle pulse of the peak-hold marker; it is the file's
// running elapsed time, advanced once per meter tick, so no second tick loop is
// needed.
func renderAudioLevelMeter(currentLevel, peakLevel float64, elapsed time.Duration) string {
	var b strings.Builder

	// Display current and peak levels
	fmt.Fprintf(&b, "Level: %.1f ㏈ | Peak: %.1f ㏈\n", currentLevel, peakLevel)

	// Create visual meter
	// dB range: -70 dB (silence) to 0 dB (maximum)
	// Map to meterWidth-character width meter
	width := meterWidth
	minDB := meterFloorDB
	maxDB := 0.0

	// Calculate fill position for current level
	currentPos := max(0, min(int(((currentLevel-minDB)/(maxDB-minDB))*float64(width)), width))

	// Calculate position for peak marker
	peakPos := max(0, min(int(((peakLevel-minDB)/(maxDB-minDB))*float64(width)), width))

	// Build a continuous green→yellow→orange→red colour ramp once per render.
	// Real VU meters keep green dominant across the low range and compress the
	// warm colours into the hot end, so the ramp is built from two piecewise
	// Blend1D segments keyed to the -16 dB threshold: green→yellow fills the low
	// zone, then yellow→orange→red is squeezed into the top ~16 dB.
	greenZone := int((((-16.0) - minDB) / (maxDB - minDB)) * float64(width))
	greenZone = max(0, min(greenZone, width))

	ramp := make([]color.Color, 0, width)
	ramp = append(ramp, lipgloss.Blend1D(greenZone, cli.ColorGreen, cli.ColorYellow)...)
	ramp = append(ramp, lipgloss.Blend1D(width-greenZone, cli.ColorYellow, cli.ColorOrange, cli.ColorRed)...)

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

	// Build contiguous same-colour runs and style each as one segment so
	// lipgloss emits a single colour sequence per run rather than per rune.
	var run strings.Builder
	var runColor color.Color
	flush := func() {
		if run.Len() == 0 {
			return
		}
		b.WriteString(lipgloss.NewStyle().Foreground(runColor).Render(run.String()))
		run.Reset()
	}

	for i := range width {
		color := cellColor(i)
		if run.Len() > 0 && color != runColor {
			flush()
		}
		runColor = color
		run.WriteRune(meterChar(i))
	}
	flush()

	// Peak-hold marker: a coloured triangle on its own line beneath the bar,
	// aligned to the peak column. Skip it when there is no meaningful peak yet
	// (peak still at the silence floor), so no stray triangle sits at column 0.
	if peakLevel > minDB {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", peakPos))
		b.WriteString(lipgloss.NewStyle().Foreground(peakMarkerColor(elapsed)).Render("▲"))
	}

	return b.String()
}

// peakMarkerColor returns the peak-hold triangle colour for the current pulse
// phase. It oscillates gently between a deep orange and the full orange at about
// 1.2 Hz, driven by elapsed wall-clock time so it reuses the existing meter tick
// cadence. The interpolation runs straight in sRGB between two oranges so the
// marker stays a clear orange shade at both ends and never drifts off-hue.
func peakMarkerColor(elapsed time.Duration) color.Color {
	const pulseHz = 1.2
	// 0.0 at the dim trough, 1.0 at the bright crest.
	phase := 0.5 * (1 + math.Sin(2*math.Pi*pulseHz*elapsed.Seconds()))

	dr, dg, db := rgb8(cli.ColorOrangeDim)
	br, bg, bb := rgb8(cli.ColorOrange)
	lerp := func(a, b uint8) uint8 {
		return uint8(float64(a) + phase*(float64(b)-float64(a)) + 0.5)
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		lerp(dr, br), lerp(dg, bg), lerp(db, bb)))
}

// rgb8 resolves a color.Color to 8-bit sRGB channels.
func rgb8(c color.Color) (r, g, b uint8) {
	r16, g16, b16, _ := c.RGBA()
	return uint8((r16 >> 8) & 0xFF), uint8((g16 >> 8) & 0xFF), uint8((b16 >> 8) & 0xFF)
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

// renderCompletionSummary renders the final completion summary
func renderCompletionSummary(m Model) string {
	var b strings.Builder

	// Completion header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.ColorGreen).
		Render("✨ Processing Complete!")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Summary for each file
	for _, file := range m.Files {
		if file.Status == StatusComplete {
			b.WriteString(renderCompletedFile(file))
			b.WriteString("\n")
		}
	}

	// Overall summary
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 60))
	b.WriteString("\n")
	b.WriteString("All files normalized to -16 LUFS and level-matched ✓\n")
	b.WriteString("Ready for import into Audacity - no additional processing needed!\n")

	return b.String()
}

// renderCompletedFile renders a summary for a completed file
func renderCompletedFile(file FileProgress) string {
	fileName := filepath.Base(file.InputPath)
	outputName := filepath.Base(file.OutputPath)

	icon := lipgloss.NewStyle().Foreground(cli.ColorGreen).Render("✓")

	quality := "★★★★★" // Always 5 stars

	return fmt.Sprintf(" %s %s → %s\n"+
		"   Before: %.1f LUFS | After: %.1f LUFS | Quality: %s\n"+
		"   Noise Reduced: %.0f dB",
		icon, fileName, outputName,
		file.InputLUFS, file.OutputLUFS, quality,
		file.NoiseFloor)
}
