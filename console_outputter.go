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
