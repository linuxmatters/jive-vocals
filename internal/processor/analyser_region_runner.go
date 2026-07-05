// Package processor handles audio analysis and processing
package processor

import (
	"context"
	"fmt"
	"time"

	ffmpeg "github.com/linuxmatters/ffmpeg-statigo"
	"github.com/linuxmatters/jive-vocals/internal/audio"
)

// regionSeekPreRoll is the head-start the demuxer seeks before a region's start
// so decoding skips the pre-region span instead of running from frame 0.
//
// Why this never shifts the measured values: atrim is the FIRST filter in the
// graph and keys off each frame's file-absolute PTS (ReadFrame sets the frame
// PTS from the demuxer's best-effort timestamp; the abuffer source uses the
// stream packet time base). Seeking changes where DECODING begins, not the PTS
// the frames carry, so atrim=start=region.Start selects the exact same absolute
// samples regardless of the seek point. astats, aspectralstats, and ebur128 all
// sit after atrim, so they only ever see the windowed frames - the pre-roll span
// is discarded before it reaches any measurement filter. The measured window is
// therefore byte-identical to the from-frame-0 path for any seek at or before
// region.Start.
//
// The pre-roll's only job is to guarantee the seek lands at or before
// region.Start. AVFormatSeekFile (flags=0) seeks BACKWARD to a keyframe at or
// before the requested timestamp, so the effective decode start is already <=
// the seek target; the pre-roll adds further slack. 5s comfortably exceeds
// ebur128's longest integration window (3s short-term, 400ms momentary) so even
// if a future change moved a measurement filter ahead of atrim, the warm-up
// would still be covered. Decoder/filter warm-up before atrim is free here:
// those frames are trimmed away.
const regionSeekPreRoll = 5 * time.Second

// seekReaderBeforeRegion seeks the demuxer to regionStart-regionSeekPreRoll
// (floored at 0) so the pre-region span is skipped before the atrim window is
// decoded. The atrim start stays region-absolute and unchanged; see
// regionSeekPreRoll for why this preserves byte-identical measurements. A seek
// failure is non-fatal - decoding simply continues from the current position,
// and atrim still selects the correct window.
func seekReaderBeforeRegion(reader *audio.Reader, regionStart time.Duration, log debugLogger) {
	seekTarget := max(regionStart-regionSeekPreRoll, 0)
	seekTS := seekTarget.Microseconds() // AV_TIME_BASE is microseconds
	if err := reader.SeekTo(seekTS); err != nil {
		log.Logf("Warning: failed to seek before region (start=%.3fs, target=%.3fs): %v; decoding from current position",
			regionStart.Seconds(), seekTarget.Seconds(), err)
	}
}

// runRegionMeasurementGraph validates a time region, seeks near its start,
// creates the filter graph, and runs the frame loop. Callers provide only the
// filter string and metric extraction callbacks.
func runRegionMeasurementGraph(
	ctx context.Context,
	reader *audio.Reader,
	start, duration time.Duration,
	filterSpec string,
	graphName string,
	log debugLogger,
	config FrameLoopConfig,
) error {
	if start < 0 {
		return fmt.Errorf("invalid region: negative start time")
	}
	if duration <= 0 {
		return fmt.Errorf("invalid region: non-positive duration")
	}

	seekReaderBeforeRegion(reader, start, log)

	filterGraph, bufferSrcCtx, bufferSinkCtx, err := setupFilterGraph(reader.DecoderContext(), filterSpec)
	if err != nil {
		return fmt.Errorf("failed to create %s filter graph: %w", graphName, err)
	}
	defer ffmpeg.AVFilterGraphFree(&filterGraph)

	return runFilterGraph(ctx, reader, bufferSrcCtx, bufferSinkCtx, config)
}
