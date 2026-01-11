package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ANSI sequence part
type ansiPart struct {
	text  string
	fg    string
	bold  bool
}

// convertAnsiToSVG converts colored terminal output to a styled SVG ensemble
func convertAnsiToSVG(ansi string) string {
	lines := strings.Split(ansi, "\n")
	
	// Default styles
	fontSize := 14
	lineHeight := 1.2
	charWidth := 8.5
	
	var maxLen int
	for _, l := range lines {
		stripped := stripAnsi(l)
		if len(stripped) > maxLen {
			maxLen = len(stripped)
		}
	}

	width := float64(maxLen)*charWidth + 80
	height := float64(len(lines))*float64(fontSize)*lineHeight + 80

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg width="%.1f" height="%.1f" viewBox="0 0 %.1f %.1f" xmlns="http://www.w3.org/2000/svg">`, width, height, width, height))
	
	// Add Shadow
	sb.WriteString(fmt.Sprintf(`<rect x="35" y="35" width="%.1f" height="%.1f" rx="12" fill="rgba(0,0,0,0.5)" filter="blur(10px)" />`, width-60, height-60))
	
	// Main Frame
	sb.WriteString(fmt.Sprintf(`<rect x="20" y="20" width="%.1f" height="%.1f" rx="12" fill="#000" stroke="#7D56F4" stroke-width="2" />`, width-60, height-60))
	
	// Title/Controls dots (Mac style)
	sb.WriteString(`<circle cx="45" cy="40" r="6" fill="#FF5F56"/>`)
	sb.WriteString(`<circle cx="65" cy="40" r="6" fill="#FFBD2E"/>`)
	sb.WriteString(`<circle cx="85" cy="40" r="6" fill="#27C93F"/>`)

	sb.WriteString(`<text font-family="monospace" font-size="14" y="20">`)

	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	
	for i, line := range lines {
		yPos := 70 + (i * int(float64(fontSize)*lineHeight))
		sb.WriteString(fmt.Sprintf(`<tspan x="40" y="%d">`, yPos))
		
		parts := parseAnsiLine(line, re)
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
			
			sb.WriteString(fmt.Sprintf(`<tspan style="%s">%s</tspan>`, style, escapedText))
		}
		sb.WriteString(`</tspan>`)
	}
	
	sb.WriteString(`</text></svg>`)
	return sb.String()
}

func parseAnsiLine(line string, re *regexp.Regexp) []ansiPart {
	var parts []ansiPart
	currFg := ""
	currBold := false
	
	indices := re.FindAllStringIndex(line, -1)
	lastEnd := 0
	
	for _, idx := range indices {
		if idx[0] > lastEnd {
			parts = append(parts, ansiPart{text: line[lastEnd:idx[0]], fg: currFg, bold: currBold})
		}
		
		code := line[idx[0]:idx[1]]
		if code == "\x1b[0m" {
			currFg = ""
			currBold = false
		} else {
			// Basic color parsing (limited for MVP)
			if strings.Contains(code, "1;") || code == "\x1b[1m" {
				currBold = true
			}
			// Look for HEX or 256 colors if possible, else use mapping
			if strings.Contains(code, "38;5;") || strings.Contains(code, "38;2;") {
				// Complex color - for now default to a highlight
				currFg = "#7D56F4"
			} else if strings.Contains(code, "35") {
				currFg = "#EE6FF8" // userStyle
			} else if strings.Contains(code, "36") {
				currFg = "#04D9FF" // aiStyle
			}
		}
		lastEnd = idx[1]
	}
	
	if lastEnd < len(line) {
		parts = append(parts, ansiPart{text: line[lastEnd:], fg: currFg, bold: currBold})
	}
	
	return parts
}

func stripAnsi(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(str, "")
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
