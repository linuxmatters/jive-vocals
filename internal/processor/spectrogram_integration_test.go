//go:build integration

package processor

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Integration-tagged render tests for internal/processor/spectrogram.go. They
// decode synthetic audio through the production showspectrumpic graph and assert
// the PNG that lands on disk. They run only under `-tags integration` (via
// `just test-integration`); the default `just test` suite excludes them and stays
// hermetic. The hermetic registry/spec/resolution tests live in spectrogram_test.go.
//
// The showspectrumpic output dimensions are frozen (s=1024x512 plus the fixed
// legend), so they are content- and duration-independent. A synthetic tone+noise
// stem written to t.TempDir() therefore exercises the render path fully without
// any testdata dependence.

// testSpectrogramInput generates a synthetic tone+noise stem in t.TempDir() for
// the render tests and returns its path.
func testSpectrogramInput(t *testing.T) string {
	t.Helper()
	return generateTestAudio(t, TestAudioOptions{
		DurationSecs: 6.0,
		ToneFreq:     440,
		ToneLevel:    -20.0,
		NoiseLevel:   -55.0,
		Dir:          t.TempDir(),
	})
}

// renderPNG runs generateSpectrogram with a live context and decodes the result,
// asserting the output is a decodable PNG with non-empty bounds. Fails on any
// render or decode error.
func renderPNG(t *testing.T, input string, bounds *regionBounds, pngPath string) {
	t.Helper()
	if err := generateSpectrogram(context.Background(), input, bounds, pngPath); err != nil {
		t.Fatalf("generateSpectrogram(%v) failed: %v", bounds, err)
	}
	if dx, dy := pngDims(t, pngPath); dx <= 0 || dy <= 0 {
		t.Fatalf("rendered png has empty bounds: %dx%d", dx, dy)
	}
}

// pngDims decodes pngPath and returns its width and height. Fails on error.
func pngDims(t *testing.T, pngPath string) (int, int) {
	t.Helper()
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatalf("open png %q: %v", pngPath, err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode png %q: %v", pngPath, err)
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

// TestGenerateSpectrogramWholeFile renders the whole file (nil bounds) and
// asserts the output is a decodable PNG with non-empty bounds.
func TestGenerateSpectrogramWholeFile(t *testing.T) {
	input := testSpectrogramInput(t)
	pngPath := filepath.Join(t.TempDir(), "whole.png")
	renderPNG(t, input, nil, pngPath)
}

// TestGenerateSpectrogramRegion renders a bounded window (a few seconds in for a
// few seconds) and asserts the output is a decodable PNG with non-empty bounds.
func TestGenerateSpectrogramRegion(t *testing.T) {
	input := testSpectrogramInput(t)
	pngPath := filepath.Join(t.TempDir(), "region.png")
	renderPNG(t, input, &regionBounds{Start: 2.0, Duration: 3.0}, pngPath)
}

// TestGenerateSpectrogramCancellation passes a cancelled context and asserts the
// render aborts with an error and removes the partial PNG (the deferred cleanup
// in generateSpectrogram).
func TestGenerateSpectrogramCancellation(t *testing.T) {
	input := testSpectrogramInput(t)
	pngPath := filepath.Join(t.TempDir(), "cancelled.png")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := generateSpectrogram(ctx, input, nil, pngPath); err == nil {
		t.Fatal("cancelled ctx: want non-nil error, got nil")
	}
	if _, err := os.Stat(pngPath); !os.IsNotExist(err) {
		t.Fatalf("cancelled render must leave no partial png; os.Stat(%q) err=%v (want not-exist)", pngPath, err)
	}
}

// TestGenerateSpectrogramDimensionParity renders a whole-file image and a region
// image of the SAME input, decodes both, and asserts identical Dx()/Dy().
// The frozen s=1024x512 plus the fixed legend make dimensions content- and
// duration-independent, so the before/after pair always matches pixel-for-pixel
// in size.
func TestGenerateSpectrogramDimensionParity(t *testing.T) {
	input := testSpectrogramInput(t)
	dir := t.TempDir()
	wholePath := filepath.Join(dir, "whole.png")
	regionPath := filepath.Join(dir, "region.png")

	if err := generateSpectrogram(context.Background(), input, nil, wholePath); err != nil {
		t.Fatalf("whole-file render failed: %v", err)
	}
	if err := generateSpectrogram(context.Background(), input, &regionBounds{Start: 2.0, Duration: 3.0}, regionPath); err != nil {
		t.Fatalf("region render failed: %v", err)
	}

	wDx, wDy := pngDims(t, wholePath)
	rDx, rDy := pngDims(t, regionPath)
	if wDx != rDx || wDy != rDy {
		t.Fatalf("dimension parity broken: whole=%dx%d region=%dx%d", wDx, wDy, rDx, rDy)
	}
}
