package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/kong"
)

// Help styles for the section headers, flag names, and argument names in the
// StyledHelpPrinter output.
var (
	helpDescStyle = lipgloss.NewStyle().
			Foreground(ColorOrange).
			Italic(true).
			MarginBottom(1)

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorOrange).
				MarginTop(1)

	helpFlagStyle = lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true)

	helpArgStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)
)

// StyledHelpPrinter returns a Kong help printer that renders the title,
// usage, arguments, and flags with the package Lipgloss styles, writing
// through a colorprofile writer so colour downsamples to the terminal.
func StyledHelpPrinter() func(kong.HelpOptions, *kong.Context) error {
	return func(options kong.HelpOptions, ctx *kong.Context) error {
		var sb strings.Builder
		width := helpWrapWidth(options)

		// Title and description (Kong's Description populates Model.Help)
		sb.WriteString(RenderTitle())
		sb.WriteString("\n")
		if ctx.Model.Help != "" {
			sb.WriteString(helpDescStyle.Render(ctx.Model.Help))
			sb.WriteString("\n")
		}

		// Usage
		sb.WriteString(helpSectionStyle.Render("Usage:"))
		sb.WriteString("\n  ")
		sb.WriteString(usageLine(ctx))
		sb.WriteString("\n")

		// Arguments and Flags sections
		writeHelpSection(&sb, "Arguments:", helpArgStyle, getArguments(ctx), width)
		writeHelpSection(&sb, "Flags:", helpFlagStyle, getFlags(ctx), width)

		sb.WriteString("\n")
		fmt.Fprint(styledWriter(ctx.Stdout), sb.String())
		return nil
	}
}

// helpRow is one label/help pair rendered in the Arguments or Flags section.
type helpRow struct {
	label string
	help  string
}

func usageLine(ctx *kong.Context) string {
	if selected := ctx.Selected(); selected != nil {
		return ctx.Model.Name + " " + selected.Summary()
	}
	return ctx.Model.Name + ctx.Model.Summary()
}

// writeHelpSection renders a help section (header plus label-styled rows) to sb,
// writing nothing when rows is empty. label is drawn with style, help follows
// after two spaces when present.
func writeHelpSection(sb *strings.Builder, header string, style lipgloss.Style, rows []helpRow, width int) {
	if len(rows) == 0 {
		return
	}

	sb.WriteString("\n")
	sb.WriteString(helpSectionStyle.Render(header))
	sb.WriteString("\n")
	for _, row := range rows {
		label := "  " + style.Render(row.label)
		sb.WriteString(label)
		if row.help != "" {
			writeWrappedHelp(sb, label, row.help, width)
		}
		sb.WriteString("\n")
	}
}

func helpWrapWidth(options kong.HelpOptions) int {
	width := 80
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			width = n
		}
	}
	if options.WrapUpperBound > 0 && width > options.WrapUpperBound {
		width = options.WrapUpperBound
	}
	return width
}

func writeWrappedHelp(sb *strings.Builder, label, help string, width int) {
	firstPrefix := label + "  "
	helpWidth := width - lipgloss.Width(firstPrefix)
	if helpWidth < 8 {
		sb.WriteString("\n")
		writeWrappedHelpLines(sb, "    ", help, width)
		return
	}

	lines := wrappedLines(help, helpWidth)
	if len(lines) == 0 {
		return
	}
	sb.WriteString("  ")
	sb.WriteString(lines[0])
	continuationPrefix := strings.Repeat(" ", lipgloss.Width(firstPrefix))
	for _, line := range lines[1:] {
		sb.WriteString("\n")
		sb.WriteString(continuationPrefix)
		sb.WriteString(line)
	}
}

func writeWrappedHelpLines(sb *strings.Builder, prefix, help string, width int) {
	helpWidth := width - lipgloss.Width(prefix)
	helpWidth = max(helpWidth, 1)
	for i, line := range wrappedLines(help, helpWidth) {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(prefix)
		sb.WriteString(line)
	}
}

func wrappedLines(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	wrapped := strings.TrimRight(lipgloss.Wrap(text, width, " "), "\n")
	if wrapped == "" {
		return nil
	}
	return strings.Split(wrapped, "\n")
}

func getArguments(ctx *kong.Context) []helpRow {
	var args []helpRow

	for _, arg := range ctx.Model.Positional {
		args = append(args, helpRow{label: arg.Summary(), help: arg.Help})
	}

	return args
}

func getFlags(ctx *kong.Context) []helpRow {
	var flags []helpRow

	// Kong omits --help from Model.Flags, so prepend it by hand.
	flags = append(flags, helpRow{
		label: "-h, --help",
		help:  "Show context-sensitive help.",
	})

	for _, f := range ctx.Model.Flags {
		if f.Hidden {
			continue
		}
		if f.Name == "help" {
			continue // the help flag is prepended above
		}

		flags = append(flags, helpRow{
			label: f.String(),
			help:  f.Help,
		})
	}

	return flags
}
