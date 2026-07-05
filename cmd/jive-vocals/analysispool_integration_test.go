//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linuxmatters/jive-vocals/internal/processor"
)

// TestRunAnalysisPool_CancellationAbortsPromptly cancels while all fixed workers
// are busy. It checks that no more than jobs analyses start and that the pool
// returns once the in-flight workers release.
func TestRunAnalysisPool_CancellationAbortsPromptly(t *testing.T) {
	const n = 6
	const jobs = 2 // fewer workers than files, so remaining files stay queued

	var started atomic.Int32
	entered := make(chan struct{}, n)
	gate := make(chan struct{})

	analyse := func(ctx context.Context, _ string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.AnalysisResult, error) {
		started.Add(1)
		entered <- struct{}{}
		// Keep active workers busy while the context is cancelled, so no worker
		// can take another queued file before the assertion.
		<-gate
		return nil, ctx.Err()
	}

	files := makeAnalysisFiles(t, n)
	slots := make([]analysisSlot, len(files))
	base := processor.DefaultFilterConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env := poolEnv{ctx: ctx, p: nil, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: jobs}
	done := make(chan struct{})
	go func() {
		runAnalysisPool(env, slots, poolDepsWithAnalyse(t, analyse))
		close(done)
	}()

	// Wait until every fixed worker has entered the fake. No other file can
	// start while the gate is closed.
	for range jobs {
		<-entered
	}

	cancel()

	// Confirm no further entry appears while the gate is closed. A short settle
	// gives any wrongly admitted worker time to bump the counter.
	if extra := waitForExtraEntry(entered, 100*time.Millisecond); extra {
		t.Fatalf("a queued file entered the fake after cancel (started=%d)", started.Load())
	}
	if got := started.Load(); got != jobs {
		t.Fatalf("started = %d, want %d (queued files must not start after cancel)", got, jobs)
	}

	// Release the gated in-flight workers so they return and the pool unwinds.
	close(gate)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runAnalysisPool did not return promptly after cancel (every wg.Done() must fire so wg.Wait() unwinds)")
	}
}

// waitForExtraEntry reports whether another worker signals the entered channel
// within d, used to detect a queued file running the fake after cancel.
func waitForExtraEntry(entered <-chan struct{}, d time.Duration) bool {
	select {
	case <-entered:
		return true
	case <-time.After(d):
		return false
	}
}

// TestRunAnalysisPool_ConcurrentRaceClean drives runAnalysisPool with jobs >= 2
// over two distinct REAL fixture copies through the REAL
// processor.AnalyseOnlyDetailed and the REAL openAudioMetadata opener. Unlike the
// seam-based unit tests, it runs actual concurrent FFmpeg analysis so `-race`
// observes the genuine concurrent paths: the shared debugSink-backed logger
// (whole-line atomic writes), the per-worker CloneForWorker config clones, and the
// pre-allocated results/metas/errs slots each worker writes only its own slot of.
// It drives with p == nil (no tea.Program, no TTY) to keep the race test focused
// on the pool internals. After the run every slot must be populated
// (results[i] != nil, errs[i] == nil, metas[i] != nil).
func TestRunAnalysisPool_ConcurrentRaceClean(t *testing.T) {
	src := findPoolTestAudio(t)
	if src == "" {
		t.Skip("no audio file found under testdata/; drop a .flac (e.g. testdata/fixture-5m.flac) to run this test")
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Skipf("testdata audio not found: %s", src)
	}

	ext := filepath.Ext(src)
	dir := t.TempDir()
	files := []string{
		copyFixtureTo(t, src, dir, "analysis-a"+ext),
		copyFixtureTo(t, src, dir, "analysis-b"+ext),
	}

	// Shared debugSink backs the shared logger; every worker writes whole lines
	// through it concurrently, exercising the sink's serialisation under -race.
	sinkFile, err := os.CreateTemp(dir, "debug-*.log")
	if err != nil {
		t.Fatalf("create debug sink file: %v", err)
	}
	t.Cleanup(func() { sinkFile.Close() })
	sink := newDebugSink(sinkFile)
	sharedLog := sink.Logf

	base := processor.DefaultFilterConfig()
	base.SetLogger(sharedLog)

	slots := make([]analysisSlot, len(files))

	// jobs == len(files) so both real analyses run concurrently, forcing
	// concurrent sink writes, CloneForWorker calls, and slot writes. p == nil so
	// no real terminal is needed. The deps inject the REAL analyse path and the
	// production openAudioMetadata opener used by defaultAnalysisPoolDeps.
	env := poolEnv{ctx: context.Background(), p: nil, files: files, base: base, sharedLog: sharedLog, jobs: len(files)}
	runAnalysisPool(env, slots, analysisPoolDeps{analyse: processor.AnalyseOnlyDetailed, openMetadata: openAudioMetadata})

	for i := range files {
		if slots[i].err != nil {
			t.Fatalf("errs[%d] = %v, want nil", i, slots[i].err)
		}
		if slots[i].result == nil {
			t.Fatalf("results[%d] = nil, want populated", i)
		}
		if slots[i].meta == nil {
			t.Fatalf("metas[%d] = nil, want populated", i)
		}
	}
}
