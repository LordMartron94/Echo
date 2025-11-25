package echo

import "fmt"

func formatLogLine(log EchoLog, config ConsoleConfig) string {
	timestamp := formatTimestamp(log.Time, config)
	prefix := formatPrefix(log.Prefixes)
	level := paddedLogLevels[log.Level]
	fields := formatFields(log.Fields)
	source := formatSource(log.SourceFile, log.SourceLine, config)

	msg := fmt.Sprintf("%s - [%s] %s | %s%s%s\n",
		timestamp,
		prefix,
		level,
		log.Message,
		fields,
		source,
	)

	if config.UseColor {
		msg = colorize(log.Level, msg)
	}
	return msg
}
