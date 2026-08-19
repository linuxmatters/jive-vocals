module github.com/linuxmatters/jive-vocals

go 1.26

require (
	charm.land/bubbles/v2 v2.1.1
	charm.land/bubbletea/v2 v2.0.8
	charm.land/lipgloss/v2 v2.0.6
	github.com/alecthomas/kong v1.16.1
	github.com/charmbracelet/colorprofile v0.4.3
	github.com/charmbracelet/harmonica v0.2.0
	github.com/charmbracelet/x/ansi v0.11.8
	github.com/linuxmatters/ffmpeg-statigo v0.0.0-00010101000000-000000000000
	github.com/mattn/go-runewidth v0.0.27
)

replace github.com/linuxmatters/ffmpeg-statigo => ./third_party/ffmpeg-statigo

require (
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
