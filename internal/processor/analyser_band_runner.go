// Package processor handles audio analysis and processing
package processor

import (
	stdcontext "context"
	"runtime"
	"sync"
	"sync/atomic"
)

// bandMeasureSem bounds the concurrent band decodes across ALL files and both
// band-measurement functions (measureSpeechBands, measureNoiseBands). It is a
// pure concurrency limiter (a buffered-channel semaphore, the same pattern as
// runWorkerPool's sem in cmd/jive-vocals/pool.go), initialised once at package
// load and only read after init, so the single large file fans its bands across
// every core while a multi-file batch never oversubscribes FFmpeg beyond the
// core count.
var bandMeasureSem = make(chan struct{}, runtime.NumCPU())

// bandProgressReporter reports the completion of one band decode. It is wired by
// the caller to drive the post-loop 0.95..1.0 progress span: each call increments
// a shared atomic counter and emits a ProgressUpdate. nil means no reporting (the
// adaptive/analysis-only paths that pass a nil progress callback).
type bandProgressReporter func()

// bandProgressTracker maps a stream of band-decode completions onto the
// post-loop 0.95..1.0 progress span. completed counts the bands finished so far
// (incremented atomically, so concurrent goroutines report safely) and total is
// the band count across the whole post-loop phase (speech + noise). Each report()
// call emits one ProgressUpdate. A nil callback or non-positive total makes
// report() a no-op.
type bandProgressTracker struct {
	callback  ProgressCallback
	duration  float64
	total     int
	completed atomic.Int64
}

// newBandProgressTracker builds a tracker for the post-loop band phase. total is
// the combined count of speech and noise band decodes the phase will run.
func newBandProgressTracker(callback ProgressCallback, duration float64, total int) *bandProgressTracker {
	return &bandProgressTracker{callback: callback, duration: duration, total: total}
}

// report records one completed band decode and emits a ProgressUpdate mapped onto
// the 0.95..1.0 span. Safe to call from multiple goroutines.
func (t *bandProgressTracker) report() {
	if t == nil || t.callback == nil || t.total <= 0 {
		return
	}
	done := t.completed.Add(1)
	t.callback(ProgressUpdate{ // #nosec G101 -- progress update, not a credential
		Pass:     PassAnalysis,
		PassName: "Analysing frequency bands",
		Progress: bandPhaseProgress(int(done), t.total),
		Duration: t.duration,
	})
}

// BandPhaseProgressStart is the progress value the main analysis decode loop is
// capped at; the post-loop band phase drives from here to 1.0. Exported so the UI
// can un-scale the capped Pass-1 progress back to true decode throughput for the
// realtime-speed badge.
const BandPhaseProgressStart = 0.95

// bandPhaseProgress maps completed-of-total band decodes onto the
// [BandPhaseProgressStart, 1.0] span. It is monotonic in completed, clamps the
// result to 1.0, and returns the span start when total is non-positive (no work
// to scale over). Pure function for unit testing.
func bandPhaseProgress(completed, total int) float64 {
	if total <= 0 {
		return BandPhaseProgressStart
	}
	if completed < 0 {
		completed = 0
	}
	progress := BandPhaseProgressStart + (1.0-BandPhaseProgressStart)*(float64(completed)/float64(total))
	if progress > 1.0 {
		progress = 1.0
	}
	return progress
}

// runBandMeasurements fans the band decodes out as bounded goroutines and blocks
// until they finish. Each band index i runs measure(i) in its own goroutine,
// acquiring a bandMeasureSem slot first so concurrency stays capped at NumCPU
// across the whole process. measure must write only to its own result slot (a
// distinct index or field), so the goroutines share no mutable state and need no
// lock. report (when non-nil) fires once per completed band to advance progress.
//
// Context cancellation: a goroutine that has not yet acquired a slot when ctx is
// done skips its decode cleanly (releasing nothing); an in-flight decode unwinds
// because ctx is threaded into runFilterGraph, which checks ctx.Err() each frame.
// Either way the goroutine returns and the WaitGroup drains, so the phase never
// blocks on a slow or cancelled band. Per-band failures stay non-fatal inside
// measure (the caller keeps its existing zero-RMS / white-noise fallback).
func runBandMeasurements(ctx stdcontext.Context, count int, report bandProgressReporter, measure func(i int)) {
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if report != nil {
				defer report()
			}

			// Skip the decode when ctx is already cancelled so a not-yet-started
			// band unwinds cleanly. The explicit check first makes cancellation
			// deterministic: a bare select races the two ready branches.
			if ctx.Err() != nil {
				return
			}

			// Acquire a slot, or bail if ctx is cancelled while we wait. Release
			// only on the branch that took a slot.
			select {
			case bandMeasureSem <- struct{}{}:
				defer func() { <-bandMeasureSem }()
			case <-ctx.Done():
				return
			}

			measure(i)
		}(i)
	}
	wg.Wait()
}
