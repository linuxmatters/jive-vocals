// Package processor handles audio analysis and processing
package processor

import (
	"math"
	"testing"
	"unsafe"

	ffmpeg "github.com/linuxmatters/ffmpeg-statigo"
)

// newFloatTestFrame allocates a mono float (AVSampleFmtFlt) frame of nbSamples
// with every sample set to amplitude. The caller must free the returned frame.
func newFloatTestFrame(t *testing.T, nbSamples int, amplitude float32) *ffmpeg.AVFrame {
	t.Helper()

	frame := ffmpeg.AVFrameAlloc()
	if frame == nil {
		t.Fatal("AVFrameAlloc returned nil")
	}
	frame.SetNbSamples(nbSamples)
	frame.SetFormat(int(ffmpeg.AVSampleFmtFlt))
	ffmpeg.AVChannelLayoutDefault(frame.ChLayout(), 1)

	if _, err := ffmpeg.AVFrameGetBuffer(frame, 0); err != nil {
		ffmpeg.AVFrameFree(&frame)
		t.Fatalf("AVFrameGetBuffer error = %v", err)
	}

	dataPtr := frame.Data().Get(0)
	if dataPtr == nil {
		ffmpeg.AVFrameFree(&frame)
		t.Fatal("frame data plane 0 is nil")
	}
	samples := unsafe.Slice((*float32)(dataPtr), nbSamples)
	for i := range samples {
		samples[i] = amplitude
	}
	return frame
}

func TestCalculateFrameLevelFloorsAtMeterFloor(t *testing.T) {
	if meterLevelFloorDB != -70.0 {
		t.Fatalf("meterLevelFloorDB = %v, want -70.0 (must match ui.meterFloorDB)", meterLevelFloorDB)
	}

	tests := []struct {
		name      string
		amplitude float32 // constant sample value; RMS == amplitude for a DC frame
		wantDB    float64
	}{
		// -65 dBFS (10^(-65/20)): previously floored to -60, now reads through.
		{name: "quiet -65 dB reads below -60", amplitude: float32(math.Pow(10, -65.0/20.0)), wantDB: -65.0},
		// -90 dBFS is below the meter floor and must clamp to -70, not -inf.
		{name: "very quiet -90 dB clamps to -70 floor", amplitude: float32(math.Pow(10, -90.0/20.0)), wantDB: -70.0},
		// Digital silence must not produce -inf garbage.
		{name: "digital silence clamps to -70 floor", amplitude: 0.0, wantDB: -70.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := newFloatTestFrame(t, 1024, tt.amplitude)
			defer ffmpeg.AVFrameFree(&frame)

			got := calculateFrameLevel(frame)

			if math.IsInf(got, 0) || math.IsNaN(got) {
				t.Fatalf("calculateFrameLevel = %v, want finite value", got)
			}
			if got < meterLevelFloorDB {
				t.Errorf("calculateFrameLevel = %.2f dB, below meter floor %.1f dB", got, meterLevelFloorDB)
			}
			if math.Abs(got-tt.wantDB) > 0.5 {
				t.Errorf("calculateFrameLevel = %.2f dB, want ~%.1f dB", got, tt.wantDB)
			}
		})
	}
}

// TestCalculateFrameLevelBelowOldMinus60Floor pins the regression: quiet audio
// that the old -60 dB clamp would have floored must now report below -60 dB.
func TestCalculateFrameLevelBelowOldMinus60Floor(t *testing.T) {
	frame := newFloatTestFrame(t, 1024, float32(math.Pow(10, -68.0/20.0)))
	defer ffmpeg.AVFrameFree(&frame)

	got := calculateFrameLevel(frame)
	if got >= -60.0 {
		t.Errorf("calculateFrameLevel = %.2f dB, want < -60 dB (old floor removed)", got)
	}
	if got < meterLevelFloorDB {
		t.Errorf("calculateFrameLevel = %.2f dB, below meter floor %.1f dB", got, meterLevelFloorDB)
	}
}
