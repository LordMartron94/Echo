package echo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// ANSI color codes
const (
	colorReset    = "\033[0m"
	colorGray     = "\033[90m"
	colorBlue     = "\033[94m"
	colorCyan     = "\033[36m"
	colorGreen    = "\033[32m"
	colorYellow   = "\033[33m"
	colorRed      = "\033[31m"
	colorBoldRed  = "\033[1;31m"
	colorCritical = "\033[97;41m" // bright white text, red background
)

const echoTimeLayout = "2006-01-02T15:04:05.000Z07:00"

var paddedLogLevels []string

func init() {
	// Pre-compute padded log levels for alignment
	paddedLogLevels = make([]string, CRITICAL+1)
	maxLen := 0
	for lvl := TRACE; lvl <= CRITICAL; lvl++ {
		str := lvl.String()
		if len(str) > maxLen {
			maxLen = len(str)
		}
	}
	for lvl := TRACE; lvl <= CRITICAL; lvl++ {
		paddedLogLevels[lvl] = fmt.Sprintf("%-*s", maxLen, lvl.String())
	}
}

// ConsoleConfig allows tweaking the output format per logger instance.
type ConsoleConfig struct {
	UseColor   bool
	ShowTime   bool
	ShowSource bool
	TimeLayout string
}

// DefaultConsoleConfigCreate returns a sensible default configuration.
func DefaultConsoleConfigCreate() ConsoleConfig {
	return ConsoleConfig{
		UseColor:   shouldEnableColors(),
		ShowTime:   true,
		ShowSource: false,
		TimeLayout: echoTimeLayout,
	}
}

// EchoConsoleOutputterCreate creates a LogHook that writes to the provided writer (usually os.Stdout).
func EchoConsoleOutputterCreate(w io.Writer, config ConsoleConfig) LogHook {
	return func(log EchoLog) {
		if !log.ForceShow && !log.CanShow {
			return
		}

		msg := formatLogLine(log, config)
		fmt.Fprint(w, msg)
	}
}

// ---------------------------------------------------------------------------
// Formatting Helpers
// ---------------------------------------------------------------------------

func formatTimestamp(t time.Time, config ConsoleConfig) string {
	if !config.ShowTime {
		return ""
	}
	return t.Format(config.TimeLayout)
}

func formatPrefix(prefixes []string) string {
	full := strings.Join(prefixes, ".")
	if echoApplicationPrefix != "" {
		if len(full) > 0 {
			full = echoApplicationPrefix + "." + full
		} else {
			full = echoApplicationPrefix
		}
	}

	max := int(echoMaxPrefixLength)
	if len(full) > max {
		if max > 3 {
			return full[:max-3] + "..."
		}
		return full[:max]
	}

	return fmt.Sprintf("%-*s", max, full)
}

func formatFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}

	// 1. Extract keys to sort them (ensure deterministic output)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Build string
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(" ")

		val := fields[k]

		if s, ok := val.(string); ok && strings.Contains(s, " ") {
			fmt.Fprintf(&sb, "%s=\"%v\"", k, s)
		} else {
			fmt.Fprintf(&sb, "%s=%v", k, val)
		}
	}
	return sb.String()
}

func formatSource(file string, line int, config ConsoleConfig) string {
	if !config.ShowSource || file == "" {
		return ""
	}
	shortFile := filepath.Base(file)
	return fmt.Sprintf(" (%s:%d)", shortFile, line)
}

func colorize(level LogLevel, line string) string {
	colorStart := ""
	switch level {
	case TRACE:
		colorStart = colorGray
	case DEBUG:
		colorStart = colorCyan
	case INFO:
		colorStart = colorGreen
	case NOTICE:
		colorStart = colorBlue
	case WARNING:
		colorStart = colorYellow
	case ERROR:
		colorStart = colorBoldRed
	case CRITICAL:
		colorStart = colorCritical
	}
	return fmt.Sprintf("%s%s%s", colorStart, line, colorReset)
}

// shouldEnableColors detects if the terminal supports color.
func shouldEnableColors() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
