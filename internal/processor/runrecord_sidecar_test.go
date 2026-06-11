package processor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestRunRecord_IntervalSummaryInlineSeriesAbsent asserts task 3.1's split: the
// inline record carries interval_summary (count + RMS distribution + largest gap)
// and never the full interval_samples series.
func TestRunRecord_IntervalSummaryInlineSeriesAbsent(t *testing.T) {
	m := populatedAudioMeasurements()
	m.Regions.IntervalSamples = syntheticIntervals(20)

	rec := NewAnalysisRunRecord("/tmp/episode.flac", m)
	tree, raw := marshalRecordTree(t, rec)

	summary, ok := tree["interval_summary"].(map[string]any)
	if !ok {
		t.Fatal("missing interval_summary block")
	}
	if summary["count"].(float64) != 20 {
		t.Errorf("interval_summary.count = %v, want 20", summary["count"])
	}
	dist, ok := summary["rms_distribution"].(map[string]any)
	if !ok {
		t.Fatal("missing interval_summary.rms_distribution")
	}
	for _, key := range []string{"min_dbfs", "p10_dbfs", "p25_dbfs", "p50_dbfs", "p75_dbfs", "p90_dbfs", "max_dbfs"} {
		if _, present := dist[key]; !present {
			t.Errorf("rms_distribution missing %q", key)
		}
	}
	if _, present := summary["largest_gap_db"]; !present {
		t.Error("interval_summary missing largest_gap_db")
	}

	// The full series must be absent everywhere in the record.
	if bytes.Contains(raw, []byte("interval_samples")) {
		t.Error("record must not inline interval_samples (sidecar only)")
	}
}

// TestNewIntervalSummary_MatchesReportMaths asserts the percentile/gap selection
// reproduces the .log diagnostic's integer-index maths exactly.
func TestNewIntervalSummary_MatchesReportMaths(t *testing.T) {
	// 11 distinct RMS values above the -120 silence floor.
	vals := []float64{-70, -68, -66, -64, -62, -40, -38, -36, -34, -32, -30}
	samples := make([]IntervalSample, 0, len(vals)+1)
	samples = append(samples, IntervalSample{RMSLevel: -130}) // digital silence, excluded
	for _, v := range vals {
		samples = append(samples, IntervalSample{RMSLevel: v})
	}

	s := newIntervalSummary(samples)
	if s.Count != len(samples) {
		t.Errorf("Count = %d, want %d (includes silence interval)", s.Count, len(samples))
	}
	if s.RMS == nil {
		t.Fatal("RMS distribution nil, want populated (>=10 non-silence intervals)")
	}
	// sorted = vals (already ascending); n = 11. Index selection per report.
	want := RMSDistribution{
		Min: vals[0], P10: vals[11/10], P25: vals[11/4], P50: vals[11/2],
		P75: vals[11*3/4], P90: vals[11*9/10], Max: vals[len(vals)-1],
	}
	if *s.RMS != want {
		t.Errorf("RMS distribution = %+v, want %+v", *s.RMS, want)
	}
	// Largest gap is the 22 dB jump from -62 to -40.
	if s.LargestGapDB == nil || *s.LargestGapDB != 22 {
		t.Errorf("largest gap = %v, want 22", s.LargestGapDB)
	}
}

// TestNewIntervalSummary_BelowThresholdDropsDistribution mirrors the .log: fewer
// than 10 non-silence intervals prints count only, no distribution/gap.
func TestNewIntervalSummary_BelowThresholdDropsDistribution(t *testing.T) {
	samples := syntheticIntervals(5)
	s := newIntervalSummary(samples)
	if s == nil || s.Count != 5 {
		t.Fatalf("summary = %+v, want count 5", s)
	}
	if s.RMS != nil || s.LargestGapDB != nil {
		t.Error("distribution/gap must drop below 10 non-silence intervals")
	}
}

// TestRunRecord_CandidatesSummaryInlineArraysAbsent asserts the candidate split:
// each kind carries a candidates_summary (count + elected score) and never the
// full candidate array inline.
func TestRunRecord_CandidatesSummaryInlineArraysAbsent(t *testing.T) {
	rec := NewRunRecord(populatedProcessingResult())
	tree, raw := marshalRecordTree(t, rec)

	regions := tree["regions"].(map[string]any)
	for _, kind := range []string{"room_tone", "speech"} {
		block := regions[kind].(map[string]any)
		cs, ok := block["candidates_summary"].(map[string]any)
		if !ok {
			t.Errorf("regions.%s missing candidates_summary", kind)
			continue
		}
		if _, present := cs["evaluated_count"]; !present {
			t.Errorf("regions.%s.candidates_summary missing evaluated_count", kind)
		}
		if _, present := block["candidates"]; present {
			t.Errorf("regions.%s must not inline full candidates array", kind)
		}
	}

	// Speech elected score is present (SpeechProfile aliases an evaluated candidate).
	spcs := regions["speech"].(map[string]any)["candidates_summary"].(map[string]any)
	if _, present := spcs["elected_score"]; !present {
		t.Error("speech candidates_summary missing elected_score")
	}

	if bytes.Contains(raw, []byte("room_tone_candidates")) || bytes.Contains(raw, []byte("speech_candidates")) {
		t.Error("record must not inline the full candidate arrays")
	}
}

// TestWriteIntervalsSidecar_OneLinePerSample asserts the streaming writer emits
// exactly N lines for N intervals, each a valid JSON object with the flattened
// spectral_* keys (IntervalSample.MarshalJSON).
func TestWriteIntervalsSidecar_OneLinePerSample(t *testing.T) {
	samples := syntheticIntervals(7)
	var buf bytes.Buffer
	if err := streamIntervals(&buf, samples); err != nil {
		t.Fatalf("write intervals: %v", err)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != len(samples) {
		t.Fatalf("line count = %d, want %d", len(lines), len(samples))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d invalid JSON: %v\n%s", i, err, line)
		}
		if _, ok := obj["spectral_mean"]; !ok {
			t.Errorf("line %d missing flattened spectral_mean key", i)
		}
	}
}

// TestWriteCandidatesSidecar_TaggedLines asserts the candidates sidecar emits
// room-tone then speech lines, each tagged with kind, total N+M lines.
func TestWriteCandidatesSidecar_TaggedLines(t *testing.T) {
	rt := []RoomToneCandidateMetrics{{Score: 1}, {Score: 2}, {Score: 3}}
	sp := []SpeechCandidateMetrics{{Score: 9}, {Score: 8}}

	var buf bytes.Buffer
	if err := streamCandidates(&buf, rt, sp); err != nil {
		t.Fatalf("write candidates: %v", err)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != len(rt)+len(sp) {
		t.Fatalf("line count = %d, want %d", len(lines), len(rt)+len(sp))
	}
	wantKinds := []string{"room_tone", "room_tone", "room_tone", "speech", "speech"}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d invalid JSON: %v\n%s", i, err, line)
		}
		if obj["kind"] != wantKinds[i] {
			t.Errorf("line %d kind = %v, want %v", i, obj["kind"], wantKinds[i])
		}
		// The candidate's own fields are spliced in alongside the kind tag.
		if _, ok := obj["score"]; !ok {
			t.Errorf("line %d missing candidate score field", i)
		}
	}
}

// syntheticIntervals builds n IntervalSamples with ascending RMS levels above the
// digital-silence floor, each carrying a spectral block.
func syntheticIntervals(n int) []IntervalSample {
	out := make([]IntervalSample, n)
	for i := range out {
		out[i] = IntervalSample{
			Timestamp: time.Duration(i) * 250 * time.Millisecond,
			RMSLevel:  -60 + float64(i),
			PeakLevel: -40 + float64(i),
			Spectral:  SpectralMetrics{Mean: float64(i), Centroid: 2000, Found: true},
		}
	}
	return out
}

func nonEmptyLines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
