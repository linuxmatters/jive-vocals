package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/colorprofile"
)

// Palette is the single source of colours for both the cli and ui packages.
// Every value is a compat.AdaptiveColor, which satisfies image/color's Color
// interface and resolves Light/Dark variants at render time from the terminal
// background detected globally by the compat package. Only a named subset
// adapts: ColorText, ColorFill, ColorSkyBlue, and ColorBlue carry distinct
// Light and Dark variants. The rest are fixed brand colours with Light == Dark,
// expressed through the same type for uniformity. Use these instead of bespoke
// lipgloss.Color literals.
var (
	// ColorRed is the Jive Vocals brand red (errors, titles, peak zone).
	ColorRed = compat.AdaptiveColor{Light: lipgloss.Color("#A40000"), Dark: lipgloss.Color("#A40000")}
	// ColorCyanBright is the bright cyan start of the header letter gradient. Its
	// CIELAB path to ColorSkyBlue stays vivid (no muddy midpoint).
	ColorCyanBright = compat.AdaptiveColor{Light: lipgloss.Color("#00D4FF"), Dark: lipgloss.Color("#00D4FF")}
	// ColorMuted is the muted grey for labels and secondary borders.
	ColorMuted = compat.AdaptiveColor{Light: lipgloss.Color("#888888"), Dark: lipgloss.Color("#888888")}
	// ColorText is the primary value text colour.
	ColorText = compat.AdaptiveColor{Light: lipgloss.Color("#1A1A1A"), Dark: lipgloss.Color("#FFFFFF")}
	// ColorOrange is the warning / caution zone colour.
	ColorOrange = compat.AdaptiveColor{Light: lipgloss.Color("#FFA500"), Dark: lipgloss.Color("#FFA500")}
	// ColorGreen is the success / safe zone colour.
	ColorGreen = compat.AdaptiveColor{Light: lipgloss.Color("#00AA00"), Dark: lipgloss.Color("#00AA00")}
	// ColorYellow is the mid-warm VU-meter stop between green and orange.
	ColorYellow = compat.AdaptiveColor{Light: lipgloss.Color("#E6E600"), Dark: lipgloss.Color("#E6E600")}
	// ColorCyan is the accent colour used in help output.
	ColorCyan = compat.AdaptiveColor{Light: lipgloss.Color("#00AAAA"), Dark: lipgloss.Color("#00AAAA")}
	// ColorFill is the empty/unfilled fill colour for meters and progress.
	ColorFill = compat.AdaptiveColor{Light: lipgloss.Color("#CCCCCC"), Dark: lipgloss.Color("#444444")}
	// ColorSkyBlue is the sky-blue used for panel borders.
	ColorSkyBlue = compat.AdaptiveColor{Light: lipgloss.Color("#0284C7"), Dark: lipgloss.Color("#38BDF8")}
	// ColorIndigo is the indigo end of the progress bar gradient.
	ColorIndigo = compat.AdaptiveColor{Light: lipgloss.Color("#6366F1"), Dark: lipgloss.Color("#6366F1")}
	// ColorOrangeDim is the deep-orange trough of the peak-marker pulse.
	ColorOrangeDim = compat.AdaptiveColor{Light: lipgloss.Color("#B35F00"), Dark: lipgloss.Color("#B35F00")}
	// ColorBlue is the cold end of the gain thermometer (under-recorded peaks).
	ColorBlue = compat.AdaptiveColor{Light: lipgloss.Color("#2563EB"), Dark: lipgloss.Color("#3B82F6")}
)

// Text styles for the version banner and the Print* helpers below.
var (
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorRed)

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorOrange)

	keyStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText)
)

// renderTitleOnce builds the wordmark once on first call and caches it. The
// output never varies (string literal plus package-level colours resolved from
// the terminal background detected once at startup), and RenderTitle is called
// every TUI frame, so the work is hoisted off the 60fps path. Lazy so the first
// call happens after terminal detection completes.
var renderTitleOnce = sync.OnceValue(func() string {
	letters := []rune("Jive Vocals")
	ramp := lipgloss.Blend1D(len(letters), ColorCyanBright, ColorSkyBlue)

	var b strings.Builder
	for i, r := range letters {
		b.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(ramp[i]).
			Render(string(r)))
	}
	b.WriteString(" 🗣️")

	return b.String()
})

// RenderTitle returns the "Jive Vocals 🗣️ wordmark drawn as a per-letter
// cyan→sky-blue Blend1D gradient (bold per letter), with the 🗣️emoji appended
// outside the gradient so it keeps its own colours. Shared by the version banner
// and the processing-TUI header so both render the wordmark identically.
func RenderTitle() string { return renderTitleOnce() }

func styledWriter(w io.Writer) io.Writer {
	return colorprofile.NewWriter(w, os.Environ())
}

// PrintVersion prints version information
func PrintVersion(version string) {
	printVersion(os.Stdout, version)
}

func printVersion(w io.Writer, version string) {
	out := styledWriter(w)
	fmt.Fprintln(out, RenderTitle())
	fmt.Fprintf(out, "%s %s\n", keyStyle.Render("Version:"), valueStyle.Render(version))
	fmt.Fprintln(out)
}

// printLabelled writes a styled label followed by message to stderr.
func printLabelled(w io.Writer, style lipgloss.Style, label, message string) {
	fmt.Fprintf(styledWriter(w), "%s %s\n", style.Render(label), message)
}

// PrintError prints an error message
func PrintError(message string) {
	printLabelled(os.Stderr, errorStyle, "Error:", message)
}

// PrintWarning prints a warning message
func PrintWarning(message string) {
	printLabelled(os.Stderr, warningStyle, "Warning:", message)
}
