package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/linuxmatters/jive-vocals/internal/processor"
	"github.com/linuxmatters/jive-vocals/internal/ui"
)

// inflightFake stands in for the workerPoolDeps processAudio dependency to
// observe pool concurrency without real FFmpeg. It tracks live in-flight
// workers and the high-water mark, records each processed path exactly once,
// then returns an error so runWorkerPool takes the FileCompleteMsg{Error}
// branch (no report/output path needed to drive the pool end-to-end).
//
// Overlap is forced deterministically, not by sleeping: every worker blocks on
// a gate channel that closes (via sync.Once) only when in-flight reaches
// expectedBound, min(jobs, file count). The high-water mark therefore reaches
// the bound on every run, never by scheduling luck, so callers can assert
// maxSeen equals the bound exactly. With jobs == 1 the first worker trips the
// gate at once and execution stays serial. A pool that could never reach the
// bound would wedge on the gate (test timeout) instead of passing spuriously.
type inflightFake struct {
	expectedBound int32
	gate          chan struct{}
	release       sync.Once

	live    atomic.Int32
	maxSeen atomic.Int32

	mu        sync.Mutex
	processed []string
}

// newInflightFake builds an inflightFake whose gate opens once in-flight
// workers reach expectedBound.
func newInflightFake(expectedBound int) *inflightFake {
	return &inflightFake{
		expectedBound: int32(expectedBound), // #nosec G115 -- bound is min(jobs, file count) in tests, far below MaxInt32.
		gate:          make(chan struct{}),
	}
}

func (f *inflightFake) fn(_ context.Context, inputPath string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.ProcessingResult, error) {
	cur := f.live.Add(1)
	for {
		old := f.maxSeen.Load()
		if cur <= old || f.maxSeen.CompareAndSwap(old, cur) {
			break
		}
	}

	if cur >= f.expectedBound {
		f.release.Do(func() { close(f.gate) })
	}
	<-f.gate

	f.mu.Lock()
	f.processed = append(f.processed, inputPath)
	f.mu.Unlock()

	f.live.Add(-1)
	return nil, errors.New("inflightFake: synthetic error to drive pool error branch")
}

// recordingModel is a headless tea.Model that captures pool messages and quits
// on ui.AllCompleteMsg, letting tests observe FileCompleteMsg/AllCompleteMsg
// deterministically without touching the production rendering model.
type recordingModel struct {
	mu           *sync.Mutex
	fileComplete *int
	allComplete  *bool
}

func (m recordingModel) Init() tea.Cmd { return nil }

func (m recordingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ui.FileCompleteMsg:
		m.mu.Lock()
		*m.fileComplete++
		m.mu.Unlock()
	case ui.AllCompleteMsg:
		m.mu.Lock()
		*m.allComplete = true
		m.mu.Unlock()
		return m, tea.Quit
	}
	return m, nil
}

func (m recordingModel) View() tea.View { return tea.NewView("") }

// makeSyntheticFiles returns n synthetic .flac paths under a fresh t.TempDir().
// The paths never exist on disk; the fakes never open them.
func makeSyntheticFiles(t *testing.T, n int) []string {
	t.Helper()

	dir := t.TempDir()
	files := make([]string, n)
	for i := range files {
		files[i] = filepath.Join(dir, "fake-"+string(rune('a'+i))+".flac")
	}
	return files
}

func TestSendWarningCountsDroppedWhenChannelFull(t *testing.T) {
	resetDroppedWarnings()
	t.Cleanup(resetDroppedWarnings)

	reportWarnings := make(chan string, 1)

	sendWarning(reportWarnings, "kept")
	sendWarning(reportWarnings, "dropped")

	if got := droppedWarningCount(); got != 1 {
		t.Fatalf("dropped warning count = %d, want 1", got)
	}

	select {
	case got := <-reportWarnings:
		if got != "kept" {
			t.Fatalf("delivered warning = %q, want %q", got, "kept")
		}
	default:
		t.Fatal("expected one delivered warning")
	}
}

func TestFormatDroppedWarningCount(t *testing.T) {
	tests := []struct {
		name  string
		count uint64
		want  string
	}{
		{
			name:  "single",
			count: 1,
			want:  "1 warning was dropped because the warning channel was full",
		},
		{
			name:  "plural",
			count: 2,
			want:  "2 warnings were dropped because the warning channel was full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDroppedWarningCount(tt.count); got != tt.want {
				t.Fatalf("formatDroppedWarningCount(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func waitForStarts(t *testing.T, started <-chan struct{}, n int) {
	t.Helper()

	for range n {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("saw fewer than %d worker starts", n)
		}
	}
}

func TestRunBoundedPool_StartsFixedWorkerCount(t *testing.T) {
	const n = 200
	const jobs = 3

	files := makeSyntheticFiles(t, n)
	release := make(chan struct{})
	started := make(chan struct{}, n)
	base := processor.DefaultFilterConfig()
	env := poolEnv{ctx: context.Background(), p: nil, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: jobs}

	before := runtime.NumGoroutine()
	done := make(chan struct{})
	go func() {
		runBoundedPool(env, nil, func(_ int, _ string, _ func(string, ...any)) {
			started <- struct{}{}
			<-release
		})
		close(done)
	}()

	waitForStarts(t, started, jobs)
	time.Sleep(50 * time.Millisecond)

	if delta := runtime.NumGoroutine() - before; delta > jobs+20 {
		t.Fatalf("runBoundedPool goroutine delta = %d, want at most %d", delta, jobs+20)
	}

	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runBoundedPool did not return")
	}
}

func TestRunBoundedPool_CancelledQueuedWorkSkipsBody(t *testing.T) {
	const n = 20
	const jobs = 3

	ctx, cancel := context.WithCancel(context.Background())
	files := makeSyntheticFiles(t, n)
	release := make(chan struct{})
	started := make(chan struct{}, n)
	var count atomic.Int32
	base := processor.DefaultFilterConfig()
	env := poolEnv{ctx: ctx, p: nil, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: jobs}

	done := make(chan struct{})
	go func() {
		runBoundedPool(env, nil, func(_ int, _ string, _ func(string, ...any)) {
			count.Add(1)
			started <- struct{}{}
			<-release
		})
		close(done)
	}()

	waitForStarts(t, started, jobs)
	cancel()
	close(release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runBoundedPool did not return after cancellation")
	}

	if got := count.Load(); got != jobs {
		t.Fatalf("body starts after queued cancellation = %d, want %d", got, jobs)
	}
}

// runPoolCapture drives runWorkerPool over files with the given processAudio
// fake under a headless recording tea.Program (no renderer, nil input),
// returning the observed FileCompleteMsg count, whether AllCompleteMsg fired,
// and the pool's returned non-cancellation failure count. runWorkerPool runs
// in a goroutine writing its count to a buffered channel; the count is read
// only after p.Run() returns (AllCompleteMsg is runWorkerPool's last send
// before it returns), so the read is race-free and never wedges. Any report
// warning fails the test: no fake in this file provokes a legitimate one.
func runPoolCapture(t *testing.T, jobs int, files []string, processAudio func(context.Context, string, *processor.BaseFilterConfig, processor.ProgressCallback) (*processor.ProcessingResult, error)) (fileComplete int, allComplete bool, failed int) {
	t.Helper()

	var mu sync.Mutex
	model := recordingModel{mu: &mu, fileComplete: &fileComplete, allComplete: &allComplete}
	p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))

	base := processor.DefaultFilterConfig()
	reportWarnings := make(chan string, len(files))

	env := poolEnv{ctx: context.Background(), p: p, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: jobs}
	failedCh := make(chan int, 1)
	go func() {
		failedCh <- runWorkerPool(env, false, reportWarnings, workerPoolDeps{processAudio: processAudio})
	}()

	if _, err := p.Run(); err != nil {
		t.Fatalf("p.Run() error = %v", err)
	}

	failed = <-failedCh
	close(reportWarnings)
	for warning := range reportWarnings {
		t.Errorf("unexpected report warning: %s", warning)
	}

	mu.Lock()
	defer mu.Unlock()
	return fileComplete, allComplete, failed
}

// runPoolWithFake drives runWorkerPool over n synthetic file paths with an
// inflightFake (every file errors), returning the fake, the observed
// completion counts, and the pool's returned failure count. The fake's gate
// bound is min(jobs, n): the pool cannot hold more than jobs workers in-flight,
// and n files cannot supply more than n.
func runPoolWithFake(t *testing.T, jobs, n int) (*inflightFake, int, bool, int) {
	t.Helper()

	fake := newInflightFake(min(jobs, n))
	fileComplete, allComplete, failed := runPoolCapture(t, jobs, makeSyntheticFiles(t, n), fake.fn)
	return fake, fileComplete, allComplete, failed
}

// TestRunWorkerPool_InFlightBoundedToOne asserts jobs == 1 holds in-flight
// workers to a single concurrent ProcessAudio call. The fake records the
// high-water in-flight mark across 5 files; with jobs == 1 it must never exceed
// 1, proving serial execution under the pool.
func TestRunWorkerPool_InFlightBoundedToOne(t *testing.T) {
	t.Parallel()

	fake, fileComplete, allComplete, failed := runPoolWithFake(t, 1, 5)

	if got := fake.maxSeen.Load(); got != 1 {
		t.Fatalf("max in-flight with jobs=1 = %d, want 1", got)
	}
	if fileComplete != 5 {
		t.Fatalf("FileCompleteMsg count = %d, want 5", fileComplete)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
	// Every file failed with a genuine (non-cancellation) error, so the pool's
	// failure count must equal the file count - the exit-code contract's
	// all-fail case.
	if failed != 5 {
		t.Fatalf("runWorkerPool failure count = %d, want 5 (all files fail)", failed)
	}
}

// TestRunWorkerPool_BoundHonouredForN asserts jobs == 3 holds in-flight
// workers at exactly 3 over 8 files: the worker queue caps the mark at 3, and the
// fake's gate holds every worker until 3 are live, so the mark reaches 3 on
// every run with no dependence on scheduling.
func TestRunWorkerPool_BoundHonouredForN(t *testing.T) {
	t.Parallel()

	fake, fileComplete, allComplete, failed := runPoolWithFake(t, 3, 8)

	if maxSeen := fake.maxSeen.Load(); maxSeen != 3 {
		t.Fatalf("max in-flight with jobs=3 = %d, want exactly 3", maxSeen)
	}
	if fileComplete != 8 {
		t.Fatalf("FileCompleteMsg count = %d, want 8", fileComplete)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
	if failed != 8 {
		t.Fatalf("runWorkerPool failure count = %d, want 8 (all files fail)", failed)
	}
}

// isolationFake stands in for the workerPoolDeps processAudio dependency so
// exactly one designated input path errors while every sibling succeeds. An
// empty failPath matches no input, so every file succeeds (the all-success
// case). Successful calls return a
// ProcessingResult whose OutputPath sits next to the (synthetic) input so the
// pool's report write produces its .md without a report warning. One failing
// input must leave siblings unaffected.
type isolationFake struct {
	failPath string
}

func (f *isolationFake) fn(_ context.Context, inputPath string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.ProcessingResult, error) {
	if inputPath == f.failPath {
		return nil, errors.New("isolationFake: synthetic unreadable input")
	}
	// Derive a sibling output path so the report write produces its .md cleanly.
	outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "-LUFS-16-processed" + filepath.Ext(inputPath)
	if err := os.WriteFile(outputPath, []byte("synthetic"), 0o600); err != nil {
		return nil, err
	}
	return &processor.ProcessingResult{
		OutputPath: outputPath,
		InputLUFS:  -23.0,
		OutputLUFS: -16.0,
	}, nil
}

// isolationModel records per-file completion detail: how many FileCompleteMsg
// arrived, which file indices carried an Error, and whether AllCompleteMsg fired.
// It is a richer sibling of recordingModel for failure-isolation assertions.
type isolationModel struct {
	mu          *sync.Mutex
	completed   *int
	erroredIdx  *map[int]bool
	allComplete *bool
}

func (m isolationModel) Init() tea.Cmd { return nil }

func (m isolationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case ui.FileCompleteMsg:
		m.mu.Lock()
		*m.completed++
		if v.Error != nil {
			(*m.erroredIdx)[v.FileIndex] = true
		}
		m.mu.Unlock()
	case ui.AllCompleteMsg:
		m.mu.Lock()
		*m.allComplete = true
		m.mu.Unlock()
		return m, tea.Quit
	}
	return m, nil
}

func (m isolationModel) View() tea.View { return tea.NewView("") }

// TestRunWorkerPool_FailureIsolation drives the pool over several files where one
// designated input errors and the rest succeed. It asserts every sibling
// completes with no error, the failing file's FileCompleteMsg carries its Error,
// AllCompleteMsg still fires (the partial-failure run completes), and all N files
// report exactly once (no early abort). Seam-based, so no real audio is needed.
func TestRunWorkerPool_FailureIsolation(t *testing.T) {
	t.Parallel()

	const n = 5
	const failIdx = 2

	dir := t.TempDir()
	files := make([]string, n)
	for i := range files {
		files[i] = filepath.Join(dir, "iso-"+string(rune('a'+i))+".flac")
	}

	fake := &isolationFake{failPath: files[failIdx]}

	var mu sync.Mutex
	completed := 0
	erroredIdx := map[int]bool{}
	allComplete := false
	model := isolationModel{mu: &mu, completed: &completed, erroredIdx: &erroredIdx, allComplete: &allComplete}
	p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))

	base := processor.DefaultFilterConfig()
	reportWarnings := make(chan string, n)

	env := poolEnv{ctx: context.Background(), p: p, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: 3}
	// Capture runWorkerPool's failure count: the goroutine writes it to a
	// buffered channel, read after p.Run() returns (the pool sends
	// AllCompleteMsg last, so by then the count is final).
	failedCh := make(chan int, 1)
	go func() {
		failedCh <- runWorkerPool(env, false, reportWarnings, workerPoolDeps{processAudio: fake.fn})
	}()

	if _, err := p.Run(); err != nil {
		t.Fatalf("p.Run() error = %v", err)
	}

	failed := <-failedCh
	close(reportWarnings)
	for warning := range reportWarnings {
		t.Errorf("unexpected report warning: %s", warning)
	}

	if failed != 1 {
		t.Fatalf("runWorkerPool failure count = %d, want 1 (only the designated failing file)", failed)
	}

	mu.Lock()
	defer mu.Unlock()

	if completed != n {
		t.Fatalf("FileCompleteMsg count = %d, want %d (every file reports exactly once)", completed, n)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire on a partial-failure run")
	}
	if !erroredIdx[failIdx] {
		t.Fatalf("failing file index %d did not carry an Error in its FileCompleteMsg", failIdx)
	}
	if len(erroredIdx) != 1 {
		t.Fatalf("errored file indices = %v, want only {%d} (siblings must be unaffected)", erroredIdx, failIdx)
	}

	// Each sibling must have produced its output (proof it ran to completion).
	for i, path := range files {
		if i == failIdx {
			continue
		}
		out := strings.TrimSuffix(path, filepath.Ext(path)) + "-LUFS-16-processed" + filepath.Ext(path)
		if _, err := os.Stat(out); err != nil {
			t.Fatalf("sibling %s did not produce output: %v", path, err)
		}
	}
}

// TestRunWorkerPool_SerialParityJobs1 asserts jobs == 1 yields the serial
// outcome: every submitted file is processed exactly once, every file emits a
// FileCompleteMsg, and AllCompleteMsg fires. Parity is proven by the fake's
// per-path record matching the submission set with no duplicates or omissions.
func TestRunWorkerPool_SerialParityJobs1(t *testing.T) {
	t.Parallel()

	const n = 5
	fake, fileComplete, allComplete, failed := runPoolWithFake(t, 1, n)

	if len(fake.processed) != n {
		t.Fatalf("processed %d files, want %d", len(fake.processed), n)
	}
	seen := make(map[string]int, n)
	for _, p := range fake.processed {
		seen[p]++
	}
	for p, count := range seen {
		if count != 1 {
			t.Fatalf("file %s processed %d times, want exactly 1", p, count)
		}
	}
	if len(seen) != n {
		t.Fatalf("distinct files processed = %d, want %d", len(seen), n)
	}
	if fileComplete != n {
		t.Fatalf("FileCompleteMsg count = %d, want %d", fileComplete, n)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
	if failed != n {
		t.Fatalf("runWorkerPool failure count = %d, want %d (all files fail)", failed, n)
	}
}

// TestRunWorkerPool_FailureCountZeroOnSuccess asserts runWorkerPool returns 0
// when every file succeeds. An isolationFake with an empty failPath matches no
// input, so all files take the success branch (output plus .md/.json written
// into the TempDir); only genuine per-file errors may raise the count.
func TestRunWorkerPool_FailureCountZeroOnSuccess(t *testing.T) {
	t.Parallel()

	const n = 4
	files := makeSyntheticFiles(t, n)
	fake := &isolationFake{}
	fileComplete, allComplete, failed := runPoolCapture(t, 2, files, fake.fn)

	if failed != 0 {
		t.Fatalf("runWorkerPool failure count = %d, want 0 (all files succeed)", failed)
	}
	if fileComplete != n {
		t.Fatalf("FileCompleteMsg count = %d, want %d", fileComplete, n)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
}

// cancelledFake stands in for processAudio to model a user-initiated cancel:
// every input returns a context.Canceled-wrapped error, except an optional
// genuinePath which fails with a plain error (a genuine failure that landed
// before the cancel). runWorkerPool must exclude the wrapped cancellations
// from its failure count while still counting the genuine failure.
type cancelledFake struct {
	genuinePath string
}

func (f *cancelledFake) fn(_ context.Context, inputPath string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.ProcessingResult, error) {
	if inputPath == f.genuinePath {
		return nil, errors.New("cancelledFake: genuine failure before the cancel")
	}
	return nil, fmt.Errorf("cancelledFake: worker aborted: %w", context.Canceled)
}

// TestRunWorkerPool_FailureCountZeroWhenOnlyCancelled asserts a run whose only
// per-file errors wrap context.Canceled returns 0: a user quit is not a
// failure, so the exit code stays 0. Cancelled files still emit their
// FileCompleteMsg{Error} and AllCompleteMsg still fires; only the count is
// filtered.
func TestRunWorkerPool_FailureCountZeroWhenOnlyCancelled(t *testing.T) {
	t.Parallel()

	const n = 5
	files := makeSyntheticFiles(t, n)
	fake := &cancelledFake{}
	fileComplete, allComplete, failed := runPoolCapture(t, 3, files, fake.fn)

	if failed != 0 {
		t.Fatalf("runWorkerPool failure count = %d, want 0 (all errors wrap context.Canceled)", failed)
	}
	if fileComplete != n {
		t.Fatalf("FileCompleteMsg count = %d, want %d (cancelled files still report)", fileComplete, n)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
}

// TestRunWorkerPool_FailureCountGenuineFailureBeforeCancel asserts a genuine
// failure that landed before the cancel still counts while the sibling
// cancellations stay excluded: exactly one of the five errors is plain, so the
// count is exactly 1.
func TestRunWorkerPool_FailureCountGenuineFailureBeforeCancel(t *testing.T) {
	t.Parallel()

	const n = 5
	files := makeSyntheticFiles(t, n)
	fake := &cancelledFake{genuinePath: files[1]}
	fileComplete, allComplete, failed := runPoolCapture(t, 3, files, fake.fn)

	if failed != 1 {
		t.Fatalf("runWorkerPool failure count = %d, want 1 (one genuine failure among cancellations)", failed)
	}
	if fileComplete != n {
		t.Fatalf("FileCompleteMsg count = %d, want %d", fileComplete, n)
	}
	if !allComplete {
		t.Fatal("AllCompleteMsg did not fire")
	}
}

// TestLaunchWorkerPool_DoneClosesAfterPoolUnwinds proves main()'s wiring: the
// channel launchWorkerPool returns must stay open while workers run and close
// only after the pool fully unwinds. Were main() not to wait on it, the process
// could exit before workers' deferred temp cleanup ran. The fake gates on a
// release channel so the test observes the not-yet-closed state deterministically
// before letting the worker finish.
func TestLaunchWorkerPool_DoneClosesAfterPoolUnwinds(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once

	deps := workerPoolDeps{processAudio: func(_ context.Context, _ string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.ProcessingResult, error) {
		once.Do(func() { close(started) })
		<-release
		return nil, errors.New("synthetic error to drive pool error branch")
	}}

	model := recordingModel{mu: &sync.Mutex{}, fileComplete: new(int), allComplete: new(bool)}
	p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))
	go func() {
		if _, err := p.Run(); err != nil {
			t.Errorf("p.Run() error = %v", err)
		}
	}()

	dir := t.TempDir()
	files := []string{filepath.Join(dir, "fake.flac")}
	base := processor.DefaultFilterConfig()
	reportWarnings := make(chan string, len(files))

	env := poolEnv{ctx: context.Background(), p: p, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: 1}
	done := launchWorkerPool(env, false, reportWarnings, deps)

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never started")
	}
	select {
	case <-done:
		t.Fatal("done closed while a worker was still in-flight")
	default:
	}

	close(release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("done did not close after the pool unwound")
	}

	p.Quit()
	p.Wait()
}

// TestLaunchWorkerPool_DoneClosesOnPreCancelledContext proves the wait main()
// performs cannot wedge: with an already-cancelled context every worker either
// skips at the queued-work ctx check or runs and returns, so every wg.Done()
// fires and launchWorkerPool's channel closes promptly. The fake returns an
// error so any worker that starts takes the pool's error branch cleanly rather
// than the nil-result success path.
func TestLaunchWorkerPool_DoneClosesOnPreCancelledContext(t *testing.T) {
	t.Parallel()

	deps := workerPoolDeps{processAudio: func(_ context.Context, _ string, _ *processor.BaseFilterConfig, _ processor.ProgressCallback) (*processor.ProcessingResult, error) {
		return nil, errors.New("synthetic error to drive pool error branch")
	}}

	model := recordingModel{mu: &sync.Mutex{}, fileComplete: new(int), allComplete: new(bool)}
	p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))
	go func() {
		if _, err := p.Run(); err != nil {
			t.Errorf("p.Run() error = %v", err)
		}
	}()

	dir := t.TempDir()
	files := []string{filepath.Join(dir, "a.flac"), filepath.Join(dir, "b.flac")}
	base := processor.DefaultFilterConfig()
	reportWarnings := make(chan string, len(files))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	env := poolEnv{ctx: ctx, p: p, files: files, base: base, sharedLog: func(string, ...any) {}, jobs: 1}
	done := launchWorkerPool(env, false, reportWarnings, deps)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("done did not close on pre-cancelled context")
	}

	p.Quit()
	p.Wait()
}
