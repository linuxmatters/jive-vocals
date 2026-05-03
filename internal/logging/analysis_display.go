// Package logging handles generation of analysis reports for processed audio files.
// This file provides console display for analysis-only mode.

package logging

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/linuxmatters/jivetalking/internal/audio"
	"github.com/linuxmatters/jivetalking/internal/processor"
)

// AnalysisTimings contains reportable analysis-only stage durations.
type AnalysisTimings struct {
	Analysis     time.Duration
	Adaptation   time.Duration
	ReportOutput time.Duration
}

type analysisMetricSpec struct {
	Label string
	Value string
}

// DisplayAnalysisResults outputs Pass 1 analysis results to the console.
// Used by --analysis-only mode for rapid inspection without full processing.
func DisplayAnalysisResults(w io.Writer, inputPath string, metadata *audio.Metadata, measurements *processor.AudioMeasurements, config *processor.FilterChainConfig, timings ...AnalysisTimings) {
	if measurements == nil {
		fmt.Fprintf(w, "No analysis data available for %s\n", filepath.Base(inputPath))
		return
	}
	reportOutputStart := time.Now()

	writeAnalysisHeader(w, inputPath, metadata)
	writeAnalysisLoudnessAndDynamics(w, measurements)
	writeAnalysisSilenceDetection(w, measurements)
	writeAnalysisSpeechDetection(w, measurements)
	writeAnalysisDerivedMeasurements(w, measurements)
	writeAnalysisFilterAdaptation(w, measurements, config)
	writeAnalysisSpectralSummary(w, measurements)
	writeAnalysisTips(w, measurements, config)

	if len(timings) > 0 && hasAnalysisTimings(timings[0]) {
		fmt.Fprintln(w)
		writeAnalysisTimingSection(w, completeAnalysisTimings(timings[0], reportOutputStart))
	}
}

func writeAnalysisHeader(w io.Writer, inputPath string, metadata *audio.Metadata) {
	fmt.Fprintln(w, strings.Repeat("=", 70))
	fmt.Fprintf(w, "ANALYSIS: %s\n", filepath.Base(inputPath))
	fmt.Fprintln(w, strings.Repeat("=", 70))
	fmt.Fprintf(w, "Duration:    %s\n", formatDuration(time.Duration(metadata.Duration*float64(time.Second))))
	fmt.Fprintf(w, "Sample Rate: %d Hz\n", metadata.SampleRate)
	fmt.Fprintf(w, "Channels:    %s\n", channelName(metadata.Channels))
	fmt.Fprintln(w)
}

func writeAnalysisLoudnessAndDynamics(w io.Writer, measurements *processor.AudioMeasurements) {
	writeAnalysisSection(w, "LOUDNESS")
	writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
		{"Integrated", fmt.Sprintf("%.1f LUFS", measurements.InputI)},
		{"True Peak", fmt.Sprintf("%.1f dBTP", measurements.InputTP)},
		{"Loudness Range", fmt.Sprintf("%.1f LU", measurements.InputLRA)},
	})
	fmt.Fprintln(w)

	writeAnalysisSection(w, "DYNAMICS")
	crestFactor := measurements.PeakLevel - measurements.RMSLevel
	crestSource := "full-file"
	if measurements.SpeechProfile != nil && measurements.SpeechProfile.CrestFactor > 0 {
		crestFactor = measurements.SpeechProfile.CrestFactor
		crestSource = "speech"
	}
	writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
		{"RMS Level", fmt.Sprintf("%.1f dBFS", measurements.RMSLevel)},
		{"Peak Level", fmt.Sprintf("%.1f dBFS", measurements.PeakLevel)},
		{"Dynamic Range", fmt.Sprintf("%.1f dB", measurements.DynamicRange)},
		{"Crest Factor", fmt.Sprintf("%.1f dB (%s)", crestFactor, crestSource)},
	})
	fmt.Fprintln(w)
}

func writeAnalysisSilenceDetection(w io.Writer, measurements *processor.AudioMeasurements) {
	writeAnalysisSection(w, "SILENCE DETECTION")
	fmt.Fprintf(w, "  Threshold:      %.1f dB (%.1f dBFS room tone estimate + 1 dB)\n",
		measurements.SilenceDetectLevel, measurements.PreScanNoiseFloor)

	writeAnalysisSilenceCandidates(w, measurements)
	fmt.Fprintln(w)
}

func writeAnalysisSpeechDetection(w io.Writer, measurements *processor.AudioMeasurements) {
	writeAnalysisSection(w, "SPEECH DETECTION")
	writeAnalysisSpeechCandidates(w, measurements)
	fmt.Fprintln(w)
}

func writeAnalysisDerivedMeasurements(w io.Writer, measurements *processor.AudioMeasurements) {
	writeAnalysisSection(w, "DERIVED MEASUREMENTS")
	if measurements.NoiseProfile != nil {
		suggestedGateDB := processor.LinearToDb(measurements.SuggestedGateThreshold)
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"Noise Floor", fmt.Sprintf("%.1f dBFS (from elected silence)", measurements.NoiseProfile.MeasuredNoiseFloor)},
			{"Gate Baseline", fmt.Sprintf("%.1f dB (noise floor + margin)", suggestedGateDB)},
			{"NR Headroom", fmt.Sprintf("%.1f dB (noise-to-speech gap)", measurements.NoiseReductionHeadroom)},
		})
	} else {
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"Noise Floor", fmt.Sprintf("%.1f dBFS (%s)", measurements.NoiseFloor, noiseFloorSourceLabel(measurements.NoiseFloorSource))},
		})
	}
	fmt.Fprintln(w)
}

func writeAnalysisFilterAdaptation(w io.Writer, measurements *processor.AudioMeasurements, config *processor.FilterChainConfig) {
	writeAnalysisSection(w, "FILTER ADAPTATION")
	if config != nil {
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"Highpass", fmt.Sprintf("%.0f Hz (from spectral analysis)", config.DS201HPFreq)},
		})
		if measurements.NoiseProfile != nil {
			gateThresholdDB := processor.LinearToDb(config.DS201GateThreshold)
			gateDesc := "(from noise floor)"
			if measurements.SpeechProfile != nil {
				gateDesc = "(with breath reduction)"
			}
			writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
				{"Gate Threshold", fmt.Sprintf("%.1f dB %s", gateThresholdDB, gateDesc)},
				{"Gate Ratio", fmt.Sprintf("%.1f:1", config.DS201GateRatio)},
			})
		}
		if config.NoiseRemoveCompandEnabled {
			writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
				{"NR Threshold", fmt.Sprintf("%.0f dB", config.NoiseRemoveCompandThreshold)},
				{"NR Expansion", fmt.Sprintf("%.0f dB", config.NoiseRemoveCompandExpansion)},
			})
		} else {
			writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
				{"NR Compander", "disabled"},
			})
		}
		if config.DeessIntensity > 0 {
			writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
				{"De-esser", fmt.Sprintf("%.0f%% intensity", config.DeessIntensity*100)},
			})
		} else {
			writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
				{"De-esser", "disabled (no sibilance detected)"},
			})
		}
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"LA-2A Thresh", fmt.Sprintf("%.0f dB", config.LA2AThreshold)},
			{"LA-2A Ratio", fmt.Sprintf("%.1f:1", config.LA2ARatio)},
		})
	}
}

func writeAnalysisSpectralSummary(w io.Writer, measurements *processor.AudioMeasurements) {
	writeAnalysisSection(w, "SPECTRAL SUMMARY")
	writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
		{"Centroid", fmt.Sprintf("%.0f Hz (%s)", measurements.SpectralCentroid, interpretCentroid(measurements.SpectralCentroid))},
		{"Spread", fmt.Sprintf("%.0f Hz (%s)", measurements.SpectralSpread, interpretSpread(measurements.SpectralSpread))},
		{"Rolloff", fmt.Sprintf("%.0f Hz (%s)", measurements.SpectralRolloff, interpretRolloff(measurements.SpectralRolloff))},
		{"Flatness", fmt.Sprintf("%.3f (%s)", measurements.SpectralFlatness, interpretFlatness(measurements.SpectralFlatness))},
		{"Kurtosis", fmt.Sprintf("%.1f (%s)", measurements.SpectralKurtosis, interpretKurtosis(measurements.SpectralKurtosis))},
		{"Skewness", fmt.Sprintf("%.2f (%s)", measurements.SpectralSkewness, interpretSkewness(measurements.SpectralSkewness))},
		{"Crest", fmt.Sprintf("%.1f (%s)", measurements.SpectralCrest, interpretCrest(measurements.SpectralCrest))},
		{"Slope", fmt.Sprintf("%.2e (%s)", measurements.SpectralSlope, interpretSlope(measurements.SpectralSlope))},
		{"Decrease", fmt.Sprintf("%.4f (%s)", measurements.SpectralDecrease, interpretDecrease(measurements.SpectralDecrease))},
		{"Entropy", fmt.Sprintf("%.3f (%s)", measurements.SpectralEntropy, interpretEntropy(measurements.SpectralEntropy))},
		{"Flux", fmt.Sprintf("%.4f (%s)", measurements.SpectralFlux, interpretFlux(measurements.SpectralFlux))},
	})
}

func writeAnalysisTips(w io.Writer, measurements *processor.AudioMeasurements, config *processor.FilterChainConfig) {
	tips := GenerateRecordingTips(measurements, config)
	fmt.Fprintln(w)
	writeAnalysisSection(w, "RECORDING TIPS")
	if len(tips) == 0 {
		fmt.Fprintln(w, "  ✓ Your recording setup looks good. No issues detected.")
	} else {
		for _, tip := range tips {
			wrapped := wrapText(tip.Message, 66, "    ")
			fmt.Fprintf(w, "  ⚠ %s\n", wrapped)
		}
	}
}

func hasAnalysisTimings(timings AnalysisTimings) bool {
	return timings.Analysis > 0 || timings.Adaptation > 0 || timings.ReportOutput > 0
}

func writeAnalysisMetricRows(w io.Writer, indent string, labelWidth int, rows []analysisMetricSpec) {
	for _, row := range rows {
		fmt.Fprintf(w, "%s%-*s %s\n", indent, labelWidth, row.Label+":", row.Value)
	}
}

func writeAnalysisTimingSection(w io.Writer, timings AnalysisTimings) {
	writeAnalysisSection(w, "ANALYSIS TIMINGS")
	writeAnalysisMetricRows(w, "  ", 14, []analysisMetricSpec{
		{"Analysis", formatDuration(timings.Analysis)},
		{"Adaptation", formatDuration(timings.Adaptation)},
		{"Report Output", formatDuration(timings.ReportOutput)},
	})
}

func completeAnalysisTimings(timings AnalysisTimings, reportOutputStart time.Time) AnalysisTimings {
	if timings.ReportOutput <= 0 {
		timings.ReportOutput = time.Since(reportOutputStart)
	}
	return timings
}

func writeAnalysisSilenceCandidates(w io.Writer, measurements *processor.AudioMeasurements) {
	if len(measurements.SilenceCandidates) == 0 {
		writeAnalysisSilenceFallback(w, measurements)
		return
	}

	electedCandidate, displayCandidates := rankedSilenceCandidateEntries(measurements)
	writeAnalysisCandidateFlow(
		w,
		len(measurements.SilenceCandidates),
		electedCandidate,
		displayCandidates,
		func() {
			if measurements.VoiceActivated {
				fmt.Fprintln(w, "  Voice-activated recording detected")
			}
		},
		func(w io.Writer, entry candidateDisplayEntry[processor.SilenceCandidateMetrics]) {
			writeAnalysisSilenceCandidateMetrics(w, entry, measurements)
		},
		func(w io.Writer, entry candidateDisplayEntry[processor.SilenceCandidateMetrics]) {
			writeCompactAnalysisSilenceCandidateRow(w, entry.index, entry.candidate)
		},
		func() {
			writeSilenceRejectionSummary(w, measurements.SilenceCandidates)
		},
	)
}

func writeAnalysisSilenceFallback(w io.Writer, measurements *processor.AudioMeasurements) {
	if measurements.NoiseProfile != nil {
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"Sample", fmt.Sprintf("%.1fs at %s", measurements.NoiseProfile.Duration.Seconds(), formatTimestamp(measurements.NoiseProfile.Start))},
			{"Noise Floor", fmt.Sprintf("%.1f dBFS", measurements.NoiseProfile.MeasuredNoiseFloor)},
		})
		return
	}

	fmt.Fprintln(w, "  No silence detected")
	if measurements.VoiceActivated {
		fmt.Fprintln(w, "  Voice-activated recording detected")
	}
}

func writeAnalysisSpeechCandidates(w io.Writer, measurements *processor.AudioMeasurements) {
	if len(measurements.SpeechCandidates) == 0 {
		writeAnalysisSpeechFallback(w, measurements)
		return
	}

	electedCandidate, displayCandidates := rankedSpeechCandidateEntries(measurements)
	writeAnalysisCandidateFlow(
		w,
		len(measurements.SpeechCandidates),
		electedCandidate,
		displayCandidates,
		nil,
		writeAnalysisSpeechCandidateMetrics,
		func(w io.Writer, entry candidateDisplayEntry[processor.SpeechCandidateMetrics]) {
			writeCompactAnalysisSpeechCandidateRow(w, entry.index, entry.candidate)
		},
		func() {
			writeSpeechRejectionSummary(w, measurements.SpeechCandidates)
		},
	)
}

func writeAnalysisSpeechFallback(w io.Writer, measurements *processor.AudioMeasurements) {
	if measurements.SpeechProfile != nil {
		writeAnalysisMetricRows(w, "  ", 15, []analysisMetricSpec{
			{"Sample", fmt.Sprintf("%.1fs at %s", measurements.SpeechProfile.Region.Duration.Seconds(), formatTimestamp(measurements.SpeechProfile.Region.Start))},
			{"RMS Level", fmt.Sprintf("%.1f dBFS", measurements.SpeechProfile.RMSLevel)},
		})
		return
	}

	fmt.Fprintln(w, "  No speech profile available")
}

func writeAnalysisCandidateFlow[T any](
	w io.Writer,
	totalCandidates int,
	electedCandidate *candidateDisplayEntry[T],
	displayCandidates []candidateDisplayEntry[T],
	afterSummary func(),
	writeElected func(io.Writer, candidateDisplayEntry[T]),
	writeDisplayed func(io.Writer, candidateDisplayEntry[T]),
	writeRejected func(),
) {
	fmt.Fprintf(w, "  Candidates:     %d evaluated\n", totalCandidates)
	if summary := candidateDisplaySummary(totalCandidates, electedCandidate != nil, len(displayCandidates)); summary != "" {
		fmt.Fprintf(w, "  Displayed:      %s\n", summary)
	}
	if afterSummary != nil {
		afterSummary()
	}

	if electedCandidate != nil || len(displayCandidates) > 0 {
		fmt.Fprintln(w)
		if electedCandidate != nil {
			writeElected(w, *electedCandidate)
			fmt.Fprintln(w)
		}
		for _, entry := range displayCandidates {
			writeDisplayed(w, entry)
			fmt.Fprintln(w)
		}
	}

	writeRejected()
}

// writeSilenceCandidateMetrics writes the metric lines for a single silence candidate.
func writeSilenceCandidateMetrics(w io.Writer, c processor.SilenceCandidateMetrics) {
	writeAnalysisMetricRows(w, "      ", 12, []analysisMetricSpec{
		{"Score", fmt.Sprintf("%.3f", c.Score)},
		{"RMS Level", fmt.Sprintf("%.1f dBFS", c.RMSLevel)},
		{"Peak Level", fmt.Sprintf("%.1f dBFS", c.PeakLevel)},
		{"Crest", fmt.Sprintf("%.1f dB", c.CrestFactor)},
		{"Entropy", fmt.Sprintf("%.3f (%s)", c.Spectral.Entropy, interpretEntropy(c.Spectral.Entropy))},
		{"Flatness", fmt.Sprintf("%.3f (%s)", c.Spectral.Flatness, interpretFlatness(c.Spectral.Flatness))},
		{"Kurtosis", fmt.Sprintf("%.1f (%s)", c.Spectral.Kurtosis, interpretKurtosis(c.Spectral.Kurtosis))},
		{"Centroid", fmt.Sprintf("%.0f Hz", c.Spectral.Centroid)},
	})
}

func writeAnalysisSilenceCandidateMetrics(w io.Writer, entry candidateDisplayEntry[processor.SilenceCandidateMetrics], measurements *processor.AudioMeasurements) {
	c := entry.candidate
	fmt.Fprintf(w, "  #%d: %.1fs at %s (elected)\n",
		entry.index+1, c.Region.Duration.Seconds(), formatTimestamp(c.Region.Start))
	if measurements.NoiseProfile.WasRefined {
		fmt.Fprintf(w, "      Refined:     %.1fs at %s (golden sub-region)\n",
			measurements.NoiseProfile.Duration.Seconds(), formatTimestamp(measurements.NoiseProfile.Start))
	}
	writeSilenceCandidateMetrics(w, c)
}

func writeAnalysisSpeechCandidateMetrics(w io.Writer, entry candidateDisplayEntry[processor.SpeechCandidateMetrics]) {
	c := entry.candidate
	fmt.Fprintf(w, "  #%d: %.1fs at %s (elected)\n",
		entry.index+1, c.Region.Duration.Seconds(), formatTimestamp(c.Region.Start))

	if c.WasRefined {
		fmt.Fprintf(w, "      Refined:     %.1fs at %s -> %.1fs at %s (golden sub-region)\n",
			c.OriginalDuration.Seconds(),
			formatTimestamp(c.OriginalStart),
			c.Region.Duration.Seconds(),
			formatTimestamp(c.Region.Start))
	}

	rows := []analysisMetricSpec{
		{"Score", fmt.Sprintf("%.2f", c.Score)},
		{"RMS Level", fmt.Sprintf("%.1f dBFS", c.RMSLevel)},
		{"Crest", fmt.Sprintf("%.1f dB", c.CrestFactor)},
		{"Centroid", fmt.Sprintf("%.0f Hz (%s)", c.Spectral.Centroid, interpretCentroid(c.Spectral.Centroid))},
		{"Kurtosis", fmt.Sprintf("%.1f (%s)", c.Spectral.Kurtosis, interpretKurtosis(c.Spectral.Kurtosis))},
	}
	if c.VoicingDensity > 0 {
		rows = append(rows, analysisMetricSpec{"Voicing", fmt.Sprintf("%.0f%%", c.VoicingDensity*100)})
	}
	writeAnalysisMetricRows(w, "      ", 12, rows)
}

func writeCompactAnalysisSilenceCandidateRow(w io.Writer, index int, c processor.SilenceCandidateMetrics) {
	fmt.Fprintf(w, "  #%d: %.1fs at %s (score: %.3f)\n",
		index+1, c.Region.Duration.Seconds(), formatTimestamp(c.Region.Start), c.Score)
	fmt.Fprintf(w, "      RMS: %.1f dBFS, Crest: %.1f dB, Entropy: %.3f (%s)\n",
		c.RMSLevel, c.CrestFactor, c.Spectral.Entropy, interpretEntropy(c.Spectral.Entropy))
}

func writeCompactAnalysisSpeechCandidateRow(w io.Writer, index int, c processor.SpeechCandidateMetrics) {
	fmt.Fprintf(w, "  #%d: %.1fs at %s (score: %.2f)\n",
		index+1, c.Region.Duration.Seconds(), formatTimestamp(c.Region.Start), c.Score)
	fmt.Fprintf(w, "      RMS: %.1f dBFS, Crest: %.1f dB, Centroid: %.0f Hz (%s)\n",
		c.RMSLevel, c.CrestFactor, c.Spectral.Centroid, interpretCentroid(c.Spectral.Centroid))
}

// writeSilenceRejectionSummary outputs a compact summary of rejected silence candidates.
// Groups zero-scored candidates by rejection reason extracted from TransientWarning.
func writeSilenceRejectionSummary(w io.Writer, candidates []processor.SilenceCandidateMetrics) {
	writeAnalysisCandidateRejectionSummary(w, silenceRejectionSummary(candidates))
}

func writeSpeechRejectionSummary(w io.Writer, candidates []processor.SpeechCandidateMetrics) {
	writeAnalysisCandidateRejectionSummary(w, speechRejectionSummary(candidates))
}

func writeAnalysisCandidateRejectionSummary(w io.Writer, summary string) {
	fmt.Fprintf(w, "  Rejected:       %s\n", summary)
}

// classifyRejectionReason maps a TransientWarning string to a short label.
func classifyRejectionReason(warning string) string {
	switch {
	case strings.Contains(warning, "digital silence"):
		return "digital silence"
	case strings.Contains(warning, "crosstalk"):
		return "crosstalk"
	case strings.Contains(warning, "transient contamination"):
		return "transient contamination"
	case warning == "":
		return "too loud"
	default:
		return "too loud"
	}
}

// noiseFloorSourceLabel returns a human-readable label for the noise floor derivation source.
func noiseFloorSourceLabel(source string) string {
	switch source {
	case "astats":
		return "from astats"
	case "rms_estimate":
		return "estimated from RMS level"
	case "ebur128_estimate":
		return "estimated from loudness"
	case "silence_profile":
		return "from silence profile"
	default:
		return "derived"
	}
}

// writeAnalysisSection writes a section header for analysis output.
func writeAnalysisSection(w io.Writer, title string) {
	fmt.Fprintln(w, title)
}

// formatTimestamp formats a duration as a timestamp string (e.g., "1m 32s" or "24.0s").
func formatTimestamp(d time.Duration) string {
	totalSeconds := d.Seconds()
	if totalSeconds < 60 {
		return fmt.Sprintf("%.1fs", totalSeconds)
	}

	minutes := int(totalSeconds) / 60
	seconds := math.Mod(totalSeconds, 60)

	if minutes >= 60 {
		hours := minutes / 60
		minutes %= 60
		return fmt.Sprintf("%dh %dm %.0fs", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm %.0fs", minutes, seconds)
}
