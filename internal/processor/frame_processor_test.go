package processor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLogAndSkipOptionalMeasurementFrameErrorLogsAndContinues(t *testing.T) {
	t.Parallel()

	var logs []string
	log := debugLogger(func(format string, args ...any) {
		logs = append(logs, formatLog(format, args...))
	})

	err := errors.New("injected frame error")
	policy := logAndSkipOptionalMeasurementFrameError(log, "speech band push")

	if got := policy(err); got != nil {
		t.Fatalf("policy returned %v, want nil", got)
	}
	if len(logs) != 1 {
		t.Fatalf("logs = %d, want 1", len(logs))
	}
	if !strings.Contains(logs[0], "speech band push frame error skipped") {
		t.Fatalf("log = %q, want named measurement", logs[0])
	}
	if !strings.Contains(logs[0], err.Error()) {
		t.Fatalf("log = %q, want error detail", logs[0])
	}
}

func TestDeferLoudnormFrameErrorToStatsFileContinues(t *testing.T) {
	t.Parallel()

	err := errors.New("injected loudnorm frame error")
	if got := deferLoudnormFrameErrorToStatsFile(err); got != nil {
		t.Fatalf("policy returned %v, want nil", got)
	}
}

func formatLog(format string, args ...any) string {
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf(format, args...), "\t", " "))
}
