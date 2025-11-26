package echo

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func formatLogLine(log EchoLog, config ConsoleConfig) string {
	segmentMeta := joinNonEmptySegments(
		formatTimestamp(log.Time, config),
		paddedLogLevels[log.Level],
		formatPrefix(log.Prefixes),
	)

	segmentMessage := log.Message
	segmentFields := formatFields(log.Fields)
	segmentSource := formatSource(log.SourceFile, log.SourceLine, config)

	line := joinNonEmptySegments(
		segmentMeta,
		segmentMessage,
		segmentFields,
		segmentSource,
	) + "\n"

	if config.UseColor {
		return colorize(log.Level, line)
	}
	return line
}

func formatSource(file string, line int, config ConsoleConfig) string {
	if !config.ShowSource || file == "" {
		return ""
	}
	shortFile := filepath.Base(file)
	return fmt.Sprintf("(%s:%d)", shortFile, line)
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

func joinNonEmpty(values ...string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, " ")
}

func joinNonEmptySegments(segments ...string) string {
	out := make([]string, 0, len(segments))
	for _, s := range segments {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " | ")
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
