package echo

import (
	"fmt"
	"io"
	"os"

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

/*
ConsoleConfig configures the console output format and behavior.

ConsoleConfig controls how log entries are formatted and displayed in console
output. It supports color coding, timestamp display, source location, and
custom time formatting.

Use cases:
- Customizing console output for different environments
- Disabling colors for non-terminal output
- Controlling verbosity of log output
- Custom time format requirements
*/
type ConsoleConfig struct {
	UseColor   bool
	ShowTime   bool
	ShowSource bool
	TimeLayout string
}

/*
DefaultConsoleConfigCreate returns a sensible default console configuration.

This function creates a ConsoleConfig with sensible defaults: colors enabled
if terminal supports them, timestamps enabled, source location disabled, and
ISO 8601 time format.

Use cases:
- Quick setup with reasonable defaults
- Starting point for custom configurations
- Standard console output formatting

Time complexity: O(1) - struct initialization
Space complexity: O(1) - returns struct by value

Prerequisites:
- None

Edge cases:
- Color detection respects NO_COLOR and FORCE_COLOR environment variables
- Automatically detects terminal capabilities
- Returns configuration by value (safe to modify)
*/
func DefaultConsoleConfigCreate() ConsoleConfig {
	return ConsoleConfig{
		UseColor:   shouldEnableColors(),
		ShowTime:   true,
		ShowSource: false,
		TimeLayout: echoTimeLayout,
	}
}

/*
EchoConsoleOutputterCreate creates a LogHook that writes formatted log entries to a writer.

This function creates a LogHook that formats log entries according to the provided
ConsoleConfig and writes them to the specified writer. The outputter performs lazy
evaluation of expensive operations (source location, field merging) only when the
log will actually be displayed.

Use cases:
- Console output to os.Stdout or os.Stderr
- Custom writer destinations (buffers, network connections)
- Formatted log output with colors and timestamps
- Standard console logging

Time complexity: O(1) - returns function closure
Space complexity: O(1) - captures writer and config in closure

Prerequisites:
- w should be a valid io.Writer (e.g., os.Stdout, os.Stderr)
- config should be a valid ConsoleConfig

Edge cases:
- Respects log.CanShow and log.ForceShow flags
- Performs lazy evaluation of source and fields
- Thread-safe: each hook call is independent
- Writer should handle concurrent writes if used from multiple goroutines
*/
func EchoConsoleOutputterCreate(w io.Writer, config ConsoleConfig) LogHook {
	return func(log EchoLog) {
		if !log.ForceShow && !log.CanShow {
			return
		}

		// Lazy evaluation: only get expensive information when we actually need to output
		EchoLogGetSource(&log)
		EchoLogGetMergedFields(&log)

		msg := formatLogLine(log, config)
		fmt.Fprint(w, msg)
	}
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
