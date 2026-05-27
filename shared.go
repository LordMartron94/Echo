package echo

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func formatLogLine(log EchoLog, config ConsoleConfig) string {
	var sb strings.Builder

	// 1. Metadata Schema
	if config.ShowTime {
		sb.WriteString(formatTimestamp(log.Time, config))
		sb.WriteString(" | ")
	}

	sb.WriteString(paddedLogLevels[log.Level])
	sb.WriteString(" | ")
	sb.WriteString(formatPrefix(log.Prefixes))
	sb.WriteString(" | ")

	// 2. Payload Schema
	sb.WriteString(formatMessageSafe(log.Message))
	sb.WriteString(" | ")
	sb.WriteString(formatFieldsSafe(log.Fields))

	// 3. Trailing Source
	if config.ShowSource {
		sb.WriteString(" | ")
		sb.WriteString(formatSourceSafe(log.SourceFile, log.SourceLine))
	}

	sb.WriteString("\n")

	if config.UseColor {
		return colorize(log.Level, sb.String())
	}
	return sb.String()
}

func formatMessageSafe(msg string) string {
	if msg == "" {
		return "-"
	}
	return msg
}

func formatFieldsSafe(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return "-"
	}
	return formatFields(fields)
}

func formatSourceSafe(file string, line int) string {
	if file == "" {
		return "-"
	}
	return fmt.Sprintf("(%s:%d)", filepath.Base(file), line)
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

func formatTimestamp(t time.Time, config ConsoleConfig) string {
	if !config.ShowTime {
		return ""
	}
	return t.Format(config.TimeLayout)
}

func formatPrefix(prefixes []string) string {
	full := strings.Join(prefixes, ".")

	if echoApplicationPrefix != "" {
		if full != "" {
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

	// Sorted keys for stable output
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder

	first := true
	for _, k := range keys {
		if !first {
			sb.WriteString(" ")
		} else {
			first = false
		}

		val := fields[k]
		if s, ok := val.(string); ok && strings.Contains(s, " ") {
			fmt.Fprintf(&sb, "%s=\"%v\"", k, s)
		} else {
			fmt.Fprintf(&sb, "%s=%v", k, val)
		}
	}

	return sb.String()
}
