package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// ANSI sequence part
type ansiPart struct {
	text string
	fg   string
	bold bool
}

// convertAnsiToSVG converts colored terminal output to a styled SVG ensemble
func convertAnsiToSVG(ansi string) string {
	lines := strings.Split(ansi, "\n")

	// Keep only SGR sequences (colors/styles). Remove cursor/alt-screen/etc.
	reSGR := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reCSI := regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	reOSC := regexp.MustCompile(`\x1b\][^\x07]*(\x07|\x1b\\)`)

	cleanLines := make([]string, 0, len(lines))
	for _, l := range lines {
		cleanLines = append(cleanLines, sanitizeANSI(l, reCSI, reOSC))
	}

	// Detect a common full-width right border column (Lipgloss borders often
	// render a vertical bar at the terminal width, making screenshots massive).
	borderCol := detectRightBorderColumn(cleanLines, reSGR)

	// Compute real content width (in terminal columns, not bytes), trimming
	// trailing whitespace and ignoring the detected right-side border.
	maxCols := 0
	for _, l := range cleanLines {
		cols := visibleTrimmedWidth(l, reSGR)
		if borderCol > 0 && cols == borderCol {
			if r, ok := lastNonSpaceRune(reSGR.ReplaceAllString(l, "")); ok && isBorderRune(r) {
				cols -= runewidth.RuneWidth(r)
			}
		}
		if cols > maxCols {
			maxCols = cols
		}
	}
	if maxCols < 1 {
		maxCols = 1
	}

	// Truncate lines to the computed width so the rendered SVG is actually cropped.
	for i := range cleanLines {
		cleanLines[i] = truncateAnsiLineToWidth(cleanLines[i], maxCols, reSGR)
	}

	// Refined dimensions
	fontSize := 14
	lineHeight := 1.25
	charWidth := 8.2

	paddingX := 30.0
	paddingY := 60.0

	width := float64(maxCols)*charWidth + (paddingX * 2)
	height := float64(len(cleanLines))*float64(fontSize)*lineHeight + paddingY + 40

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg width="%.1f" height="%.1f" viewBox="0 0 %.1f %.1f" xmlns="http://www.w3.org/2000/svg">`, width, height, width, height))

	// Add Shadow
	sb.WriteString(fmt.Sprintf(`<rect x="15" y="15" width="%.1f" height="%.1f" rx="12" fill="rgba(0,0,0,0.4)" filter="blur(8px)" />`, width-20, height-20))

	// Main Frame
	sb.WriteString(fmt.Sprintf(`<rect x="10" y="10" width="%.1f" height="%.1f" rx="12" fill="#0D0D0D" stroke="#7D56F4" stroke-width="2" />`, width-20, height-20))

	// Title/Controls dots (Mac style)
	sb.WriteString(`<circle cx="35" cy="30" r="5" fill="#FF5F56"/>`)
	sb.WriteString(`<circle cx="55" cy="30" r="5" fill="#FFBD2E"/>`)
	sb.WriteString(`<circle cx="75" cy="30" r="5" fill="#27C93F"/>`)

	sb.WriteString(`<text font-family="Menlo, Monaco, Consolas, Courier New, monospace" font-size="14" xml:space="preserve">`)

	for i, line := range cleanLines {
		yPos := 70 + (i * int(float64(fontSize)*lineHeight))
		sb.WriteString(fmt.Sprintf(`<tspan x="%d" y="%d">`, int(paddingX), yPos))

		parts := parseAnsiLine(line, reSGR)
		for _, p := range parts {
			style := ""
			if p.fg != "" {
				style += fmt.Sprintf("fill:%s;", p.fg)
			} else {
				style += "fill:#FAFAFA;"
			}
			if p.bold {
				style += "font-weight:bold;"
			}

			escapedText := strings.ReplaceAll(p.text, "&", "&amp;")
			escapedText = strings.ReplaceAll(escapedText, "<", "&lt;")
			escapedText = strings.ReplaceAll(escapedText, ">", "&gt;")
			// Ensure spaces are visible
			escapedText = strings.ReplaceAll(escapedText, " ", "&#160;")

			sb.WriteString(fmt.Sprintf(`<tspan style="%s">%s</tspan>`, style, escapedText))
		}
		sb.WriteString(`</tspan>`)
	}

	sb.WriteString(`</text></svg>`)
	return sb.String()
}

func sanitizeANSI(line string, reCSI, reOSC *regexp.Regexp) string {
	// Strip OSC sequences entirely (titles, hyperlinks, etc).
	line = reOSC.ReplaceAllString(line, "")
	// Strip CSI sequences unless they are SGR (ending with 'm').
	return reCSI.ReplaceAllStringFunc(line, func(seq string) string {
		if strings.HasSuffix(seq, "m") {
			return seq
		}
		return ""
	})
}

func visibleTrimmedWidth(line string, reSGR *regexp.Regexp) int {
	visible := reSGR.ReplaceAllString(line, "")
	visible = strings.TrimRight(visible, " \t")
	return runewidth.StringWidth(visible)
}

func lastNonSpaceRune(s string) (rune, bool) {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return 0, false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r, true
}

func isBorderRune(r rune) bool {
	switch r {
	case '|',
		'│', '┃', '║',
		'┤', '├', '┐', '┘', '┌', '└',
		'┬', '┴', '┼',
		'╡', '╢', '╣', '╠', '╞',
		'╭', '╮', '╯', '╰',
		'─', '━', '═':
		return true
	default:
		return false
	}
}

func detectRightBorderColumn(lines []string, reSGR *regexp.Regexp) int {
	counts := map[int]int{}
	for _, l := range lines {
		visible := reSGR.ReplaceAllString(l, "")
		visible = strings.TrimRight(visible, " \t")
		if visible == "" {
			continue
		}
		last, ok := lastNonSpaceRune(visible)
		if !ok || !isBorderRune(last) {
			continue
		}
		col := runewidth.StringWidth(visible)
		counts[col]++
	}

	bestCol := 0
	bestCount := 0
	for col, count := range counts {
		if count > bestCount {
			bestCol = col
			bestCount = count
		}
	}

	// Heuristic: if many lines share the same ending border column, treat it as a
	// full-width frame and crop it away.
	if bestCount >= 3 && bestCount >= len(lines)/3 {
		return bestCol
	}
	return 0
}

func truncateAnsiLineToWidth(line string, maxCols int, reSGR *regexp.Regexp) string {
	if maxCols <= 0 || line == "" {
		return ""
	}

	indices := reSGR.FindAllStringIndex(line, -1)
	var b strings.Builder
	visibleCols := 0
	lastEnd := 0

	writeText := func(segment string) bool {
		for _, r := range segment {
			rw := runewidth.RuneWidth(r)
			if rw == 0 {
				rw = 1
			}
			if visibleCols+rw > maxCols {
				return false
			}
			b.WriteRune(r)
			visibleCols += rw
		}
		return true
	}

	for _, idx := range indices {
		if idx[0] > lastEnd {
			if !writeText(line[lastEnd:idx[0]]) {
				return b.String()
			}
		}
		if visibleCols >= maxCols {
			return b.String()
		}
		b.WriteString(line[idx[0]:idx[1]])
		lastEnd = idx[1]
	}

	if lastEnd < len(line) {
		_ = writeText(line[lastEnd:])
	}
	return b.String()
}

func parseAnsiLine(line string, re *regexp.Regexp) []ansiPart {
	var parts []ansiPart
	currFg := "#FAFAFA"
	currBold := false

	indices := re.FindAllStringIndex(line, -1)
	lastEnd := 0

	for _, idx := range indices {
		if idx[0] > lastEnd {
			parts = append(parts, ansiPart{text: line[lastEnd:idx[0]], fg: currFg, bold: currBold})
		}

		code := line[idx[0]:idx[1]]
		if code == "\x1b[0m" {
			currFg = "#FAFAFA"
			currBold = false
		} else {
			// Handle TrueColor: \x1b[38;2;r;g;bm
			if strings.Contains(code, "38;2;") {
				clean := strings.Trim(code, "\x1b[m")
				parts := strings.Split(clean, ";")
				if len(parts) >= 5 {
					r, _ := strconv.Atoi(parts[2])
					g, _ := strconv.Atoi(parts[3])
					b, _ := strconv.Atoi(parts[4])
					currFg = fmt.Sprintf("#%02x%02x%02x", r, g, b)
				}
			} else if strings.Contains(code, "38;5;") {
				currFg = "#7D56F4"
			} else {
				// Map basic colors only if not TrueColor
				if strings.Contains(code, "35") {
					currFg = "#EE6FF8"
				} else if strings.Contains(code, "36") {
					currFg = "#04D9FF"
				} else if strings.Contains(code, "34") {
					currFg = "#7D56F4"
				}
			}

			if strings.Contains(code, ";1m") || strings.Contains(code, "[1;") || code == "\x1b[1m" {
				currBold = true
			}
		}
		lastEnd = idx[1]
	}

	if lastEnd < len(line) {
		parts = append(parts, ansiPart{text: line[lastEnd:], fg: currFg, bold: currBold})
	}

	return parts
}

// convertToPNG attempts to convert SVG to PNG using system tools
func convertToPNG(svgPath, pngPath string) error {
	// Try rsvg-convert (common on Linux)
	if _, err := exec.LookPath("rsvg-convert"); err == nil {
		return exec.Command("rsvg-convert", "-o", pngPath, svgPath).Run()
	}

	// Try ImageMagick
	if _, err := exec.LookPath("magick"); err == nil {
		return exec.Command("magick", svgPath, pngPath).Run()
	} else if _, err := exec.LookPath("convert"); err == nil {
		return exec.Command("convert", svgPath, pngPath).Run()
	}

	// Try ffmpeg (common on Termux)
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return exec.Command("ffmpeg", "-i", svgPath, pngPath).Run()
	}

	return fmt.Errorf("no conversion tool found (rsvg-convert, magick, or ffmpeg)")
}
