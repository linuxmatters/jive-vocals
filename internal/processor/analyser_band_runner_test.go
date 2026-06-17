package processor

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBandPhaseProgress(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		want      float64
	}{
		{"zero total returns span start", 0, 0, BandPhaseProgressStart},
		{"negative total returns span start", 3, -1, BandPhaseProgressStart},
		{"none done sits at span start", 0, 17, BandPhaseProgressStart},
		{"half done sits at span midpoint", 5, 10, BandPhaseProgressStart + (1.0-BandPhaseProgressStart)*0.5},
		{"all done reaches 1.0", 17, 17, 1.0},
		{"over-count clamps to 1.0", 20, 17, 1.0},
		{"negative completed clamps to span start", -4, 10, BandPhaseProgressStart},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bandPhaseProgress(tc.completed, tc.total)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("bandPhaseProgress(%d, %d) = %v, want %v", tc.completed, tc.total, got, tc.want)
			}
		})
	}
}

func TestBandPhaseProgressMonotonic(t *testing.T) {
	const total = 17
	prev := bandPhaseProgress(0, total)
	for completed := 1; completed <= total; completed++ {
		cur := bandPhaseProgress(completed, total)
		if cur < prev {
			t.Fatalf("progress decreased at completed=%d: %v < %v", completed, cur, prev)
		}
		if cur > 1.0 {
			t.Fatalf("progress exceeded 1.0 at completed=%d: %v", completed, cur)
		}
		prev = cur
	}
	if prev != 1.0 {
		t.Fatalf("final progress = %v, want 1.0", prev)
	}
}

func TestBandProgressTrackerEmitsMappedUpdates(t *testing.T) {
	const total = 4
	var got []float64
	tracker := newBandProgressTracker(func(u ProgressUpdate) {
		if u.PassName != "Analysing frequency bands" {
			t.Errorf("PassName = %q, want %q", u.PassName, "Analysing frequency bands")
		}
		if u.Pass != PassAnalysis {
			t.Errorf("Pass = %v, want PassAnalysis", u.Pass)
		}
		got = append(got, u.Progress)
	}, 12.5, total)

	for range total {
		tracker.report()
	}

	if len(got) != total {
		t.Fatalf("got %d updates, want %d", len(got), total)
	}
	for i := 1; i <= total; i++ {
		want := bandPhaseProgress(i, total)
		if got[i-1] != want {
			t.Fatalf("update %d = %v, want %v", i, got[i-1], want)
		}
	}
	if got[total-1] != 1.0 {
		t.Fatalf("final update = %v, want 1.0", got[total-1])
	}
}

func TestBandProgressTrackerNilSafe(t *testing.T) {
	// nil tracker, nil callback, and zero total must all no-op without panicking.
	var nilTracker *bandProgressTracker
	nilTracker.report()

	newBandProgressTracker(nil, 0, 4).report()
	newBandProgressTracker(func(ProgressUpdate) { t.Fatal("callback should not fire on zero total") }, 0, 0).report()
}

func TestRunBandMeasurementsDeterministicSlots(t *testing.T) {
	// The fan-out must write each band's result to its own fixed index regardless
	// of goroutine scheduling: the values land in band order, not completion order.
	const count = 32
	for trial := range 50 {
		results := make([]int, count)
		runBandMeasurements(context.Background(), count, nil, func(i int) {
			results[i] = i * 7 // each goroutine writes only its own slot
		})
		for i := range count {
			if results[i] != i*7 {
				t.Fatalf("trial %d: results[%d] = %d, want %d", trial, i, results[i], i*7)
			}
		}
	}
}

func TestRunBandMeasurementsReportsEveryBand(t *testing.T) {
	const count = 17
	var reported int64
	runBandMeasurements(context.Background(), count, func() {
		atomic.AddInt64(&reported, 1)
	}, func(int) {})

	if reported != count {
		t.Fatalf("reported = %d, want %d", reported, count)
	}
}

func TestRunBandMeasurementsCancelledStopsCleanly(t *testing.T) {
	// With ctx already cancelled, no band decode runs but every goroutine still
	// reports (so the progress phase drains) and the call returns without hanging.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	const count = 17
	var measured, reported int64
	runBandMeasurements(ctx, count, func() {
		atomic.AddInt64(&reported, 1)
	}, func(int) {
		atomic.AddInt64(&measured, 1)
	})

	if measured != 0 {
		t.Fatalf("measure ran %d times under a cancelled ctx, want 0", measured)
	}
	if reported != count {
		t.Fatalf("reported = %d, want %d (every band must still drain progress)", reported, count)
	}
}

func TestRunBandMeasurementsBoundedConcurrency(t *testing.T) {
	// In-flight measure calls must never exceed the shared semaphore capacity.
	capacity := len(bandMeasureSem)
	if capacity < 1 {
		t.Skip("semaphore capacity below 1")
	}

	var (
		mu       sync.Mutex
		inFlight int
		peak     int
	)
	count := capacity * 4
	gate := make(chan struct{})
	var release sync.Once

	runBandMeasurements(context.Background(), count, nil, func(int) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		reached := inFlight >= capacity
		mu.Unlock()

		if reached {
			release.Do(func() { close(gate) })
		}
		<-gate

		mu.Lock()
		inFlight--
		mu.Unlock()
	})

	if peak > capacity {
		t.Fatalf("peak in-flight = %d, exceeds semaphore capacity %d", peak, capacity)
	}
}

func TestDrainBandProgress(t *testing.T) {
	var n int64
	drainBandProgress(func() { atomic.AddInt64(&n, 1) }, 5)
	if n != 5 {
		t.Fatalf("drainBandProgress fired %d times, want 5", n)
	}
	// nil reporter is a no-op and must not panic.
	drainBandProgress(nil, 5)
}
