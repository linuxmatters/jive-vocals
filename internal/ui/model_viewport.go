package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// processingFooterHeight is the row count View reserves below the viewport for
// the scroll hint footer. It stays reserved when the hint is blank, so the file
// boxes do not reflow when overflow changes.
const processingFooterHeight = 1

// scrollbarWidth is the column reserved at the right edge of the viewport.
const scrollbarWidth = 1

func (m *Model) sizeViewport() {
	if m.Width <= 0 || m.Height <= 0 {
		return
	}
	headerHeight := lipgloss.Height(renderProcessingHeader(*m))
	vpHeight := max(m.Height-headerHeight-processingFooterHeight, 1)
	vpWidth := max(m.Width-scrollbarWidth, 1)
	if !m.vpReady {
		m.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
		m.vpReady = true
		return
	}
	m.vp.SetWidth(vpWidth)
	m.vp.SetHeight(vpHeight)
}

func stableFileEntryStatus(status FileStatus) bool {
	switch status {
	case StatusQueued, StatusComplete, StatusError:
		return true
	default:
		return false
	}
}

func (m *Model) ensureFileEntryCaches() {
	if len(m.fileEntryCaches) == len(m.Files) {
		return
	}
	next := make([]fileEntryCache, len(m.Files))
	copy(next, m.fileEntryCaches)
	m.fileEntryCaches = next
}

func (m *Model) ensureActiveFileEntryCaches() {
	if len(m.activeFileEntryCaches) == len(m.Files) {
		return
	}
	next := make([]activeFileEntryCache, len(m.Files))
	copy(next, m.activeFileEntryCaches)
	m.activeFileEntryCaches = next
}

func (m *Model) invalidateFileEntryCache(index int) {
	if index < 0 || index >= len(m.fileEntryCaches) {
		return
	}
	m.fileEntryCaches[index] = fileEntryCache{}
}

func (m *Model) invalidateStableEntryCaches() {
	m.ensureFileEntryCaches()
	for i := range m.fileEntryCaches {
		if stableFileEntryStatus(m.Files[i].Status) {
			m.fileEntryCaches[i] = fileEntryCache{}
		}
	}
}

func (m *Model) refreshFileEntryCaches() bool {
	m.ensureFileEntryCaches()
	changed := false
	for i := range m.Files {
		if !stableFileEntryStatus(m.Files[i].Status) {
			if m.fileEntryCaches[i].valid {
				changed = true
			}
			m.fileEntryCaches[i] = fileEntryCache{}
			continue
		}
		cache := &m.fileEntryCaches[i]
		if cache.valid && cache.status == m.Files[i].Status && cache.termWidth == m.Width {
			continue
		}
		changed = true
		cache.valid = true
		cache.status = m.Files[i].Status
		cache.termWidth = m.Width
		cache.rendered = renderFileEntry(&m.Files[i], m.progress, 0, 0, 0, m.Width)
	}
	return changed
}

func (m *Model) renderActiveEntriesForRefresh() []string {
	activeEntries := make([]string, len(m.Files))
	fitStatusBoxes := statusBoxesFit(m.Width)
	for i := range m.Files {
		if !fileActive(m.Files[i].Status) {
			continue
		}
		easedLevel, easedProgress, easedPeak := m.displayValues(i)
		passBox := renderFileDetails(&m.Files[i], m.progress, easedLevel, easedProgress, easedPeak)
		if fitStatusBoxes {
			refreshStatusBoxCache(&m.Files[i], lipgloss.Height(passBox))
		}
		activeEntries[i] = renderFileEntryWithPassBox(&m.Files[i], m.progress, easedLevel, easedProgress, easedPeak, m.Width, passBox)
	}
	return activeEntries
}

func (m *Model) activeFileEntriesChanged(activeEntries []string) bool {
	m.ensureActiveFileEntryCaches()
	if len(activeEntries) != len(m.activeFileEntryCaches) {
		return true
	}
	for i, rendered := range activeEntries {
		if rendered != m.activeFileEntryCaches[i].rendered {
			return true
		}
	}
	return false
}

func (m *Model) storeActiveFileEntries(activeEntries []string) {
	m.ensureActiveFileEntryCaches()
	for i := range m.activeFileEntryCaches {
		rendered := ""
		if i < len(activeEntries) {
			rendered = activeEntries[i]
		}
		m.activeFileEntryCaches[i].rendered = rendered
	}
}

// refreshViewportContent writes the file queue into the persistent viewport.
// It runs in Update, because View has a value receiver.
func (m *Model) refreshViewportContent() {
	m.refreshViewportContentIfChanged(false)
}

func (m *Model) refreshViewportContentIfChanged(skipUnchanged bool) bool {
	if !m.vpReady {
		return false
	}
	activeEntries := m.renderActiveEntriesForRefresh()
	stableChanged := m.refreshFileEntryCaches()
	activeChanged := m.activeFileEntriesChanged(activeEntries)
	if skipUnchanged && !stableChanged && !activeChanged {
		return false
	}
	atBottom := m.vp.AtBottom()
	m.vp.SetContent(renderFileQueueWithActiveEntries(*m, m.progress, activeEntries))
	m.storeActiveFileEntries(activeEntries)
	if atBottom {
		m.vp.GotoBottom()
	}
	return true
}

func (m Model) displayValues(i int) (level, progressValue, peak float64) {
	level = m.Files[i].CurrentLevel
	progressValue = m.Files[i].Progress
	peak = m.Files[i].PeakLevel
	if i < len(m.meters) {
		level = m.meters[i].pos
		progressValue = m.meters[i].progPos
		peak = m.meters[i].peakPos
	}
	return level, progressValue, peak
}

// renderScrollingView composes the pinned header with the scrollable file queue.
// It stays pure, because viewport content and scroll offset are owned by Update.
func (m Model) renderScrollingView() string {
	if !m.vpReady {
		return renderProcessingView(m)
	}
	overflow := m.vp.TotalLineCount() > m.vp.Height()

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.vp.View(), m.scrollbarStrip(overflow))

	hint := ""
	if overflow {
		hint = renderScrollHint()
	}

	return renderProcessingHeader(m) + "\n" + body + "\n" + hint
}

// scrollbarStrip returns the right-edge scrollbar column for the viewport.
func (m Model) scrollbarStrip(overflow bool) string {
	vpHeight := m.vp.Height()
	if !overflow {
		return strings.TrimRight(strings.Repeat(" \n", vpHeight), "\n")
	}
	return renderScrollbar(vpHeight, m.vp.TotalLineCount(), vpHeight, m.vp.ScrollPercent())
}
