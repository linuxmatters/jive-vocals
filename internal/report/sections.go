package report

import (
	"strconv"
	"strings"
	"time"

	"github.com/linuxmatters/jive-vocals/internal/processor"
)

// This file holds the per-domain section renderers: Header, Processing Summary,
// Loudness, Dynamics, Spectral, Noise Floor, Regions, and Interval Summary. Each
// is a pure func(...) string reading ONLY the run record (and Timings for the
// summary) - no AudioMeasurements, no .json re-read, no internal/logging. The
// metric-table engine lives in metricrow.go; the filter/normalisation and
// spectrogram renderers live in sections_filters.go and
// sections_spectrograms.go.

// =============================================================================
// Header
// =============================================================================

// renderHeader renders the run provenance block: input file, jive-vocals
// version, resolved executable path, processed-at, audio duration, sample rate,
// and channel layout. Reads only rec.Run.
func renderHeader(rec *processor.RunRecord) string {
	var b strings.Builder
	b.WriteString("# Audio Processing Report\n\n")
	b.WriteString("## Run\n\n")

	rows := [][]string{
		{"Input file", rec.Run.InputFile},
		{"Version", stringCell(rec.Run.Version)},
		{"Executable", stringCell(rec.Run.Executable)},
		{"Processed at", rec.Run.ProcessedAt},
		{"Duration", formatDuration(durationFromSeconds(rec.Run.DurationS))},
		{"Sample rate", formatSampleRate(rec.Run.SampleRateHz)},
		{"Channels", channelName(rec.Run.Channels)},
	}
	b.WriteString(mdTable([]string{"Field", "Value"}, rows))
	return b.String()
}

// =============================================================================
// Processing Summary
// =============================================================================

// renderProcessingSummary renders the pass durations, adaptation time, and
// real-time factor. It reads ONLY timings; the record carries no run timing.
// Returns the empty string when timings is the zero value (analysis-only mode has
// no processing timings), so the orchestrator omits the section entirely.
func renderProcessingSummary(timings Timings) string {
	if timings == (Timings{}) {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Processing Summary\n\n")

	rows := make([][]string, 0, 7)
	addDurationRow := func(label string, d time.Duration) {
		if d > 0 {
			rows = append(rows, []string{label, formatDuration(d)})
		}
	}
	addDurationRow("Pass 1 (analysis)", timings.Pass1)
	addDurationRow("Pass 2 (filter chain)", timings.Pass2)
	addDurationRow("Pass 3 (loudnorm measure)", timings.Pass3)
	addDurationRow("Pass 4 (loudnorm apply)", timings.Pass4)
	addDurationRow("Analysis", timings.Analysis)
	addDurationRow("Adaptation", timings.Adaptation)
	if timings.RealTimeFactor > 0 {
		rows = append(rows, []string{"Real-time factor", formatFloat(timings.RealTimeFactor, 1) + "x"})
	}

	b.WriteString(mdTable([]string{"Stage", "Duration"}, rows))
	return b.String()
}

// =============================================================================
// Loudness
// =============================================================================

// renderLoudness renders the EBU R128 loudness table, one row per loudness
// metric, with Input/Filtered/Final value columns (absent stages omitted). The
// input stage is *InputLoudnessMetrics; filtered/final are *OutputLoudnessMetrics
// (different Go types, same JSON keys), so each row's getters read the value off
// whichever stage struct is present.
func renderLoudness(rec *processor.RunRecord) string {
	in := rec.Loudness.Stages.Input
	filt := rec.Loudness.Stages.Filtered
	final := rec.Loudness.Stages.Final

	rows := []metricRow{
		{
			metric: integratedLUFSMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.InputI }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputI }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputI }),
		},
		{
			metric: truePeakDBTPMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.InputTP }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputTP }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputTP }),
		},
		{
			metric: lraLUMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.InputLRA }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputLRA }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputLRA }),
		},
		{
			metric: threshLUFSMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.InputThresh }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputThresh }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.OutputThresh }),
		},
		{
			metric: momentaryLUFSMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.MomentaryLoudness }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.MomentaryLoudness }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.MomentaryLoudness }),
		},
		{
			metric: shortTermLUFSMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.ShortTermLoudness }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.ShortTermLoudness }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.ShortTermLoudness }),
		},
		{
			metric: samplePeakDBFSMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.SamplePeak }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.SamplePeak }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.SamplePeak }),
		},
		{
			metric: targetOffsetDBMetric,
			input:  stageGetter(in, func(m *processor.InputLoudnessMetrics) float64 { return m.TargetOffset }),
			filt:   stageGetter(filt, func(m *processor.OutputLoudnessMetrics) float64 { return m.TargetOffset }),
			final:  stageGetter(final, func(m *processor.OutputLoudnessMetrics) float64 { return m.TargetOffset }),
		},
	}

	var b strings.Builder
	b.WriteString("## Loudness\n\n")
	b.WriteString(renderMetricTable(rows))
	return b.String()
}

// =============================================================================
// Dynamics
// =============================================================================

// renderDynamics renders the astats time-domain table. All three stages share the
// *DynamicsMetrics Go type (input/filtered/final), so the getters read the same
// fields off whichever stage is present.
func renderDynamics(rec *processor.RunRecord) string {
	in := rec.Dynamics.Stages.Input
	filt := rec.Dynamics.Stages.Filtered
	final := rec.Dynamics.Stages.Final

	row := func(metric metricDescriptor, f func(*processor.DynamicsMetrics) float64) metricRow {
		return metricRow{
			metric: metric,
			input:  stageGetter(in, f), filt: stageGetter(filt, f), final: stageGetter(final, f),
		}
	}

	rows := []metricRow{
		row(rmsLevelDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.RMSLevel }),
		row(peakLevelDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.PeakLevel }),
		row(crestFactorAstatsDBMetric, func(m *processor.DynamicsMetrics) float64 { return m.CrestFactor }),
		row(dynamicRangeDBMetric, func(m *processor.DynamicsMetrics) float64 { return m.DynamicRange }),
		row(minLevelDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.MinLevel }),
		row(maxLevelDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.MaxLevel }),
		row(rmsPeakDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.RMSPeak }),
		row(rmsTroughDBFSMetric, func(m *processor.DynamicsMetrics) float64 { return m.RMSTrough }),
		row(flatFactorMetric, func(m *processor.DynamicsMetrics) float64 { return m.FlatFactor }),
		row(dcOffsetMetric, func(m *processor.DynamicsMetrics) float64 { return m.DCOffset }),
		row(zeroCrossingsRateMetric, func(m *processor.DynamicsMetrics) float64 { return m.ZeroCrossingsRate }),
		row(bitDepthMetric, func(m *processor.DynamicsMetrics) float64 { return m.BitDepth }),
		row(entropyMetric, func(m *processor.DynamicsMetrics) float64 { return m.Entropy }),
	}

	var b strings.Builder
	b.WriteString("## Dynamics\n\n")
	b.WriteString(renderMetricTable(rows))
	return b.String()
}

// =============================================================================
// Spectral
// =============================================================================

// renderSpectral renders the aspectralstats table (the 13 spectral metrics). All
// three stages share the *SpectralMetrics Go type.
func renderSpectral(rec *processor.RunRecord) string {
	in := rec.Spectral.Stages.Input
	filt := rec.Spectral.Stages.Filtered
	final := rec.Spectral.Stages.Final

	row := func(metric metricDescriptor) metricRow {
		return metricRow{
			metric: metric,
			input:  stageGetter(in, func(m *processor.SpectralMetrics) float64 { return spectralMetricValue(metric, m) }),
			filt:   stageGetter(filt, func(m *processor.SpectralMetrics) float64 { return spectralMetricValue(metric, m) }),
			final:  stageGetter(final, func(m *processor.SpectralMetrics) float64 { return spectralMetricValue(metric, m) }),
		}
	}

	rows := make([]metricRow, 0, len(spectralMetricDescriptors))
	for _, metric := range spectralMetricDescriptors {
		rows = append(rows, row(metric))
	}

	var b strings.Builder
	b.WriteString("## Spectral\n\n")
	b.WriteString(renderMetricTable(rows))
	return b.String()
}

func spectralMetricValue(metric metricDescriptor, m *processor.SpectralMetrics) float64 {
	switch metric.key {
	case keyMean:
		return m.Mean
	case keyVariance:
		return m.Variance
	case keyCentroidHz:
		return m.Centroid
	case keySpreadHz:
		return m.Spread
	case keySkewness:
		return m.Skewness
	case keyKurtosis:
		return m.Kurtosis
	case keyEntropy:
		return m.Entropy
	case keyFlatness:
		return m.Flatness
	case keyCrest:
		return m.Crest
	case keyFlux:
		return m.Flux
	case keySlope:
		return m.Slope
	case keyDecrease:
		return m.Decrease
	case keyRolloffHz:
		return m.Rolloff
	default:
		panic("report: unrouted spectral metric " + string(metric.key))
	}
}

// =============================================================================
// Noise Floor
// =============================================================================

// renderNoiseFloor renders the input-only noise domain block: the elected floor
// and its source, the two distinct floor estimates (prescan, astats), the
// adaptive room-tone detect level, the voice-activated flag, the floored-interval
// fraction behind it, and the reduction
// headroom. Reads only rec.Noise. Raw measured values only - no "Noise Reduction"
// delta, no "Floor-Speech SNR", no "Character", no per-row verdict.
// Returns the empty string when the record carries no noise block (defensive;
// analysis and processing records both populate it).
func renderNoiseFloor(rec *processor.RunRecord) string {
	n := rec.Noise
	if n == nil {
		return ""
	}

	rows := [][]string{
		metricValueRow(floorDBFSMetric, n.Floor),
		{metricLabel(floorSourceMetric), metricDefinition(floorSourceMetric), stringCell(n.FloorSource)},
		metricValueRow(floorPrescanDBFSMetric, n.FloorPrescan),
		metricValueRow(floorAstatsDBFSMetric, n.FloorAstats),
		metricValueRow(roomToneDetectLevelDBFSMetric, n.RoomToneDetectLevel),
		{metricLabel(voiceActivatedMetric), metricDefinition(voiceActivatedMetric), boolCell(n.VoiceActivated)},
		metricValueRow(flooredFractionMetric, n.FlooredFraction),
		metricValueRow(reductionHeadroomDBMetric, n.ReductionHeadroom),
	}

	return renderValueTable("## Noise Floor\n\n", rows)
}

// =============================================================================
// Regions (room-tone + speech)
// =============================================================================

// renderRegions renders the room-tone and speech region blocks. For each kind it
// emits (a) the elected profile metrics, (b) for speech, a candidate summary
// (evaluated count + elected score ONLY - the full ranked array lives in the
// .candidates.jsonl sidecar, never inline; room tone carries no candidate
// summary), and (c) the per-stage Input/Filtered/Final region samples (absent
// stages omitted exactly like the loudness/dynamics tables).
//
// Record field paths (the densest record area):
//   - rec.Regions.RoomTone.Elected.Profile()  -> *processor.NoiseProfile
//   - rec.Regions.RoomTone.Samples.{Input,Filtered,Final} -> *processor.RegionSample
//   - rec.Regions.Speech.Elected.Profile()     -> *processor.SpeechCandidateMetrics
//   - rec.Regions.Speech.CandidatesSummary     -> *processor.CandidatesSummary
//   - rec.Regions.Speech.Samples.{Input,Filtered,Final}   -> *processor.RegionSample
//
// Room-tone Samples.Input may be nil (the elected NoiseProfile has no embedded
// RegionSample; the input sample is wired only when the elected candidate's
// RegionSample was captured at election). A nil input renders the placeholder for
// every cell, matching the absent-stage convention. Reads only rec.Regions.
// Returns the empty string when the record carries no regions block.
func renderRegions(rec *processor.RunRecord) string {
	if rec.Regions == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Regions\n\n")

	b.WriteString("### Room Tone\n\n")
	b.WriteString(renderRoomToneElected(rec.Regions.RoomTone.ElectedProfile()))
	b.WriteString(renderRegionSamples(rec.Regions.RoomTone.Samples))

	b.WriteString("### Speech\n\n")
	b.WriteString(renderSpeechElected(rec.Regions.Speech.ElectedProfile()))
	b.WriteString(renderCandidatesSummary(rec.Regions.Speech.CandidatesSummary))
	b.WriteString(renderRegionSamples(rec.Regions.Speech.Samples))

	b.WriteString(renderGateStatistics(rec.Regions.GateStatistics))

	return b.String()
}

// renderGateStatistics renders the gate-window measurements derived from the one
// Pass 1 VAD split: the voiced-speech low percentile, the noise high percentile,
// and their separation. The two percentiles are on the VAD level axis (momentary
// LUFS); the separation is a dB difference. Returns the empty string when the
// record carries no gate statistics.
func renderGateStatistics(g *processor.GateStatistics) string {
	if g == nil {
		return ""
	}

	rows := [][]string{
		metricValueRow(voicedLowPercentileDBFSMetric, g.VoicedLowPercentile),
		metricValueRow(noiseHighPercentileDBFSMetric, g.NoiseHighPercentile),
		metricValueRow(gateSeparationDBMetric, g.SeparationDB),
	}

	return renderValueTable("### Gate Statistics\n\n", rows)
}

// renderRoomToneElected renders the elected room-tone NoiseProfile metrics as a
// Metric | Definition | Value table. Returns a short note when no profile was
// elected. Reads the wrapped *NoiseProfile via the record's Profile() read seam.
func renderRoomToneElected(p *processor.NoiseProfile) string {
	if p == nil {
		return "_No room-tone profile elected._\n\n"
	}

	rows := [][]string{
		metricValueRow(startSMetric, p.Start.Seconds()),
		metricValueRow(durationSMetric, p.Duration.Seconds()),
		metricValueRow(measuredFloorDBFSMetric, p.MeasuredNoiseFloor),
		metricValueRow(peakLevelDBFSMetric, p.PeakLevel),
		metricValueRow(crestFactorDBMetric, p.CrestFactor),
		metricValueRow(entropyMetric, p.Entropy),
		metricValueRow(spectralCentroidHzMetric, p.Spectral.Centroid),
		metricValueRow(spectralFlatnessMetric, p.Spectral.Flatness),
		metricValueRow(spectralKurtosisMetric, p.Spectral.Kurtosis),
	}

	return renderValueTable("**Elected profile**\n\n", rows)
}

// renderSpeechElected renders the elected speech-candidate metrics (region length,
// amplitude/loudness, band RMS, voicing, score) as a Metric | Definition | Value
// table. Returns a short note when no speech profile was elected.
func renderSpeechElected(p *processor.SpeechCandidateMetrics) string {
	if p == nil {
		return "_No speech profile elected._\n\n"
	}

	rows := [][]string{
		metricValueRow(durationSMetric, p.Region.Duration.Seconds()),
		metricValueRow(rmsLevelDBFSMetric, p.RMSLevel),
		metricValueRow(peakLevelDBFSMetric, p.PeakLevel),
		metricValueRow(crestFactorDBMetric, p.CrestFactor),
		metricValueRow(momentaryLUFSMetric, p.MomentaryLUFS),
		metricValueRow(shortTermLUFSMetric, p.ShortTermLUFS),
		metricValueRow(truePeakDBTPMetric, p.TruePeak),
		metricValueRow(samplePeakDBFSMetric, p.SamplePeak),
		metricValueRow(speechBandBodyRMSDBFSMetric, p.BodyBandRMS),
		metricValueRow(speechBandSibilantRMSDBFSMetric, p.SibBandRMS),
		metricValueRow(voicingDensityMetric, p.VoicingDensity),
		metricValueRow(scoreMetric, p.Score),
	}

	return renderValueTable("**Elected profile**\n\n", rows)
}

// renderCandidatesSummary renders the bare candidate summary: the evaluated count
// and the elected candidate's score. The full ranked candidate array is NOT inline
// (it streams to the .candidates.jsonl sidecar); this renderer emits count +
// elected only, with no per-candidate entropy/flatness/kurtosis gloss. Returns the
// empty string when no candidates were evaluated.
func renderCandidatesSummary(s *processor.CandidatesSummary) string {
	if s == nil {
		return ""
	}

	rows := [][]string{
		{"Evaluated count", "Number of region candidates evaluated.", formatInt(s.EvaluatedCount)},
	}
	if s.ElectedScore != nil {
		rows = append(rows, []string{metricLabel(scoreMetric), metricDefinition(scoreMetric), formatMetric(*s.ElectedScore, 4)})
	}

	var b strings.Builder
	b.WriteString("**Candidates**\n\n")
	b.WriteString(mdTable([]string{"Metric", "Definition", "Value"}, rows))
	b.WriteString("\n")
	return b.String()
}

// renderRegionSamples renders the per-stage Input/Filtered/Final region samples
// (amplitude, loudness, the 13 spectral metrics). All three stages share the
// *processor.RegionSample Go type, so the metricRow getters read the same fields
// off whichever stage is present - reusing stageColumns/renderMetricTable so an
// absent stage's column is omitted (analysis-only carries Input only) and a nil
// stage (e.g. room-tone Samples.Input) renders the placeholder.
func renderRegionSamples(s processor.RegionSamples) string {
	in, filt, final := s.Input, s.Filtered, s.Final

	row := func(metric metricDescriptor, f func(*processor.RegionSample) float64) metricRow {
		return metricRow{
			metric: metric,
			input:  stageGetter(in, f), filt: stageGetter(filt, f), final: stageGetter(final, f),
		}
	}
	spec := func(metric metricDescriptor) metricRow {
		return row(metric, func(rs *processor.RegionSample) float64 {
			return spectralMetricValue(metric, &rs.Spectral)
		})
	}

	rows := []metricRow{
		row(rmsLevelDBFSMetric, func(rs *processor.RegionSample) float64 { return rs.RMSLevel }),
		row(peakLevelDBFSMetric, func(rs *processor.RegionSample) float64 { return rs.PeakLevel }),
		row(regionSampleCrestFactorDBMetric, func(rs *processor.RegionSample) float64 { return rs.CrestFactor }),
		row(momentaryLUFSMetric, func(rs *processor.RegionSample) float64 { return rs.MomentaryLUFS }),
		row(shortTermLUFSMetric, func(rs *processor.RegionSample) float64 { return rs.ShortTermLUFS }),
		row(truePeakDBTPMetric, func(rs *processor.RegionSample) float64 { return rs.TruePeak }),
		row(samplePeakDBFSMetric, func(rs *processor.RegionSample) float64 { return rs.SamplePeak }),
	}
	for _, metric := range spectralMetricDescriptors {
		rows = append(rows, spec(metric))
	}

	var b strings.Builder
	b.WriteString("**Samples**\n\n")
	b.WriteString(renderMetricTable(rows))
	b.WriteString("\n")
	return b.String()
}

// =============================================================================
// Interval Summary
// =============================================================================

// renderIntervalSummary renders the per-250ms interval summary: the interval
// count, the RMS distribution percentiles, and the largest adjacent gap. The full
// per-interval series lives in the .intervals.jsonl sidecar; only this summary is
// inline. The RMS distribution and gap are present only when at least 10 intervals
// sit above digital silence (they are nil otherwise, so those rows drop). Reads
// only rec.IntervalSummary. Returns the empty string when no summary exists.
func renderIntervalSummary(rec *processor.RunRecord) string {
	s := rec.IntervalSummary
	if s == nil {
		return ""
	}

	rows := [][]string{
		{metricLabel(intervalCountMetric), metricDefinition(intervalCountMetric), formatInt(s.Count)},
	}
	if s.RMS != nil {
		rows = append(rows,
			metricValueRow(rmsDistributionMinDBFSMetric, s.RMS.Min),
			metricValueRow(rmsDistributionP10DBFSMetric, s.RMS.P10),
			metricValueRow(rmsDistributionP25DBFSMetric, s.RMS.P25),
			metricValueRow(rmsDistributionP50DBFSMetric, s.RMS.P50),
			metricValueRow(rmsDistributionP75DBFSMetric, s.RMS.P75),
			metricValueRow(rmsDistributionP90DBFSMetric, s.RMS.P90),
			metricValueRow(rmsDistributionMaxDBFSMetric, s.RMS.Max),
		)
	}
	if s.LargestGapDB != nil {
		rows = append(rows, metricValueRow(largestGapDBMetric, *s.LargestGapDB))
	}

	return renderValueTable("## Interval Summary\n\n", rows)
}

// =============================================================================
// Region/summary cell helpers
// =============================================================================

// renderValueTable builds a single-stage Metric | Definition | Value table under
// the given heading, with a trailing blank line. It owns the header literal, the
// builder, and the newline the five single-stage renderers (noise floor, gate
// statistics, room-tone/speech elected, interval summary) shared verbatim.
func renderValueTable(heading string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(heading)
	b.WriteString(mdTable([]string{"Metric", "Definition", "Value"}, rows))
	b.WriteString("\n")
	return b.String()
}

// metricValueRow builds a three-cell Metric | Definition | Value row for a
// single-stage table. The descriptor owns the key, definition, format rule, and
// decimals, so rows cannot repeat the raw metric key or formatter choice.
func metricValueRow(metric metricDescriptor, value float64) []string {
	return []string{
		metricLabel(metric),
		metricDefinition(metric),
		formatByRule(value, metric.format, metric.decimals),
	}
}

// unitMetricFormat exposes a descriptor's report format for the focused routing
// tests.
func unitMetricFormat(metric metricDescriptor) (metricFormat, int) {
	if _, ok := DefinitionFor(string(metric.key)); !ok {
		panic("report: unitMetricFormat: no definition for key " + string(metric.key))
	}
	return metric.format, metric.decimals
}

// stringCell renders a categorical string value, the placeholder when empty.
func stringCell(s string) string {
	if s == "" {
		return placeholder
	}
	return s
}

// boolCell renders a boolean flag as "yes"/"no" (objective, not a verdict).
func boolCell(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// formatInt renders an integer count cell.
func formatInt(n int) string {
	return strconv.Itoa(n)
}
