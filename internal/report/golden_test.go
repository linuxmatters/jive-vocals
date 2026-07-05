package report

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/linuxmatters/jive-vocals/internal/processor"
)

// updateGolden regenerates the checked-in golden files instead of asserting
// against them. Run `go test ./internal/report -run TestGolden -update` after an
// intentional rendering change, then hand-review the diff before committing.
var updateGolden = flag.Bool("update", false, "regenerate golden report files")

// goldenPath is the checked-in golden. It lives BESIDE the test (not under a
// testdata/ subdir): .gitignore ignores any testdata/ directory recursively, so a
// golden under internal/report/testdata/ would not be version-controlled and the
// regression guard would be local-only. A *.golden file directly in the package
// directory commits and travels across machines.
const goldenPath = "report_full.md.golden"

// goldenTimings is the fixed run metadata spliced into the golden so the
// Processing Summary section renders deterministically.
var goldenTimings = Timings{
	Pass1:          2 * time.Second,
	Pass2:          90 * time.Second,
	Pass3:          3 * time.Second,
	Pass4:          12 * time.Second,
	RealTimeFactor: 12.5,
}

// TestGoldenFullReport pins the complete rendered Markdown for a representative
// FULL processing record (every section: staged loudness/dynamics/spectral, noise,
// regions with elected room-tone + speech, interval summary, filter chain, peak
// limiter, loudnorm). The record is built IN-MEMORY via the production assembly
// path (NewRunRecord / NewAnalysisRunRecord) so the golden is complete and
// CI-runnable WITHOUT the gitignored corpus. Any rendered drift fails this test;
// regenerate with -update after review.
func TestGoldenFullReport(t *testing.T) {
	got := RenderMarkdown(fullProcessingRecord(), goldenTimings)

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("golden regenerated: %s (%d bytes)", goldenPath, len(got))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered report differs from golden %s; re-run with -update if intended.\n--- got ---\n%s", goldenPath, got)
	}
}

// TestGoldenNoInterpretationTokens grep-asserts the criterion-5 bans over the
// PINNED golden output: no verdict glyphs, no range-to-meaning character words, no
// gain-normalisation dagger, no recording-tips heading.
func TestGoldenNoInterpretationTokens(t *testing.T) {
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	got := string(want)
	for _, banned := range []string{
		"✓", "⚠", "❌", // verdict glyphs
		"Character",
		"(warm)", "(bright)", "(tonal)", "(broadband)",
		"†", // gain-normalisation dagger
		"Recording Tips", "Recording tips", "recording tips",
	} {
		if strings.Contains(got, banned) {
			t.Errorf("golden contains banned token %q (criterion 5)", banned)
		}
	}
}

// TestRoundTripFromEmittedJSON proves the emitted run-record JSON round-trips into
// a *processor.RunRecord that RenderMarkdown turns into the same elected-profile
// and normalisation sections as the live record path.
func TestRoundTripFromEmittedJSON(t *testing.T) {
	data, err := processor.MarshalRunRecord(fullProcessingRecord())
	if err != nil {
		t.Fatalf("marshalling emitted JSON: %v", err)
	}

	var rec processor.RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshalling emitted .json into RunRecord: %v", err)
	}

	got := RenderMarkdown(&rec, Timings{})
	if strings.TrimSpace(got) == "" {
		t.Fatal("render-from-unmarshalled-json produced an empty report")
	}

	for _, want := range []string{
		"# Audio Processing Report",
		"## Loudness",
		"## Dynamics",
		"## Spectral",
		"## Noise Floor",
		"## Regions",
		"### Room Tone",
		"### Speech",
		"**Elected profile**",
		"Measured floor",
		"Voicing density",
		"**Candidates**",
		"**Samples**",
		"## Interval Summary",
		"## Filter Chain",
		"### Speech gate",
		"## Peak Limiter",
		"## Loudnorm",
		"Measured output integrated (LUFS)",
		"Normalisation type",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("round-trip report missing %q", want)
		}
	}

	if count := strings.Count(got, "**Elected profile**"); count != 2 {
		t.Errorf("round-trip elected profile count = %d, want 2\n%s", count, got)
	}
}
