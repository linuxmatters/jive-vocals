package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/linuxmatters/jivetalking/internal/processor"
)

func TestProgressMsgIndexRouting(t *testing.T) {
	m := NewModel([]string{"a.wav", "b.wav"})

	updated, _ := m.Update(ProgressMsg{FileIndex: 0, Pass: processor.PassAnalysis, Progress: 0.25})
	m = updated.(Model)
	updated, _ = m.Update(ProgressMsg{FileIndex: 1, Pass: processor.PassProcessing, Progress: 0.75})
	m = updated.(Model)

	if m.Files[0].Progress != 0.25 {
		t.Errorf("Files[0].Progress = %v, want 0.25", m.Files[0].Progress)
	}
	if m.Files[0].CurrentPass != processor.PassAnalysis {
		t.Errorf("Files[0].CurrentPass = %v, want PassAnalysis", m.Files[0].CurrentPass)
	}
	if m.Files[0].Status != StatusAnalyzing {
		t.Errorf("Files[0].Status = %v, want StatusAnalyzing", m.Files[0].Status)
	}

	if m.Files[1].Progress != 0.75 {
		t.Errorf("Files[1].Progress = %v, want 0.75", m.Files[1].Progress)
	}
	if m.Files[1].CurrentPass != processor.PassProcessing {
		t.Errorf("Files[1].CurrentPass = %v, want PassProcessing", m.Files[1].CurrentPass)
	}
	if m.Files[1].Status != StatusProcessing {
		t.Errorf("Files[1].Status = %v, want StatusProcessing", m.Files[1].Status)
	}
}

func TestFileCompleteMsgIndexRouting(t *testing.T) {
	m := NewModel([]string{"a.wav", "b.wav"})

	// Put index 0 mid-process.
	updated, _ := m.Update(ProgressMsg{FileIndex: 0, Pass: processor.PassProcessing, Progress: 0.5})
	m = updated.(Model)
	before := m.Files[0]

	updated, _ = m.Update(FileCompleteMsg{FileIndex: 1, InputLUFS: -23, OutputLUFS: -16, OutputPath: "b-out.wav"})
	m = updated.(Model)

	if m.Files[1].Status != StatusComplete {
		t.Errorf("Files[1].Status = %v, want StatusComplete", m.Files[1].Status)
	}
	if m.Files[1].OutputPath != "b-out.wav" {
		t.Errorf("Files[1].OutputPath = %q, want b-out.wav", m.Files[1].OutputPath)
	}
	if m.Files[0] != before {
		t.Errorf("Files[0] changed: got %+v, want %+v", m.Files[0], before)
	}
}

func TestUpdateOutOfRangeSafety(t *testing.T) {
	m := NewModel([]string{"a.wav", "b.wav"})
	want := append([]FileProgress(nil), m.Files...)

	indices := []int{-1, len(m.Files)}
	for _, idx := range indices {
		updated, _ := m.Update(ProgressMsg{FileIndex: idx, Pass: processor.PassProcessing, Progress: 0.9})
		m = updated.(Model)
		updated, _ = m.Update(FileCompleteMsg{FileIndex: idx, OutputPath: "x"})
		m = updated.(Model)
	}

	for i := range want {
		if m.Files[i] != want[i] {
			t.Errorf("Files[%d] changed after out-of-range messages: got %+v, want %+v", i, m.Files[i], want[i])
		}
	}
	if m.CompletedFiles != 0 || m.FailedFiles != 0 {
		t.Errorf("counts changed: completed=%d failed=%d, want 0/0", m.CompletedFiles, m.FailedFiles)
	}
}

func TestRenderOverallProgressFooter(t *testing.T) {
	m := NewModel([]string{"a.wav", "b.wav", "c.wav"})

	// One complete, one failed, one in progress.
	updated, _ := m.Update(FileCompleteMsg{FileIndex: 0, OutputPath: "a-out.wav"})
	m = updated.(Model)
	updated, _ = m.Update(FileCompleteMsg{FileIndex: 1, Error: errors.New("boom")})
	m = updated.(Model)
	updated, _ = m.Update(ProgressMsg{FileIndex: 2, Pass: processor.PassProcessing, Progress: 0.4})
	m = updated.(Model)

	footer := renderOverallProgress(m)

	if !strings.Contains(footer, "3") {
		t.Errorf("footer missing total count 3: %q", footer)
	}
	if !strings.Contains(footer, "1 complete") {
		t.Errorf("footer missing complete count: %q", footer)
	}
	if !strings.Contains(footer, "1 failed") {
		t.Errorf("footer missing failed count: %q", footer)
	}
	if strings.Contains(strings.ToLower(footer), "file 3 of") || strings.Contains(strings.ToLower(footer), "of 3") {
		t.Errorf("footer must not contain a 'file N of M' cursor: %q", footer)
	}
}
