package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Vibeauracle Color Palette - A vibrant, modern theme
var (
	// Primary accents
	ColorPrimary   = lipgloss.Color("#7C3AED") // Violet
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorAccent    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	ColorSuccess = lipgloss.Color("#10B981") // Emerald
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorInfo    = lipgloss.Color("#3B82F6") // Blue

	// Neutral tones
	ColorMuted = lipgloss.Color("#6B7280") // Gray
	ColorDim   = lipgloss.Color("#9CA3AF") // Light Gray
	ColorBold  = lipgloss.Color("#F3F4F6") // Almost White

	// Special
	ColorMagic   = lipgloss.Color("#EC4899") // Pink
	ColorNeon    = lipgloss.Color("#22D3EE") // Bright Cyan
	ColorSunrise = lipgloss.Color("#FB923C") // Orange
)

// CLI Styles - for colorful command-line output
var (
	cliTitle     = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	cliSubtitle  = lipgloss.NewStyle().Italic(true).Foreground(ColorSecondary)
	cliSuccess   = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	cliError     = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	cliWarning   = lipgloss.NewStyle().Foreground(ColorWarning)
	cliInfo      = lipgloss.NewStyle().Foreground(ColorInfo)
	cliLabel     = lipgloss.NewStyle().Foreground(ColorNeon).Bold(true)
	cliValue     = lipgloss.NewStyle().Foreground(ColorBold)
	cliMuted     = lipgloss.NewStyle().Foreground(ColorMuted)
	cliBullet    = lipgloss.NewStyle().Foreground(ColorMagic).Bold(true)
	cliCommand   = lipgloss.NewStyle().Foreground(ColorSunrise).Bold(true)
	cliHighlight = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	cliBadgeSuccess = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000")).Background(ColorSuccess).Padding(0, 1)
	cliBadgeError   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Background(ColorError).Padding(0, 1)
	cliBadgeInfo    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Background(ColorInfo).Padding(0, 1)
	cliBadgeWarning = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#000")).Background(ColorWarning).Padding(0, 1)
)

// ============================================================================
// COLOR WRITER - Wraps any io.Writer to auto-colorize output
// ============================================================================

// ColorWriter wraps an io.Writer and applies automatic colorization
type ColorWriter struct {
	underlying io.Writer
}

// NewColorWriter creates a new color-aware writer
func NewColorWriter(w io.Writer) *ColorWriter {
	return &ColorWriter{underlying: w}
}

func (cw *ColorWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	var output strings.Builder

	for i, line := range lines {
		coloredLine := colorizeLine(line)
		output.WriteString(coloredLine)
		if i < len(lines)-1 {
			output.WriteString("\n")
		}
	}

	written, err := cw.underlying.Write([]byte(output.String()))
	// Return original length to satisfy Write contract
	if err == nil {
		return len(p), nil
	}
	return written, err
}

// Regex patterns for colorization (compiled once)
var (
	reFlag        = regexp.MustCompile(`(\s)(--?[a-zA-Z][-a-zA-Z0-9]*)`)
	reCommand     = regexp.MustCompile(`(vibeaura(?:\s+[a-z]+)+)`)
	reHeader      = regexp.MustCompile(`^([A-Z][a-zA-Z ]+:)\s*$`)
	reURL         = regexp.MustCompile(`(https?://[^\s]+)`)
	reQuoted      = regexp.MustCompile(`("[^"]*")`)
	rePlaceholder = regexp.MustCompile(`(<[^>]+>)`)
	reOptional    = regexp.MustCompile(`(\[[^\]]+\])`)
)

func colorizeLine(line string) string {
	// Skip already-colored lines (contain ANSI escape codes)
	if strings.Contains(line, "\x1b[") {
		return line
	}

	// Section headers get full treatment
	if reHeader.MatchString(strings.TrimSpace(line)) {
		return lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(line)
	}

	result := line

	// Colorize flags (--help, -v, etc)
	result = reFlag.ReplaceAllStringFunc(result, func(m string) string {
		parts := reFlag.FindStringSubmatch(m)
		if len(parts) >= 3 {
			return parts[1] + lipgloss.NewStyle().Foreground(ColorNeon).Render(parts[2])
		}
		return m
	})

	// Colorize commands
	result = reCommand.ReplaceAllStringFunc(result, func(m string) string {
		return lipgloss.NewStyle().Foreground(ColorSunrise).Bold(true).Render(m)
	})

	// Colorize URLs
	result = reURL.ReplaceAllStringFunc(result, func(m string) string {
		return lipgloss.NewStyle().Foreground(ColorInfo).Underline(true).Render(m)
	})

	// Colorize quoted strings
	result = reQuoted.ReplaceAllStringFunc(result, func(m string) string {
		return lipgloss.NewStyle().Foreground(ColorAccent).Render(m)
	})

	// Colorize <placeholders>
	result = rePlaceholder.ReplaceAllStringFunc(result, func(m string) string {
		return lipgloss.NewStyle().Foreground(ColorMagic).Italic(true).Render(m)
	})

	// Colorize [optional]
	result = reOptional.ReplaceAllStringFunc(result, func(m string) string {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render(m)
	})

	return result
}

// ============================================================================
// MODULAR OUTPUT FUNCTIONS - Use these for explicit colorful output
// ============================================================================

func printTitle(emoji, title string) {
	fmt.Println()
	fmt.Println(cliTitle.Render(emoji + " " + title))
	fmt.Println(cliMuted.Render("─────────────────────────────────────────────"))
}

func printKeyValue(key, value string) {
	fmt.Printf("%s %s\n", cliLabel.Render(key+":"), cliValue.Render(value))
}

func printKeyValueHighlight(key, value string) {
	fmt.Printf("%s %s\n", cliLabel.Render(key+":"), cliHighlight.Render(value))
}

func printSuccess(message string) {
	fmt.Println(cliBadgeSuccess.Render("SUCCESS") + " " + cliSuccess.Render(message))
}

func printError(message string) {
	fmt.Println(cliBadgeError.Render("ERROR") + " " + cliError.Render(message))
}

func printInfo(message string) {
	fmt.Println(cliInfo.Render("ℹ️  " + message))
}

func printWarning(message string) {
	fmt.Println(cliWarning.Render("⚠️  " + message))
}

func printBullet(text string) {
	fmt.Println(cliBullet.Render("●") + " " + cliValue.Render(text))
}

func printBulletWithMeta(text, meta string) {
	fmt.Printf("%s %s %s\n", cliBullet.Render("●"), cliValue.Render(text), cliMuted.Render("("+meta+")"))
}

func printCommand(prefix, cmd, suffix string) {
	fmt.Println(cliInfo.Render(prefix) + " " + cliCommand.Render(cmd) + " " + cliInfo.Render(suffix))
}

func printStatus(badge, message string) {
	fmt.Println(cliBadgeInfo.Render(badge) + " " + cliValue.Render(message))
}

func printDone() {
	fmt.Println()
	fmt.Println(cliSuccess.Render("✓ Done"))
}

func printNewline() {
	fmt.Println()
}
