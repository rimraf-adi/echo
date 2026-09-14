package output

import (
	"fmt"
	"os"
	"time"
)

var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		colorReset = ""
		colorRed = ""
		colorGreen = ""
		colorYellow = ""
		colorBlue = ""
		colorCyan = ""
		colorBold = ""
	}
}

// Green returns text wrapped in green terminal color
func Green(s string) string {
	return colorGreen + s + colorReset
}

// Red returns text wrapped in red terminal color
func Red(s string) string {
	return colorRed + s + colorReset
}

// Yellow returns text wrapped in yellow terminal color
func Yellow(s string) string {
	return colorYellow + s + colorReset
}

// Cyan returns text wrapped in cyan terminal color
func Cyan(s string) string {
	return colorCyan + s + colorReset
}

// Bold returns text wrapped in bold
func Bold(s string) string {
	return colorBold + s + colorReset
}

// RelativeTime formats a past timestamp into a human-friendly string (e.g. "2 minutes ago")
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < 2*time.Minute {
		return "1 minute ago"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	if d < 2*time.Hour {
		return "1 hour ago"
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", days)
}

// FormatBytes formats byte sizes into readable units
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
