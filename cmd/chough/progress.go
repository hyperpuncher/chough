package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	defaultProgressBarWidth = 40
	minProgressBarWidth     = 10
)

func renderProgressBar(current, total, width int) string {
	if total <= 0 {
		return ""
	}
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s%s%s", gray, bar, reset)
}

func formatETA(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func terminalWidth() int {
	if !isStderrTTY() {
		return 0
	}
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
		return w
	}
	return 0
}

func progressBarWidthForETA(etaText string) int {
	cols := terminalWidth()
	if cols <= 0 {
		return defaultProgressBarWidth
	}
	reserved := 1 + len("ETA ") + len(etaText)
	width := cols - reserved
	if width < minProgressBarWidth {
		return minProgressBarWidth
	}
	return width
}

func renderProgressLine(current, total int, eta time.Duration) string {
	etaText := formatETA(eta)
	bar := renderProgressBar(current, total, progressBarWidthForETA(etaText))
	return fmt.Sprintf("\r%s %sETA %s%s\033[K", bar, dim, etaText, reset)
}

func renderProgressErrorLine(current, total int, eta time.Duration, err error) string {
	etaText := formatETA(eta)
	bar := renderProgressBar(current, total, progressBarWidthForETA(etaText))
	return fmt.Sprintf("\r%s %sETA %s ERR: %v%s", bar, dim, etaText, err, reset)
}

func hideCursor() {
	if isStderrTTY() {
		fmt.Fprint(os.Stderr, "\033[?25l")
	}
}

func showCursor() {
	if isStderrTTY() {
		fmt.Fprint(os.Stderr, "\033[?25h")
	}
}
