# Jive Vocals demo tape. Record with `just record`.

Output .demo.mp4

Require jive-vocals

Set Shell "/bin/sh"
Set Columns 116
Set Rows 32
Set FontFamily "Fira Code"
Set FontSize 14
Set Padding 16
Set Framerate 8
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"
Set CursorBlink false

Hide
Type 'PS1="$ " && cd "${JIVE_VOCALS_DEMO_DIR:-.}" && clear'
Enter
Sleep 200ms
Show

Sleep 400ms
Type "jive-vocals LMP-81s-mark.flac LMP-81s-popey.flac"
Enter
Sleep 7s
Screenshot pages/demo-still.png
Wait+Screen@60s /2 complete/
Sleep 2.5s
