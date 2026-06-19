package ui

import (
	"strconv"
	"strings"
)

// ansiColorTriples extracts the distinct RGB foreground triples (38;2;r;g;b)
// from a styled string, in order of first appearance. With anchored true, the
// 38;2; marker must start the SGR segment (the bar fill case). With anchored
// false, the marker may follow leading SGR attributes such as the title's bold
// 1; prefix (the header case). The empty-track / non-foreground colours fall out
// naturally because they carry no 38;2; foreground.
func ansiColorTriples(s string, anchored bool) [][3]int {
	var out [][3]int
	seen := map[[3]int]bool{}
	for seg := range strings.SplitSeq(s, "\x1b[") {
		var body string
		if anchored {
			if !strings.HasPrefix(seg, "38;2;") {
				continue
			}
			body, _, _ = strings.Cut(seg, "m")
		} else {
			_, after, found := strings.Cut(seg, "38;2;")
			if !found {
				continue
			}
			rest, _, _ := strings.Cut(after, "m")
			body = "38;2;" + rest
		}
		parts := strings.Split(body, ";")
		// parts holds the SGR codes; the r;g;b channels follow the 38;2; marker.
		if len(parts) < 5 {
			continue
		}
		r, err1 := strconv.Atoi(parts[2])
		g, err2 := strconv.Atoi(parts[3])
		b, err3 := strconv.Atoi(parts[4])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		c := [3]int{r, g, b}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
