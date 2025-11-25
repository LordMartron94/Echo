package echo

import (
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	formatTemplate string
	colorEnabled   bool
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
	term := os.Getenv("TERM")
	noColor := os.Getenv("NO_COLOR")
	colorEnabled = (term != "dumb" && noColor == "")

	// Register the internal Echo system after setting the default template.
	updateFormatTemplate()
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

func ConsoleOutputterCreate() LogOutputter {
	return func(log EchoLog) {
		if log.forceShow || log.canShow {
			echoLog(log.prefixes, log.message, log.level.String(), log.level)
		}
	}
}

//go:inline
func echoLog(prefixes []string, content, logType string, level LogLevel) {
	currentTime := time.Now().Local().Format(echoTimeLayout)

	prefixesFormatted := strings.Join(prefixes, ".")
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
		completePrefix = completePrefix[:int(echoMaxPrefixLength)]
	}

	logType = fmt.Sprintf("%-*s", echoMaxLogLevelStringLen(), logType)

	colorStart := ""
	colorEnd := ""
	if colorEnabled {
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
		colorEnd = colorReset
	}

	line := fmt.Sprintf(formatTemplate, currentTime, completePrefix, logType, content)
	if colorEnabled {
		fmt.Printf("%s%s%s", colorStart, line, colorEnd)
	} else {
		fmt.Print(line)
	}
}
