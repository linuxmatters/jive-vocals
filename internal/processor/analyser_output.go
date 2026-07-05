// Package processor handles audio analysis and processing
package processor

import (
	"context"
	"fmt"
	"time"

	ffmpeg "github.com/linuxmatters/ffmpeg-statigo"
	"github.com/linuxmatters/jive-vocals/internal/audio"
)

// outputRegionAnalysisFilterFormat is the fmt.Sprintf format string for the
// output-region analysis filter graph in measureOutputRegionFromReader. The
// %f verbs take the region start and duration in seconds. Hoisted to a
// package-level constant so guard tests can assert the metadata flags against
// live source without re-typing the filter string.
const outputRegionAnalysisFilterFormat = "atrim=start=%f:duration=%f,asetpts=PTS-STARTPTS,astats=metadata=1:measure_perchannel=0,aspectralstats=measure=all,ebur128=metadata=1:peak=sample+true"

// regionMeasurements holds the common measurement results from analysing an
// output audio region. Both room tone and speech region measurement functions
// share this intermediate type before mapping to their specific candidate types.
type regionMeasurements struct {
	RMSLevel        float64
	PeakLevel       float64
	CrestFactor     float64
	Spectral        SpectralMetrics
	MomentaryLUFS   float64
	ShortTermLUFS   float64
	TruePeak        float64
	SamplePeak      float64
	FramesProcessed int64
}

// toRegionSample maps the measured region metrics to a bare RegionSample
// (amplitude/spectral/loudness only). FramesProcessed is a measurement-internal
// counter and is not carried onto the sample. Both output region wrappers share
// this so the eight-field copy lives in one place.
func (r *regionMeasurements) toRegionSample() *RegionSample {
	return &RegionSample{
		RMSLevel:      r.RMSLevel,
		PeakLevel:     r.PeakLevel,
		CrestFactor:   r.CrestFactor,
		Spectral:      r.Spectral,
		MomentaryLUFS: r.MomentaryLUFS,
		ShortTermLUFS: r.ShortTermLUFS,
		TruePeak:      r.TruePeak,
		SamplePeak:    r.SamplePeak,
	}
}

// measureOutputRegionFromReader measures amplitude, spectral, and loudness
// metrics for a time region in an already-opened audio file. This is the
// shared implementation behind measureOutputRoomToneRegionFromReader and
// measureOutputSpeechRegionFromReader.
func measureOutputRegionFromReader(ctx context.Context, reader *audio.Reader, start, duration time.Duration, log debugLogger) (*regionMeasurements, error) {
	filterSpec := fmt.Sprintf(
		outputRegionAnalysisFilterFormat,
		start.Seconds(),
		duration.Seconds(),
	)

	var rmsLevel float64
	var peakLevel float64
	var crestFactor float64
	var momentaryLUFS float64
	var shortTermLUFS float64
	var truePeak float64
	var samplePeak float64
	var rmsLevelFound bool
	var framesProcessed int64

	var spectralAcc SpectralMetrics
	var spectralFrameCount int64

	extractMeasurements := func(_ *ffmpeg.AVFrame, filteredFrame *ffmpeg.AVFrame) error {
		if metadata := filteredFrame.Metadata(); metadata != nil {
			if value, ok := getFloatMetadata(metadata, metaKeyOverallRMSLevel); ok {
				rmsLevel = value
				rmsLevelFound = true
			}
			if value, ok := getFloatMetadata(metadata, metaKeyOverallPeakLevel); ok {
				peakLevel = value
			}
			if value, ok := getFloatMetadata(metadata, metaKeyOverallCrestFactor); ok {
				crestFactor = value
			}

			sm := extractSpectralMetrics(metadata)
			if sm.Found {
				spectralAcc.add(sm)
				spectralFrameCount++
			}

			if value, ok := getFloatMetadata(metadata, metaKeyEbur128M); ok {
				momentaryLUFS = value
			}
			if value, ok := getFloatMetadata(metadata, metaKeyEbur128S); ok {
				shortTermLUFS = value
			}
			if value, ok := getFloatMetadata(metadata, metaKeyEbur128TruePeak); ok {
				truePeak = value
			}
			if value, ok := getFloatMetadata(metadata, metaKeyEbur128SamplePeak); ok {
				samplePeak = value
			}
		}

		framesProcessed++
		return nil
	}

	if err := runRegionMeasurementGraph(ctx, reader, start, duration, filterSpec, "analysis", log, FrameLoopConfig{
		OnPushError: logAndSkipOptionalMeasurementFrameError(log, "output region push"),
		OnPullError: logAndSkipOptionalMeasurementFrameError(log, "output region pull"),
		OnFrame:     extractMeasurements,
	}); err != nil {
		return nil, err
	}

	if framesProcessed == 0 {
		return nil, fmt.Errorf("no frames processed in region")
	}

	var avg SpectralMetrics
	if spectralFrameCount > 0 {
		avg = spectralAcc.average(float64(spectralFrameCount))
	}

	log.Logf("  Frames processed: %d", framesProcessed)
	log.Logf("  Spectral frames: %d", spectralFrameCount)
	log.Logf("  Final ebur128 values:")
	log.Logf("    momentaryLUFS: %f", momentaryLUFS)
	log.Logf("    shortTermLUFS: %f", shortTermLUFS)
	log.Logf("    truePeak: %f", truePeak)
	log.Logf("    samplePeak: %f", samplePeak)
	log.Logf("  Final astats values:")
	log.Logf("    rmsLevel: %f (found: %v)", rmsLevel, rmsLevelFound)
	log.Logf("    peakLevel: %f", peakLevel)
	log.Logf("  Averaged spectral values:")
	log.Logf("    spectralCentroid: %f", avg.Centroid)
	log.Logf("    spectralRolloff: %f", avg.Rolloff)

	ebur128Valid := momentaryLUFS != 0.0 || shortTermLUFS != 0.0 || truePeak != 0.0
	if !ebur128Valid {
		log.Logf("Warning: ebur128 measurements not captured (insufficient duration or warmup time)")
	}

	if crestFactor == 0.0 && rmsLevelFound && peakLevel != 0 {
		crestFactor = peakLevel - rmsLevel
	}

	result := &regionMeasurements{
		RMSLevel:        rmsLevel,
		PeakLevel:       peakLevel,
		CrestFactor:     crestFactor,
		Spectral:        avg,
		MomentaryLUFS:   momentaryLUFS,
		ShortTermLUFS:   shortTermLUFS,
		TruePeak:        linearRatioToDB(truePeak),
		SamplePeak:      linearRatioToDB(samplePeak),
		FramesProcessed: framesProcessed,
	}

	if !rmsLevelFound {
		result.RMSLevel = -60.0 // Conservative fallback
	}

	return result, nil
}

// measureOutputRoomToneRegionFromReader measures a room tone region and maps
// the result to a bare RegionSample (amplitude/spectral/loudness only). Output
// re-measure never scores or elects, so no candidate scoring/band
// fields are produced.
func measureOutputRoomToneRegionFromReader(ctx context.Context, reader *audio.Reader, region RoomToneRegion, log debugLogger) (*RegionSample, error) {
	log.Logf("=== measureOutputRoomToneRegion: start=%.3fs, duration=%.3fs ===",
		region.Start.Seconds(), region.Duration.Seconds())

	result, err := measureOutputRegionFromReader(ctx, reader, region.Start, region.Duration, log)
	if err != nil {
		return nil, err
	}

	log.Logf("=== measureOutputRoomToneRegion SUMMARY ===")

	return result.toRegionSample(), nil
}

// extractRegionPair builds optional RoomToneRegion and SpeechRegion pointers
// from AudioMeasurements profiles. Returns (nil, nil) when both profiles are absent.
func extractRegionPair(m *AudioMeasurements) (*RoomToneRegion, *SpeechRegion) {
	var roomToneRegion *RoomToneRegion
	var spRegion *SpeechRegion
	if m.Regions.NoiseProfile != nil {
		roomToneRegion = &RoomToneRegion{
			Start:    m.Regions.NoiseProfile.Start,
			End:      m.Regions.NoiseProfile.Start + m.Regions.NoiseProfile.Duration,
			Duration: m.Regions.NoiseProfile.Duration,
		}
	}
	if m.Regions.SpeechProfile != nil {
		spRegion = &SpeechRegion{
			Start:    m.Regions.SpeechProfile.Region.Start,
			End:      m.Regions.SpeechProfile.Region.End,
			Duration: m.Regions.SpeechProfile.Region.Duration,
		}
	}
	return roomToneRegion, spRegion
}

// MeasureOutputRegions measures both room tone and speech regions from the same
// output file in a single open/close cycle. This avoids redundant file opens,
// demuxing, and decoding that would occur if room tone and speech regions were
// measured in separate passes.
//
// Either region parameter may be nil to skip that measurement. Returns nil for
// any skipped or failed measurement (non-fatal - matches existing behaviour).
func MeasureOutputRegions(ctx context.Context, outputPath string, roomToneRegion *RoomToneRegion, speechRegion *SpeechRegion, log debugLogger) (*RegionSample, *RegionSample) {
	if roomToneRegion == nil && speechRegion == nil {
		return nil, nil
	}

	// Open the output file once for both measurements
	reader, _, err := audio.OpenAudioFile(outputPath)
	if err != nil {
		log.Logf("Warning: Failed to open output file for region measurements: %v", err)
		return nil, nil
	}
	defer reader.Close()

	// Measure room tone region first (if requested)
	var roomToneMetrics *RegionSample
	if roomToneRegion != nil {
		roomToneMetrics, err = measureOutputRoomToneRegionFromReader(ctx, reader, *roomToneRegion, log)
		if err != nil {
			log.Logf("Warning: Failed to measure room tone region: %v", err)
			// Non-fatal - continue to speech measurement
		}
	}

	// Measure the speech region. No explicit seek-back is needed here:
	// measureOutputRegionFromReader seeks the demuxer near the region start
	// itself (seekReaderBeforeRegion), which repositions the reader regardless
	// of where the room-tone pass left it.
	if speechRegion != nil {
		speechMetrics, err := measureOutputSpeechRegionFromReader(ctx, reader, *speechRegion, log)
		if err != nil {
			log.Logf("Warning: Failed to measure speech region: %v", err)
			return roomToneMetrics, nil
		}
		return roomToneMetrics, speechMetrics
	}

	return roomToneMetrics, nil
}

// measureOutputSpeechRegionFromReader measures a speech region and maps
// the result to a bare RegionSample (amplitude/spectral/loudness only). Output
// re-measure never scores or elects, so no candidate scoring/band fields are
// produced.
func measureOutputSpeechRegionFromReader(ctx context.Context, reader *audio.Reader, region SpeechRegion, log debugLogger) (*RegionSample, error) {
	log.Logf("=== measureOutputSpeechRegion: start=%.3fs, duration=%.3fs ===",
		region.Start.Seconds(), region.Duration.Seconds())

	result, err := measureOutputRegionFromReader(ctx, reader, region.Start, region.Duration, log)
	if err != nil {
		return nil, err
	}

	log.Logf("=== measureOutputSpeechRegion SUMMARY ===")

	return result.toRegionSample(), nil
}
