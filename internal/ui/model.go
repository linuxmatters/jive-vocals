// Package ui provides the Bubbletea terminal user interface for jive-vocals
package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/linuxmatters/jive-vocals/internal/cli"
	"github.com/linuxmatters/jive-vocals/internal/processor"
)

// meterWidth is the cell width of the audio level meter. The progress bar caps
// its rendered total at this width so its right edge aligns with the meter.
const meterWidth = 40

// defaultProgressWidth is the fallback bar width before a WindowSizeMsg arrives.
const defaultProgressWidth = meterWidth

// minProgressWidth floors the bar so it stays usable on narrow terminals.
const minProgressWidth = 10

// maxProgressWidth caps the bar's rendered total (fill + percentage label) so it
// aligns with the meterWidth-cell audio level meter rather than sprawling.
const maxProgressWidth = meterWidth

// processingBarOverhead is the horizontal chrome around the processing-view
// progress bar: RoundedBorder (2 cols) + Padding(0,1) (2 cols) plus a 2-col
// safety margin so the box and its percentage label never reach the edge.
const processingBarOverhead = 6

// analysisBarOverhead is the horizontal chrome around the analysis-view progress
// bar: a 3-col leading indent, the " [MM:SS]" elapsed suffix (~8 cols), plus a
// 2-col safety margin.
const analysisBarOverhead = 13

// progressWidthFor clamps the bar width derived from a terminal width and its
// surrounding chrome into the supported range.
func progressWidthFor(termWidth, overhead int) int {
	return max(minProgressWidth, min(termWidth-overhead, maxProgressWidth))
}

// handleCommonMsg processes the messages both Update methods treat identically:
// the quit keys ("q"/"ctrl+c"), WindowSizeMsg (store dimensions + clamp the
// progress width via progressWidthFor with the caller's chrome overhead), and
// AllCompleteMsg (mark done + quit). It mutates the shared fields through
// pointers and returns handled=true when it owned the message, so each Update
// can return early; per-model messages fall through with handled=false to the
// model's own switch. Kept to the genuinely-identical block: the models share no
// struct shape, only these four fields.
func handleCommonMsg(msg tea.Msg, width, height *int, done *bool, prog *progress.Model, overhead int) (handled bool, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return true, tea.Quit
		}

	case tea.WindowSizeMsg:
		*width = msg.Width
		*height = msg.Height
		if msg.Width > 0 {
			prog.SetWidth(progressWidthFor(msg.Width, overhead))
		}
		return true, nil

	case AllCompleteMsg:
		*done = true
		return true, tea.Quit
	}

	return false, nil
}

// meterFPS is the spring step rate for the eased audio level meter (~60fps).
const meterFPS = 60

// meterFloorDB is the audio level meter's silence floor in dB: the bottom of the
// rendered dB range, the meter spring start position, and the initial PeakLevel.
// Shared so the meter display (views.go) and these start values never drift apart.
const meterFloorDB = -70.0

// meterTickMsg drives the spring step for the eased audio level meter. The loop
// is self-scheduling while any file is active and stops once m.Done is set.
type meterTickMsg struct{}

// meterState holds the harmonica spring smoothing state for a single file's
// audio level meter and its progress bar. It is parallel to Model.Files (keyed
// by file index) so the routed FileProgress data contract stays free of
// presentation-only state.
type meterState struct {
	pos     float64 // eased meter display position in dB
	vel     float64 // meter spring velocity
	progPos float64 // eased progress display position (0.0-1.0)
	progVel float64 // progress spring velocity
	peakPos float64 // eased peak-hold marker position in dB
	peakVel float64 // peak spring velocity
}

// newProgressModel builds the shared gradient progress bar used by both the
// processing and analysis models.
func newProgressModel() progress.Model {
	// Sky-blue to indigo gradient. WithScaled blends the two stops across the
	// filled portion only, so the gradient is always visible regardless of fill.
	// The CIELAB path between these endpoints stays vivid (no muddy midpoint).
	p := progress.New(
		progress.WithColors(cli.ColorSkyBlue, cli.ColorIndigo),
		progress.WithScaled(true),
	)
	p.EmptyColor = cli.ColorFill
	p.SetWidth(defaultProgressWidth)
	return p
}

// FileStatus represents the processing state of a single file
type FileStatus int

const (
	StatusQueued FileStatus = iota
	StatusAnalysing
	StatusProcessing
	StatusNormalising
	StatusComplete
	StatusError
)

// FileProgress tracks progress for a single audio file
type FileProgress struct {
	InputPath string
	Status    FileStatus

	// Phase tracking
	CurrentPass processor.PassNumber
	PassName    string

	// Progress tracking (percentage-based)
	Progress    float64 // 0.0 to 1.0
	StartTime   time.Time
	ElapsedTime time.Duration

	// Duration is the total audio length in seconds (constant per file; the
	// first non-zero value is kept). Drives the realtime-speed badge.
	Duration float64

	// Analysis results (from Pass 1)
	Measurements *processor.AudioMeasurements

	// Summary is the filter-chain status view-model behind the two side boxes. It
	// is populated from AdaptedSummaryMsg (Pass-2 start, then completion) and read
	// only by the renderer; the meter tick never touches it.
	Summary AdaptedSummary

	// statusBoxCache memoises the rendered Filter Chain + Analysis panel strings so
	// the side boxes are not rebuilt byte-for-byte on every 60 fps frame. The panels
	// depend only on (Summary, Pass-box height): every other input is a compile-time
	// constant (box widths, glyphs, units) or a palette colour fixed at startup. The
	// cache is populated by Update before the viewport content is refreshed and
	// reused while the key matches. Render helpers read it but never mutate it.
	statusBoxCache statusBoxCache

	// fileDetailsTitleCache is retained for test fixtures and future Update-owned
	// caching. Render helpers do not mutate it.
	fileDetailsTitleCache overlayTitleCache

	// Processing statistics
	CurrentLevel float64 // Current audio level in dB
	HasLevel     bool
	PeakLevel    float64 // Peak level seen so far

	// Completion results, copied wholesale from FileCompleteMsg.CompletionResult.
	// Carries the done-box numbers (InputLUFS/OutputLUFS, noise floors and their
	// Have* gates, OutputTP/OutputLRA, the two quality scores, ProcessingTime) and
	// the per-file Error.
	CompletionResult
}

// fileEntryCache holds a rendered, stable queue entry. Active entries are never
// cached here because their meters, clocks, and side panels can change every tick.
type fileEntryCache struct {
	valid     bool
	status    FileStatus
	termWidth int
	rendered  string
}

// activeFileEntryCache holds the previous rendered output for active entries.
// Meter ticks compare full rendered strings so every visible field participates
// in the no-op refresh check.
type activeFileEntryCache struct {
	rendered string
}

// statusBoxCache holds the memoised side-panel render keyed on the inputs the
// render functions read. valid guards against a zero-value false positive (a
// genuine key of {Summary{}, 0}). chain and analysis are the rendered box
// strings; joinHeight is the Pass-box height they were rendered against.
type statusBoxCache struct {
	valid      bool
	summary    AdaptedSummary
	joinHeight int
	chain      string
	analysis   string
}

// overlayTitleCache holds a memoised overlayBorderTitle result keyed on the inputs
// that vary it: the title text and the rendered box's top-border width. valid guards
// the zero-value (a genuine key of {"", 0}). line is the spliced first border line.
// The colour is not part of the key: this cache serves a single-colour call site
// (cli.ColorSkyBlue), so colour never varies.
type overlayTitleCache struct {
	valid bool
	title string
	width int
	line  string
}

// Model is the Bubbletea model for the processing UI
type Model struct {
	// File queue
	Files          []FileProgress
	TotalFiles     int
	CompletedFiles int
	FailedFiles    int

	// Global state
	StartTime time.Time
	Done      bool

	// Progress bar (owned by Update; rendered via ViewAs)
	progress progress.Model

	// fileEntryCaches stores rendered entries for queued, completed, and errored
	// files. Update owns population and invalidation; View only reads cached strings.
	fileEntryCaches []fileEntryCache

	// activeFileEntryCaches stores the previous rendered active entries, keyed by
	// file index. It is used only by Update to skip viewport writes on no-op meter
	// ticks.
	activeFileEntryCaches []activeFileEntryCache

	// vp scrolls the file queue inside the alt-screen processing view, which has
	// no native scrollback. The title + overall-progress header stays pinned
	// above it; vp holds only the file list. Built on the first WindowSizeMsg
	// (the zero Model has no usable viewport: New() sets MouseWheelEnabled and the
	// initialised flag), then resized on each subsequent resize. vpReady gates the
	// pre-size frames so View() renders the unscrolled fallback until then.
	vp      viewport.Model
	vpReady bool

	// Eased audio level meter and progress bar state, parallel to Files (keyed
	// by file index). Owned and mutated only in Update; never touched by pool
	// workers.
	meters         []meterState
	spring         harmonica.Spring // eases the audio level meter
	progressSpring harmonica.Spring // eases the progress bar fill
	peakSpring     harmonica.Spring // eases the peak-hold marker

	// Terminal dimensions
	Width  int
	Height int
}

// NewModel creates a new UI model with the given input files
func NewModel(inputFiles []string) Model {
	files := make([]FileProgress, len(inputFiles))
	meters := make([]meterState, len(inputFiles))
	for i, path := range inputFiles {
		files[i] = FileProgress{
			InputPath: path,
			Status:    StatusQueued,
			PeakLevel: meterFloorDB, // Initialize to silence threshold
		}
		meters[i] = meterState{pos: meterFloorDB, peakPos: meterFloorDB}
	}

	return Model{
		Files:                 files,
		TotalFiles:            len(inputFiles),
		StartTime:             time.Now(),
		progress:              newProgressModel(),
		fileEntryCaches:       make([]fileEntryCache, len(inputFiles)),
		activeFileEntryCaches: make([]activeFileEntryCache, len(inputFiles)),
		meters:                meters,
		// Gentle under-damped spring: eases toward target without hard snapping.
		spring: harmonica.NewSpring(harmonica.FPS(meterFPS), 6.0, 0.7),
		// Snappier critically-damped spring for the bar fill: smooth motion that
		// tracks progress promptly without overshoot.
		progressSpring: harmonica.NewSpring(harmonica.FPS(meterFPS), 10.0, 1.0),
		// Critically-damped spring for the peak marker: damping ratio 1.0 guarantees
		// a monotonic, no-overshoot approach so the eased marker/label never report a
		// value louder than the measured peak-hold.
		peakSpring: harmonica.NewSpring(harmonica.FPS(meterFPS), 8.0, 1.0),
	}
}

// meterTick schedules the next spring step for the eased audio level meter.
func meterTick() tea.Cmd {
	return tea.Tick(time.Second/meterFPS, func(time.Time) tea.Msg {
		return meterTickMsg{}
	})
}

// fileActive reports whether a file is still being worked on and therefore its
// meter should keep easing.
func fileActive(s FileStatus) bool {
	switch s {
	case StatusAnalysing, StatusProcessing, StatusNormalising:
		return true
	default:
		return false
	}
}

// anyActive reports whether at least one file is still active, gating the tick
// loop so it terminates once processing finishes.
func (m Model) anyActive() bool {
	for i := range m.Files {
		if fileActive(m.Files[i].Status) {
			return true
		}
	}
	return false
}

// Init initializes the model and starts the meter tick loop.
func (m Model) Init() tea.Cmd {
	return meterTick()
}

// processingFooterHeight is the row count View() reserves below the viewport for
// the scroll-hint footer. It is ALWAYS 1, even when the queue fits and the hint
// is blank: reserving the row unconditionally keeps the viewport height (and so
// the file boxes) from reflowing when the hint toggles on overflow.
const processingFooterHeight = 1

// scrollbarWidth is the column reserved at the right edge of the viewport for the
// vertical scrollbar strip. The viewport is sized to m.Width - scrollbarWidth so
// the strip joins beside it without pushing content off-screen; the column is
// reserved unconditionally (a blank strip when the queue fits) so the file boxes
// never reflow when the scrollbar toggles on overflow.
const scrollbarWidth = 1

// sizeViewport (re)builds and sizes the file-queue viewport from the current
// terminal dimensions. The viewport height is the terminal height minus the
// rendered header height (title + overall box) and the footer reservation,
// floored at 1 so a tiny terminal still yields a usable viewport. The header
// height is measured from the rendered header, not guessed, so it tracks any
// future header change automatically.
func (m *Model) sizeViewport() {
	if m.Width <= 0 || m.Height <= 0 {
		return
	}
	headerHeight := lipgloss.Height(renderProcessingHeader(*m))
	vpHeight := max(m.Height-headerHeight-processingFooterHeight, 1)
	// Reserve one column for the scrollbar strip, floored at 1 so a tiny terminal
	// still yields a usable viewport.
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

// refreshViewportContent re-renders the file queue into the PERSISTENT viewport
// so its content (and therefore its scrollable height) tracks the model. It must
// run in Update, not View: View has a value receiver, so any SetContent there
// mutates a throwaway copy and the real viewport stays empty and unscrollable.
//
// Follow-the-active-files: if the user has not scrolled up (still at the bottom),
// re-pin to the bottom after setting content so in-progress entries stay visible
// as the list grows. A user who scrolled up keeps their offset (the wheel/key
// branch never calls this, so it never yanks them back down). renderFileQueue
// takes the model by value, hence *m.
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

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonMsg(msg, &m.Width, &m.Height, &m.Done, &m.progress, processingBarOverhead); handled {
		// handleCommonMsg owns WindowSizeMsg (it stores the new dimensions); size
		// the file-queue viewport to the area below the fixed header, then load its
		// content so the scrollable height is correct from the first frame.
		if _, ok := msg.(tea.WindowSizeMsg); ok {
			m.sizeViewport()
			m.invalidateStableEntryCaches()
			m.refreshViewportContent()
		}
		// Scroll keys (PgUp/PgDn/arrows) are KeyPressMsg values that handleCommonMsg
		// does NOT own (only the quit keys), so they fall through below. The quit
		// keys return tea.Quit here and never reach the viewport.
		return m, cmd
	}

	// Forward scroll input (mouse wheel + pager keys) to the viewport so it can
	// page the file queue. handleCommonMsg already consumed the quit keys and the
	// resize, so they never reach here; everything else is safe to forward. Do NOT
	// refresh content here: refreshViewportContent re-pins to the bottom, which
	// would cancel the user's upward scroll. The content set on the prior message
	// is still loaded, so there is something to scroll.
	switch msg.(type) {
	case tea.MouseWheelMsg, tea.KeyPressMsg:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case ProgressMsg:
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.Files) {
			// Deliberate in-place write into the aliased Files backing array; safe because Bubbletea drives Update/View serially.
			m.Files[msg.FileIndex] = updateFileProgress(m.Files[msg.FileIndex], msg)
			m.invalidateFileEntryCache(msg.FileIndex)
		}
		m.refreshViewportContent()
		return m, nil

	case FileStartMsg:
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.Files) {
			// Deliberate in-place write into the aliased Files backing array; safe because Bubbletea drives Update/View serially.
			m.Files[msg.FileIndex].Status = StatusAnalysing
			m.Files[msg.FileIndex].StartTime = time.Now()
			m.invalidateFileEntryCache(msg.FileIndex)
		}
		m.refreshViewportContent()
		return m, nil

	case AdaptedSummaryMsg:
		// State-change update for the filter-chain status boxes. Re-renders on
		// receipt only; the 60 fps meter tick never carries this. Sent at Pass-2
		// start (chain + analysis) and at completion (limiter ceiling).
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.Files) {
			// Deliberate in-place write into the aliased Files backing array; safe because Bubbletea drives Update/View serially.
			m.Files[msg.FileIndex].Summary = msg.Summary
			// Invalidate the memoised side panels: the summary is the primary cache
			// key, so a new one must force a re-render. joinStatusBoxes also re-checks
			// the Pass-box height, so this clear plus the height check covers every
			// input the panels read.
			m.Files[msg.FileIndex].statusBoxCache.valid = false
			m.invalidateFileEntryCache(msg.FileIndex)
		}
		m.refreshViewportContent()
		return m, nil

	case FileCompleteMsg:
		if msg.FileIndex >= 0 && msg.FileIndex < len(m.Files) {
			// Deliberate in-place write into the aliased Files backing array; safe because Bubbletea drives Update/View serially.
			m.Files[msg.FileIndex].Status = StatusComplete
			m.Files[msg.FileIndex].CompletionResult = msg.CompletionResult

			if msg.Error != nil {
				m.Files[msg.FileIndex].Status = StatusError
				m.FailedFiles++
			} else {
				m.CompletedFiles++
			}
			m.invalidateFileEntryCache(msg.FileIndex)
		}
		m.refreshViewportContent()
		return m, nil

	case meterTickMsg:
		// Step each active file's meter spring toward its target level, then
		// re-schedule only while work remains. Stop once m.Done is set or no
		// file is active, guaranteeing the loop terminates on AllCompleteMsg.
		if m.Done {
			return m, nil
		}
		for i := range m.Files {
			if !fileActive(m.Files[i].Status) {
				continue
			}
			if i >= len(m.meters) {
				continue
			}
			if m.Files[i].HasLevel {
				target := m.Files[i].CurrentLevel
				// Deliberate in-place write into the aliased meters backing array; safe because Bubbletea drives Update/View serially.
				m.meters[i].pos, m.meters[i].vel = m.spring.Update(
					m.meters[i].pos, m.meters[i].vel, target)
				m.meters[i].peakPos, m.meters[i].peakVel = m.peakSpring.Update(
					m.meters[i].peakPos, m.meters[i].peakVel, m.Files[i].PeakLevel)
			}
			m.meters[i].progPos, m.meters[i].progVel = m.progressSpring.Update(
				m.meters[i].progPos, m.meters[i].progVel, m.Files[i].Progress)
		}
		// Render the full active entries, then skip the viewport write when the
		// rendered output is byte-for-byte unchanged from the previous tick.
		m.refreshViewportContentIfChanged(true)
		if !m.anyActive() {
			return m, nil
		}
		return m, meterTick()
	}

	return m, nil
}

// View renders the UI
func (m Model) View() tea.View {
	// Render a placeholder until the first WindowSizeMsg sets m.Width.
	if m.Width == 0 {
		view := tea.NewView(fmt.Sprintf("Initialising...\nFiles: %d\n", len(m.Files)))
		view.AltScreen = true
		return view
	}

	var view tea.View
	if m.Done {
		// On completion the program quits to the normal buffer (AllCompleteMsg ->
		// tea.Quit), where the completion summary is reprinted with native
		// scrollback. Render it whole here for the brief final frame.
		view = tea.NewView(renderCompletionSummary(m))
	} else {
		view = tea.NewView(m.renderScrollingView())
	}
	view.AltScreen = true
	// Enable mouse cell-motion so the viewport receives MouseWheelMsg; its own
	// MouseWheelEnabled (default true) then scrolls the file queue.
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

// renderScrollingView composes the pinned header with the scrollable file queue.
// The header (title + overall progress) stays fixed; the file list lives inside
// the viewport so the user can scroll it with the wheel or pager keys during a
// run. Before the first WindowSizeMsg sizes the viewport, it falls back to the
// unscrolled stack so early frames still render.
func (m Model) renderScrollingView() string {
	if !m.vpReady {
		return renderProcessingView(m)
	}
	// Pure render: the viewport's content and scroll offset are managed in Update
	// (refreshViewportContent), since View's value receiver discards any mutation.
	//
	// The scrollbar and the scroll hint appear ONLY when the file queue overflows
	// the viewport. Both the scrollbar column and the hint row are reserved
	// unconditionally in sizeViewport / processingFooterHeight, so toggling them
	// fills a pre-reserved slot and never reflows the file boxes.
	overflow := m.vp.TotalLineCount() > m.vp.Height()

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.vp.View(), m.scrollbarStrip(overflow))

	hint := ""
	if overflow {
		hint = renderScrollHint()
	}

	return renderProcessingHeader(m) + "\n" + body + "\n" + hint
}

// scrollbarStrip returns the right-edge scrollbar column for the viewport. On
// overflow it is the live thumb/track strip from the viewport's scroll state; when
// the queue fits it is a blank strip of spaces so the reserved column holds its
// width and the file boxes do not reflow. Read-only viewport queries only, keeping
// renderScrollingView pure.
func (m Model) scrollbarStrip(overflow bool) string {
	vpHeight := m.vp.Height()
	if !overflow {
		return strings.TrimRight(strings.Repeat(" \n", vpHeight), "\n")
	}
	return renderScrollbar(vpHeight, m.vp.TotalLineCount(), vpHeight, m.vp.ScrollPercent())
}

// updateFileProgress updates a FileProgress based on a ProgressMsg
func updateFileProgress(fp FileProgress, msg ProgressMsg) FileProgress {
	// Reset the start time when transitioning to a new pass
	if msg.Pass != fp.CurrentPass {
		fp.StartTime = time.Now()
	}

	fp.Progress = msg.Progress
	fp.CurrentPass = msg.Pass
	fp.PassName = msg.PassName
	fp.ElapsedTime = time.Since(fp.StartTime)

	// Duration is constant per file; keep the first non-zero value seen.
	if msg.Duration > 0 && fp.Duration == 0 {
		fp.Duration = msg.Duration
	}

	if msg.Measurements != nil {
		fp.Measurements = msg.Measurements
	}

	if msg.HasLevel {
		fp.CurrentLevel = msg.Level
		fp.HasLevel = true
		if msg.Level > fp.PeakLevel {
			fp.PeakLevel = msg.Level
		}
	}

	switch msg.Pass {
	case processor.PassAnalysis:
		fp.Status = StatusAnalysing
	case processor.PassProcessing:
		fp.Status = StatusProcessing
	case processor.PassMeasuring, processor.PassNormalising:
		fp.Status = StatusNormalising
	}

	return fp
}
