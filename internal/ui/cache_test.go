package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/linuxmatters/jive-vocals/internal/processor"
)

func TestStableFileEntryCacheInvalidatesOnStateTransitions(t *testing.T) {
	m := NewModel([]string{"voice.wav"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)

	if !m.fileEntryCaches[0].valid {
		t.Fatal("queued entry cache should be valid after viewport refresh")
	}
	if m.fileEntryCaches[0].status != StatusQueued {
		t.Errorf("queued cache status = %v, want StatusQueued", m.fileEntryCaches[0].status)
	}

	m.fileEntryCaches[0].rendered = "QUEUED SENTINEL"
	if got := renderFileEntryWithCache(&m.Files[0], m.fileEntryCache(0), m.progress, 0, 0, 0, m.Width); got != "QUEUED SENTINEL" {
		t.Errorf("stable cache hit rendered %q, want sentinel", ansi.Strip(got))
	}

	updated, _ = m.Update(FileStartMsg{FileIndex: 0})
	m = updated.(Model)
	if m.fileEntryCaches[0].valid {
		t.Fatal("FileStartMsg should invalidate the queued entry cache")
	}

	updated, _ = m.Update(FileCompleteMsg{
		FileIndex: 0,
		CompletionResult: CompletionResult{
			InputLUFS:  -23,
			OutputLUFS: -16,
			OutputPath: "voice-LUFS-16-processed.flac",
		},
	})
	m = updated.(Model)
	if !m.fileEntryCaches[0].valid {
		t.Fatal("completed entry cache should be valid after FileCompleteMsg")
	}
	if m.fileEntryCaches[0].status != StatusComplete {
		t.Errorf("completed cache status = %v, want StatusComplete", m.fileEntryCaches[0].status)
	}
	if strings.Contains(m.fileEntryCaches[0].rendered, "QUEUED SENTINEL") {
		t.Fatal("completed cache reused the queued sentinel")
	}

	m.fileEntryCaches[0].rendered = "STALE WIDTH SENTINEL"
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	m = updated.(Model)
	if !m.fileEntryCaches[0].valid {
		t.Fatal("resize should rebuild a stable entry cache")
	}
	if m.fileEntryCaches[0].termWidth != 90 {
		t.Errorf("cache width = %d, want 90", m.fileEntryCaches[0].termWidth)
	}
	if m.fileEntryCaches[0].rendered == "STALE WIDTH SENTINEL" {
		t.Fatal("resize should not keep stale rendered content")
	}
}

func TestStableFileEntryCacheInvalidatesOnErrorCompletion(t *testing.T) {
	m := NewModel([]string{"voice.wav"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	m.fileEntryCaches[0].rendered = "QUEUED SENTINEL"

	updated, _ = m.Update(FileCompleteMsg{
		FileIndex: 0,
		CompletionResult: CompletionResult{
			Error: errors.New("decode failed"),
		},
	})
	m = updated.(Model)

	if !m.fileEntryCaches[0].valid {
		t.Fatal("error entry cache should be valid after FileCompleteMsg")
	}
	if m.fileEntryCaches[0].status != StatusError {
		t.Errorf("error cache status = %v, want StatusError", m.fileEntryCaches[0].status)
	}
	plain := ansi.Strip(m.fileEntryCaches[0].rendered)
	if !strings.Contains(plain, "decode failed") {
		t.Errorf("error cache missing error text:\n%s", plain)
	}
	if strings.Contains(plain, "QUEUED SENTINEL") {
		t.Fatal("error cache reused the queued sentinel")
	}
}

func TestMeterTickNoOpRefreshSkipsViewportWrite(t *testing.T) {
	m := NewModel([]string{"voice.wav"})

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = updated.(Model)
	updated, _ = m.Update(ProgressMsg{FileIndex: 0, Pass: processor.PassProcessing, Progress: 0})
	m = updated.(Model)

	before := m.activeFileEntryCaches[0].rendered
	if before == "" {
		t.Fatal("active entry cache should be populated before the tick")
	}
	if changed := m.refreshViewportContentIfChanged(true); changed {
		t.Fatal("unchanged active and stable entries should skip viewport refresh")
	}

	updated, cmd := m.Update(meterTickMsg{})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("meterTickMsg should schedule another tick while a file is active")
	}
	if m.activeFileEntryCaches[0].rendered != before {
		t.Fatal("no-op meter tick should leave the cached active entry unchanged")
	}
}

func TestActiveEntryRenderingReusesPreRenderedPassBox(t *testing.T) {
	file := FileProgress{
		InputPath:    "voice.wav",
		Status:       StatusProcessing,
		CurrentPass:  processor.PassProcessing,
		PassName:     "Processing Audio",
		CurrentLevel: meterFloorDB,
	}

	got := renderFileEntryWithPassBox(&file, newProgressModel(), 0, 0, 0, 60, "PRE-RENDERED PASS BOX")
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "PRE-RENDERED PASS BOX") {
		t.Errorf("active entry should include the supplied Pass box:\n%s", plain)
	}
	if strings.Contains(plain, "Pass 2/4") || strings.Contains(plain, "Processing Audio") {
		t.Errorf("active entry rendered a second Pass box instead of reusing the supplied one:\n%s", plain)
	}
}

func TestFileQueueUsesPreRenderedActiveEntry(t *testing.T) {
	m := NewModel([]string{"voice.wav"})
	m.Width = 120
	m.Files[0].Status = StatusProcessing
	m.Files[0].CurrentPass = processor.PassProcessing

	got := renderFileQueueWithActiveEntries(m, m.progress, []string{"PRE-RENDERED ACTIVE ENTRY"})
	if got != "PRE-RENDERED ACTIVE ENTRY\n" {
		t.Errorf("file queue should reuse the pre-rendered active entry, got %q", ansi.Strip(got))
	}
}
