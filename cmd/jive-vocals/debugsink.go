package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// debugSink is the shared, serialised writer for the debug log file. It holds
// the destination file and a mutex so concurrent writers each emit whole lines
// without interleaving. It carries no per-file prefix; per-file attribution is
// the wrapper's job (see withFilePrefix).
type debugSink struct {
	mu       sync.Mutex
	file     *os.File
	disabled bool
}

// newDebugSink builds a sink over file, which may be nil. The disabled/no-op
// case is decided once here (nil file) rather than re-checked per write under a
// race.
func newDebugSink(file *os.File) *debugSink {
	return &debugSink{file: file, disabled: file == nil}
}

// Logf formats one full line and writes it atomically against other writers.
// The signature matches BaseFilterConfig.SetLogger so the sink can back the
// shared log closure. The line format mirrors the original closure
// (fmt.Fprintf(debugLog, format+"\n", args...)) for byte-identity.
func (s *debugSink) Logf(format string, args ...any) {
	if s.disabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.file, format+"\n", args...)
}

// withFilePrefix wraps base and returns a logger that prepends a per-file
// marker (the basename of path) to the format string before delegating, so the
// args still bind to the original verbs. It owns no file and no lock; it is the
// per-worker, prefix-only seam.
func withFilePrefix(path string, base func(format string, args ...any)) func(format string, args ...any) {
	marker := "[" + filepath.Base(path) + "] "
	return func(format string, args ...any) {
		base(marker+format, args...)
	}
}
