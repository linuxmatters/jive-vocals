package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/linuxmatters/jive-vocals/internal/cli"
)

// renderCompletionSummary renders the final view that prints after alt-screen
// exits. Completed files use renderDoneBox, so the final view matches the live
// view at completion.
func renderCompletionSummary(m Model) string {
	var b strings.Builder

	b.WriteString(cli.RenderTitle())
	b.WriteString("\n\n")

	b.WriteString(renderOverallProgress(m))
	b.WriteString("\n\n")

	for i := range m.Files {
		if m.Files[i].Status == StatusError {
			b.WriteString(renderFileEntryWithCache(&m.Files[i], m.fileEntryCache(i), m.progress, 0, 0, 0, m.Width))
			b.WriteString("\n")
			continue
		}
		if m.Files[i].Status == StatusComplete {
			b.WriteString(renderFileEntryWithCache(&m.Files[i], m.fileEntryCache(i), m.progress, 0, 0, 0, m.Width))
			b.WriteString("\n")
		}
	}

	return b.String()
}

const doneBoxLabelWidth = 12

const (
	doneBoxValueWidth = 5
	doneBoxUnitWidth  = 4
	doneBoxDeltaWidth = 5
)

func doneBoxBeforeAfterRow(before, after float64, unit string, delta float64) string {
	beforeCol := fmt.Sprintf("%*.1f", doneBoxValueWidth, before)
	afterCol := fmt.Sprintf("%*.1f", doneBoxValueWidth, after)
	unitCol := fitWidth(unit, doneBoxUnitWidth)
	deltaCol := fmt.Sprintf("%+*.1f", doneBoxDeltaWidth, delta)
	return fmt.Sprintf("%s → %s %s  Δ %s", beforeCol, afterCol, unitCol, deltaCol)
}

func doneBoxOptionalBeforeAfter(unit string, before, after float64, haveBefore bool) string {
	if haveBefore {
		delta := after - before
		return doneBoxBeforeAfterRow(before, after, unit, delta)
	}
	return fmt.Sprintf("%.1f %s", after, unit)
}

const noiseFloorMinDB = -96.0

func formatNoiseFloorCell(floor float64) string {
	if math.IsInf(floor, -1) || floor <= noiseFloorMinDB {
		return fmt.Sprintf("%*s", doneBoxValueWidth, "< -96")
	}
	return fmt.Sprintf("%*.0f", doneBoxValueWidth, floor)
}

func doneBoxNoiseFloorRow(input, output float64, haveInput, haveOutput bool) string {
	unitCol := fitWidth("㏈", doneBoxUnitWidth)
	switch {
	case haveInput && haveOutput:
		return fmt.Sprintf("%s → %s %s",
			formatNoiseFloorCell(input), formatNoiseFloorCell(output), unitCol)
	case haveOutput:
		return fmt.Sprintf("%s ㏈", strings.TrimSpace(formatNoiseFloorCell(output)))
	case haveInput:
		return fmt.Sprintf("%s ㏈", strings.TrimSpace(formatNoiseFloorCell(input)))
	default:
		return "n/a"
	}
}

func renderDoneBox(file FileProgress) string {
	outputName := filepath.Base(file.OutputPath)

	icon := lipgloss.NewStyle().Foreground(cli.ColorGreen).Render("🗸")
	heading := fmt.Sprintf(" %s %s", icon, outputName)

	labelStyle := lipgloss.NewStyle().Foreground(cli.ColorMuted).Width(doneBoxLabelWidth)
	valueStyle := lipgloss.NewStyle().Foreground(cli.ColorText)
	starStyle := lipgloss.NewStyle().Foreground(cli.ColorOrange)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.ColorIndigo).
		Padding(0, 1).
		Width(meterWidth + 4)

	var content strings.Builder

	muted := lipgloss.NewStyle().Foreground(cli.ColorMuted)
	speedBadge := "⚡ \u2014×"
	if file.ProcessingTime > 0 && file.Duration > 0 {
		rt := file.Duration / file.ProcessingTime.Seconds()
		speedBadge = fmt.Sprintf("⚡ %.1f×", rt)
	}
	timeValue := fmt.Sprintf("%s  %s  %s",
		formatElapsed(file.ProcessingTime), muted.Render("·"), muted.Render(speedBadge))
	fmt.Fprintf(&content, "%s%s\n", labelStyle.Render("Time"), timeValue)

	loudnessDelta := file.OutputLUFS - file.InputLUFS
	loudnessValue := doneBoxBeforeAfterRow(file.InputLUFS, file.OutputLUFS, "LUFS", loudnessDelta)
	fmt.Fprintf(&content, "%s%s\n",
		labelStyle.Render("Loudness"), valueStyle.Render(loudnessValue))

	tpValue := doneBoxOptionalBeforeAfter(unitDBTP, file.Summary.TruePeakDBTP, file.OutputTP, file.Summary.ChainReady)
	fmt.Fprintf(&content, "%s%s\n",
		labelStyle.Render("True peak"), valueStyle.Render(tpValue))

	lraValue := doneBoxOptionalBeforeAfter("LU", file.Summary.InputLRA, file.OutputLRA, file.Summary.ChainReady)
	fmt.Fprintf(&content, "%s%s\n",
		labelStyle.Render("Dynamics"), valueStyle.Render(lraValue))

	noiseValue := doneBoxNoiseFloorRow(
		file.InputNoiseFloor, file.FinalNoiseFloor,
		file.HaveInputNoiseFloor, file.HaveFinalNoiseFloor)
	fmt.Fprintf(&content, "%s%s\n",
		labelStyle.Render("Noise floor"), valueStyle.Render(noiseValue))

	recStars := starStyle.Render(QualityStars(file.RecordingQuality.Stars))
	fmt.Fprintf(&content, "%s%s  %s\n",
		labelStyle.Render("Recording"), recStars, valueStyle.Render(file.RecordingQuality.Label))

	procStars := starStyle.Render(QualityStars(file.Quality.Stars))
	fmt.Fprintf(&content, "%s%s  %s",
		labelStyle.Render("Processed"), procStars, valueStyle.Render(file.Quality.Label))

	return heading + "\n" + box.Render(content.String())
}

// QualityStars renders an n-of-5 star bar as filled stars then empty stars.
// The analysis-only console path reuses it to avoid duplicate glyph rules.
func QualityStars(n int) string {
	n = max(0, min(5, n))
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}
