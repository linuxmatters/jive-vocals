package cli

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// helpTestCLI is a minimal Kong grammar exercising every branch of getFlags and
// getArguments: a bool flag, a value flag with a placeholder, a short+long flag,
// and a positional argument. Kong synthesises the -h/--help flag itself, so it is
// not declared here.
type helpTestCLI struct {
	Files []string `arg:"" name:"files" help:"Audio files to process" optional:""`

	Debug   bool   `name:"debug" help:"Enable debug logging"`
	Output  string `name:"output" placeholder:"path" help:"Write result here"`
	Verbose bool   `short:"v" name:"verbose"`
}

// newHelpTestContext builds a kong.Context from helpTestCLI the same way main.go
// does (kong.New then Parse), so getFlags/getArguments see a real flag model.
func newHelpTestContext(t *testing.T, args ...string) *kong.Context {
	t.Helper()
	k, err := kong.New(&helpTestCLI{}, kong.Name("jive-vocals"))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	ctx, err := k.Parse(args)
	if err != nil {
		t.Fatalf("kong parse: %v", err)
	}
	return ctx
}

// findRow returns the helpRow whose label matches, or fails the test.
func findRow(t *testing.T, rows []helpRow, label string) helpRow {
	t.Helper()
	for _, r := range rows {
		if r.label == label {
			return r
		}
	}
	t.Fatalf("no row with label %q in %+v", label, rows)
	return helpRow{}
}

// TestWriteHelpSectionRendersRows asserts the shared section writer produces the
// header, the two-space indent, the styled label, the two-space help separator,
// and a trailing newline per row, matching the prior per-section render loops.
func TestWriteHelpSectionRendersRows(t *testing.T) {
	var sb strings.Builder
	writeHelpSection(&sb, "Flags:", helpFlagStyle, []helpRow{
		{label: "-h, --help", help: "Show context-sensitive help."},
		{label: "--debug", help: ""},
	})

	got := sb.String()
	want := "\n" +
		helpSectionStyle.Render("Flags:") + "\n" +
		"  " + helpFlagStyle.Render("-h, --help") + "  Show context-sensitive help.\n" +
		"  " + helpFlagStyle.Render("--debug") + "\n"

	if got != want {
		t.Errorf("writeHelpSection mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestWriteHelpSectionEmptyRowsWritesNothing confirms an empty row slice yields no
// output, matching the prior len(rows) > 0 guard around each section.
func TestWriteHelpSectionEmptyRowsWritesNothing(t *testing.T) {
	var sb strings.Builder
	writeHelpSection(&sb, "Arguments:", helpArgStyle, nil)
	if got := sb.String(); got != "" {
		t.Errorf("expected no output for empty rows, got %q", got)
	}
}

// TestGetFlagsFormatsLabels pins the hand-rolled flag-string formatting in
// getFlags: the prepended -h/--help row, the long-only bool flag, the
// short+long join, and the upcased =PLACEHOLDER on a value flag.
func TestGetFlagsFormatsLabels(t *testing.T) {
	rows := getFlags(newHelpTestContext(t))

	tests := []struct {
		name      string
		wantLabel string
		wantHelp  string
	}{
		{
			name:      "help prepended by hand",
			wantLabel: "-h, --help",
			wantHelp:  "Show context-sensitive help.",
		},
		{
			name:      "long-only bool flag has no placeholder",
			wantLabel: "--debug",
			wantHelp:  "Enable debug logging",
		},
		{
			name:      "value flag upcases its placeholder",
			wantLabel: "--output=PATH",
			wantHelp:  "Write result here",
		},
		{
			name:      "short and long flag join with comma",
			wantLabel: "-v, --verbose",
			wantHelp:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := findRow(t, rows, tt.wantLabel)
			if row.help != tt.wantHelp {
				t.Errorf("help for %q = %q, want %q", tt.wantLabel, row.help, tt.wantHelp)
			}
		})
	}
}

// TestGetFlagsHelpFirstAndDeduplicated confirms the manual --help row is first
// and Kong's own help flag is not emitted a second time.
func TestGetFlagsHelpFirstAndDeduplicated(t *testing.T) {
	rows := getFlags(newHelpTestContext(t))

	if len(rows) == 0 || rows[0].label != "-h, --help" {
		t.Fatalf("first row = %+v, want -h, --help first", rows)
	}

	help := 0
	for _, r := range rows {
		if strings.Contains(r.label, "--help") {
			help++
		}
	}
	if help != 1 {
		t.Errorf("--help appears %d times, want exactly 1", help)
	}
}

// hiddenFlagCLI is a Kong grammar pairing a hidden flag with a visible sibling,
// exercising the Hidden guard in getFlags.
type hiddenFlagCLI struct {
	Secret  bool `name:"secret" hidden:"" help:"Internal only"`
	Visible bool `name:"visible" help:"Shown in help"`
}

// TestGetFlagsOmitsHiddenFlags confirms a hidden:"" flag is left out of the
// rendered Flags section while its visible sibling appears.
func TestGetFlagsOmitsHiddenFlags(t *testing.T) {
	k, err := kong.New(&hiddenFlagCLI{}, kong.Name("jive-vocals"))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	ctx, err := k.Parse(nil)
	if err != nil {
		t.Fatalf("kong parse: %v", err)
	}

	var sb strings.Builder
	writeHelpSection(&sb, "Flags:", helpFlagStyle, getFlags(ctx))
	got := sb.String()

	if strings.Contains(got, "--secret") {
		t.Errorf("hidden flag --secret appears in help output:\n%s", got)
	}
	if !strings.Contains(got, "--visible") {
		t.Errorf("visible flag --visible missing from help output:\n%s", got)
	}
}

// TestGetArgumentsRendersPositionals confirms getArguments lists positional
// arguments with their summary label and help text.
func TestGetArgumentsRendersPositionals(t *testing.T) {
	rows := getArguments(newHelpTestContext(t))

	if len(rows) != 1 {
		t.Fatalf("got %d argument rows, want 1: %+v", len(rows), rows)
	}
	if !strings.Contains(rows[0].label, "files") {
		t.Errorf("argument label = %q, want it to mention files", rows[0].label)
	}
	if rows[0].help != "Audio files to process" {
		t.Errorf("argument help = %q, want %q", rows[0].help, "Audio files to process")
	}
}
