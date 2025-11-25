package echo

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

var (
	formatTemplate string
	colorEnabled   bool

	paddedLogLevels []string
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

const echoTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func init() {
	colorEnabled = shouldEnableColors()

	paddedLogLevels = make([]string, CRITICAL+1)
	for lvl := TRACE; lvl <= CRITICAL; lvl++ {
		paddedLogLevels[lvl] = fmt.Sprintf("%-*s", echoMaxLogLevelStringLen(), lvl.String())
	}

	// Register the internal Echo system after setting the default template.
	updateFormatTemplate()
}

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

// updateFormatTemplate rebuilds the printf format string for all future logs.
// It is called automatically whenever relevant parameters change.
func updateFormatTemplate() {
	formatTemplate = fmt.Sprintf("%%s - [%%-%ds] %%-%ds | %%s\n",
		echoMaxPrefixLength, echoMaxLogLevelStringLen())
}

func echoMaxLogLevelStringLen() int {
	max := 0
	for lvl := TRACE; lvl <= CRITICAL; lvl++ {
		if n := len(lvl.String()); n > max {
			max = n
		}
	}
	return max
}

func ConsoleOutputterCreate() LogHook {
	return func(log EchoLog) {
		if log.ForceShow || log.CanShow {
			echoLog(log)
		}
	}
}

//go:inline
func echoLog(log EchoLog) {
	currentTime := log.Time.Format(echoTimeLayout)

	prefixesFormatted := strings.Join(log.Prefixes, ".")
	var completePrefix string
	if echoApplicationPrefix != "" {
		completePrefix = echoApplicationPrefix
		if len(prefixesFormatted) > 0 {
			completePrefix += "." + prefixesFormatted
		}
	} else {
		completePrefix = prefixesFormatted
	}

	if len(completePrefix) > int(echoMaxPrefixLength) {
		EchoLogWarning(echoNamespaceUUID, fmt.Sprintf("identifier longer than limit: got=%v,max=%v; truncating prefix", len(completePrefix), echoMaxPrefixLength), true)
		completePrefix = completePrefix[:int(echoMaxPrefixLength)]
	}

	colorStart := ""
	colorEnd := ""
	if colorEnabled {
		switch log.Level {
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
		colorEnd = colorReset
	}

	logType := paddedLogLevels[log.Level]

	line := fmt.Sprintf(formatTemplate, currentTime, completePrefix, logType, log.Message)
	if colorEnabled {
		fmt.Printf("%s%s%s", colorStart, line, colorEnd)
	} else {
		fmt.Print(line)
	}
}
