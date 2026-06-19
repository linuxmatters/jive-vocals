package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// This file holds the one round-trip test that exercises reader.go's hardest
// code - the ReadFrame EAGAIN/EOF decode loop and the OpenAudioFile/Close
// resource lifecycle - against a real decode. It stays hermetic: the audio is a
// synthetic 16-bit PCM WAV written into t.TempDir() with Go stdlib only, so
// nothing under testdata/ is touched and CI (which ships no audio fixtures) can
// run it. WAV/PCM is chosen because it needs no encoder: ffmpeg's built-in
// pcm_s16le decoder drains it, so the test verifies the real decode path rather
// than a mock.

// writeMonoWAV writes a mono 16-bit little-endian PCM WAV file holding samples
// at sampleRate. It mirrors the canonical 44-byte RIFF/WAVE header so ffmpeg's
// demuxer and pcm decoder accept it. It returns an error rather than failing
// the test so the caller controls cleanup ordering.
func writeMonoWAV(path string, samples []int16, sampleRate int) error {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)

	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(samples) * 2 // 2 bytes per 16-bit sample
	fileSize := 36 + dataSize    // total size minus the 8-byte RIFF prefix

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// RIFF header
	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(fileSize)); err != nil { //nolint:gosec // test file sizes are small
		return err
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		return err
	}

	// fmt subchunk
	if _, err := f.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil { // subchunk size
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil { // PCM format
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil { //nolint:gosec // sample rate fits in uint32
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil { //nolint:gosec // byte rate fits in uint32
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// data subchunk
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil { //nolint:gosec // test data sizes are small
		return err
	}
	for _, s := range samples {
		if err := binary.Write(f, binary.LittleEndian, s); err != nil {
			return err
		}
	}

	return f.Close()
}

// TestOpenAudioFile_RoundTripToEOF is the only test that drives reader.go's
// real decode loop end to end. The error-path tests in reader_test.go cover
// open failures and Close nil-guards but never decode a frame, so the
// EAGAIN/EOF receive-send-flush loop and the success lifecycle (open hands
// ownership to the Reader, Close frees it) went unexercised. This generates a
// short synthetic WAV, opens it, drains every frame, and asserts the happy
// path: frames are returned, EOF maps to the documented (nil, nil), metadata
// is sane, and Close releases without panic.
func TestOpenAudioFile_RoundTripToEOF(t *testing.T) {
	t.Parallel()

	const (
		sampleRate = 44100
		durationS  = 0.25 // fraction of a second keeps the test fast
		toneFreq   = 440.0
	)

	totalSamples := int(durationS * sampleRate)
	samples := make([]int16, totalSamples)
	amp := 0.5 * float64(math.MaxInt16) // -6 dBFS sine, well clear of clipping
	for i := range samples {
		t := float64(i) / float64(sampleRate)
		samples[i] = int16(amp * math.Sin(2.0*math.Pi*toneFreq*t))
	}

	path := filepath.Join(t.TempDir(), "roundtrip.wav")
	if err := writeMonoWAV(path, samples, sampleRate); err != nil {
		t.Fatalf("writing synthetic WAV: %v", err)
	}

	r, meta, err := OpenAudioFile(path)
	if err != nil {
		t.Fatalf("OpenAudioFile(%q): unexpected error: %v", path, err)
	}
	if r == nil {
		t.Fatal("OpenAudioFile: want non-nil Reader on success, got nil")
	}
	defer r.Close()

	// Metadata must reflect what we wrote: mono, the requested rate, and a
	// duration close to 0.25 s. Container rounding makes an exact match brittle,
	// so bound it rather than equate it.
	if meta == nil {
		t.Fatal("OpenAudioFile: want non-nil Metadata on success, got nil")
	}
	if meta.SampleRate != sampleRate {
		t.Errorf("Metadata.SampleRate = %d, want %d", meta.SampleRate, sampleRate)
	}
	if meta.Channels != 1 {
		t.Errorf("Metadata.Channels = %d, want 1", meta.Channels)
	}
	if meta.Duration <= 0 || meta.Duration > 1.0 {
		t.Errorf("Metadata.Duration = %v s, want within (0, 1] for a 0.25 s clip", meta.Duration)
	}

	// Drain the decoder to EOF. The loop must return at least one frame, the
	// returned samples must sum to the count we encoded (the decode is lossless
	// for PCM), and EOF must surface as (nil, nil) - never an error.
	var (
		frameCount   int
		decodedSamps int
	)
	for {
		frame, ferr := r.ReadFrame()
		if ferr != nil {
			t.Fatalf("ReadFrame: unexpected error after %d frames: %v", frameCount, ferr)
		}
		if frame == nil {
			break // documented EOF sentinel
		}
		frameCount++
		decodedSamps += frame.NbSamples()
	}

	if frameCount == 0 {
		t.Fatal("ReadFrame: drained zero frames from a non-empty file")
	}
	if decodedSamps != totalSamples {
		t.Errorf("decoded sample count = %d, want %d (lossless PCM round-trip)", decodedSamps, totalSamples)
	}

	// A second drain attempt at EOF must keep returning (nil, nil); the loop
	// must not error or hang once the stream is exhausted.
	frame, ferr := r.ReadFrame()
	if ferr != nil {
		t.Errorf("ReadFrame past EOF: want (nil, nil), got error %v", ferr)
	}
	if frame != nil {
		t.Errorf("ReadFrame past EOF: want nil frame, got %v", frame)
	}

	// Close must release cleanly and be safe to call again (the lifecycle the
	// deferred Close above also relies on); a double free would panic here.
	r.Close()
	r.Close()
}
