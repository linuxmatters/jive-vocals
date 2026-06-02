package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// TestDebugSinkConcurrentWritesRace fans many goroutines through one shared
// sink. Run under `CGO_ENABLED=1 go test -race` to detect data races on the
// shared file and mutex.
func TestDebugSinkConcurrentWritesRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "race.log")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer file.Close()

	sink := newDebugSink(file)

	const (
		workers        = 16
		linesPerWorker = 500
	)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			for i := range linesPerWorker {
				sink.Logf("worker %d line %d", id, i)
			}
		}(w)
	}
	wg.Wait()

	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	lines := readLines(t, path)
	if got, want := len(lines), workers*linesPerWorker; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
}

// TestDebugSinkPrefixAttribution drives MANY per-file withFilePrefix wrappers
// over ONE shared sink with concurrent writes, then asserts every output line
// is whole (no mid-line interleaving) and carries exactly one file marker that
// matches the wrapper that produced it.
func TestDebugSinkPrefixAttribution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attrib.log")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer file.Close()

	sink := newDebugSink(file)

	const (
		wrappers       = 12
		linesPerWriter = 400
	)

	// One distinct marker per wrapper; basename drives the marker text.
	names := make([]string, wrappers)
	markers := make([]string, wrappers)
	for w := range wrappers {
		names[w] = fmt.Sprintf("episode-%02d.wav", w)
		markers[w] = "[" + names[w] + "] "
	}

	var wg sync.WaitGroup
	wg.Add(wrappers)
	for w := range wrappers {
		go func(id int) {
			defer wg.Done()
			logf := withFilePrefix(names[id], sink.Logf)
			for i := range linesPerWriter {
				logf("payload writer %d seq %d", id, i)
			}
		}(w)
	}
	wg.Wait()

	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	lines := readLines(t, path)
	if got, want := len(lines), wrappers*linesPerWriter; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}

	// A whole, well-formed line is exactly one marker followed by a payload
	// naming the same writer id as the marker. Any interleaving breaks this.
	markerRe := regexp.MustCompile(`\[episode-\d{2}\.wav\] `)
	lineRe := regexp.MustCompile(`^\[episode-(\d{2})\.wav\] payload writer (\d+) seq \d+$`)

	seen := make([]int, wrappers)
	for n, line := range lines {
		// Exactly one marker per line (no mid-line interleaving).
		if count := len(markerRe.FindAllString(line, -1)); count != 1 {
			t.Fatalf("line %d has %d markers, want 1: %q", n, count, line)
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("line %d malformed: %q", n, line)
		}
		// The marker's writer id must match the payload's writer id.
		if m[1] != fmt.Sprintf("%02d", mustAtoi(t, m[2])) {
			t.Fatalf("line %d marker/payload writer mismatch: %q", n, line)
		}
		id := mustAtoi(t, m[2])
		if id < 0 || id >= wrappers {
			t.Fatalf("line %d writer id %d out of range", n, id)
		}
		// The line carries the exact marker string for that writer.
		if !strings.HasPrefix(line, markers[id]) {
			t.Fatalf("line %d prefix %q, want marker %q", n, line, markers[id])
		}
		seen[id]++
	}

	for id, count := range seen {
		if count != linesPerWriter {
			t.Fatalf("writer %d produced %d lines, want %d", id, count, linesPerWriter)
		}
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return lines
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			t.Fatalf("non-numeric value %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n
}
