package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// FormatDuration formats a duration in a human-readable way with millisecond precision
func FormatDuration(d time.Duration) string {
	return FormatDurationWithConfig(d, 10) // Default: show ms under 10s
}

// FormatDurationWithConfig formats duration using configuration
func FormatDurationWithConfig(d time.Duration, showMillisecondsUnderSeconds int) string {
	if d == 0 {
		return "0ms"
	}

	// For durations less than 1 second, show in milliseconds
	if d < time.Second {
		ms := d.Milliseconds()
		return fmt.Sprintf("%dms", ms)
	}

	// For durations less than configured threshold, show seconds with 1 decimal place
	if d < time.Duration(showMillisecondsUnderSeconds)*time.Second {
		seconds := float64(d) / float64(time.Second)
		return fmt.Sprintf("%.1fs", seconds)
	}

	// For longer durations, show seconds without decimals
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	// For very long durations, show minutes and seconds
	minutes := seconds / 60
	seconds = seconds % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// FormatDurationPrecise always shows milliseconds for maximum precision
func FormatDurationPrecise(d time.Duration) string {
	ms := d.Milliseconds()

	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}

	// Show as X.XXXs for readability but keep precision
	seconds := float64(ms) / 1000.0
	return fmt.Sprintf("%.3fs", seconds)
}

// FormatPercentage formats a percentage with appropriate precision
func FormatPercentage(percentage float64) string {
	if percentage < 1 && percentage > -1 && percentage != 0 {
		return fmt.Sprintf("%.1f%%", percentage)
	}
	return fmt.Sprintf("%.0f%%", percentage)
}

// FormatTimestamp formats a time with millisecond precision
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05.000")
}

func GetStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "running", "ready":
		return "✅"
	case "pending":
		return "⏳"
	case "containercreating", "terminating":
		return "🔄"
	case "failed", "error", "crashloopbackoff":
		return "❌"
	default:
		return "❓"
	}
}

func GetDurationColor(duration time.Duration, isBottleneck bool) func(string, ...interface{}) string {
	return GetDurationColorWithConfig(duration, isBottleneck, 30, 60)
}

func GetDurationColorWithConfig(duration time.Duration, isBottleneck bool, greenSeconds, yellowSeconds int) func(string, ...interface{}) string {
	if isBottleneck {
		return color.New(color.FgRed).SprintfFunc()
	}

	if duration < time.Duration(greenSeconds)*time.Second {
		return color.New(color.FgGreen).SprintfFunc()
	} else if duration < time.Duration(yellowSeconds)*time.Second {
		return color.New(color.FgYellow).SprintfFunc()
	}
	return color.New(color.FgRed).SprintfFunc()
}
