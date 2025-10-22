// Package echo provides a small utility library to facilitate logging in all my software.
package echo

import (
	"essence"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultNamespaceStringRepresentation = "46cea14a-fbd4-4cbd-9a92-f291d7425f03"
	echoNamespaceStringRepresentation    = "427c17ba-cdff-40ac-8da9-39aa0757968d"
)

var (
	defaultNamespaceUUID essence.UUID
	echoNamespaceUUID    essence.UUID

	echoMaxPrefixLength   uint8  = 30
	echoApplicationPrefix string = ""

	loggingConfiguration = make(map[essence.UUID]*EchoSystemConfiguration)
	loggingMu            sync.RWMutex

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

func init() {
	// Determine if colors should be enabled.
	term := os.Getenv("TERM")
	noColor := os.Getenv("NO_COLOR")
	colorEnabled = (term != "dumb" && noColor == "")

	// Initialize UUIDs first.
	var err error
	echoNamespaceUUID, err = essence.UUIDFromString(echoNamespaceStringRepresentation)
	if err != nil {
		log.Fatalf("could not generate UUID from namespace: '%s'", echoNamespaceStringRepresentation)
	}

	defaultNamespaceUUID, err = essence.UUIDFromString(defaultNamespaceStringRepresentation)
	if err != nil {
		log.Fatalf("could not generate UUID from namespace: '%s'", defaultNamespaceStringRepresentation)
	}

	// Register the internal Echo system after setting the default template.
	updateFormatTemplate()

	EchoSystemRegister(echoNamespaceUUID, EchoSystemConfiguration{
		MinLogLevel:    INFO,
		SystemPrefixes: []string{"Echo"},
	})
}

// LogLevel represents the level for logging.
type LogLevel uint8

const (
	TRACE    LogLevel = iota // Extremely fine-grained details used for tracing execution paths.
	DEBUG                    // Debug information useful during development and debugging.
	INFO                     // General operational information: startup, shutdown, config loaded, etc.
	NOTICE                   // Noteworthy but expected events: successful reloads, recoveries, or transitions.
	WARNING                  // Something unexpected occurred but the system can continue running safely.
	ERROR                    // An error occurred that affected functionality; likely needs developer attention.
	CRITICAL                 // Severe error that compromises program stability; immediate attention required.
)

func (l LogLevel) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case NOTICE:
		return "NOTICE"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "unknown"
	}
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

// EchoSystemConfiguration defines the configuration to use for a (sub-)system.
type EchoSystemConfiguration struct {
	MinLogLevel    LogLevel
	SystemPrefixes []string
}

// EchoConfigurationMaxPrefixLengthSet sets the maximum prefix length and
// automatically updates internal alignment templates.
func EchoConfigurationMaxPrefixLengthSet(maxPrefixLength uint8) {
	echoMaxPrefixLength = maxPrefixLength
	updateFormatTemplate()
}

// EchoConfigurationApplicationPrefixSet sets the application prefix.
// If it is empty (default) it won't be used. Automatically updates formatting.
func EchoConfigurationApplicationPrefixSet(applicationPrefix string) {
	echoApplicationPrefix = applicationPrefix
	updateFormatTemplate()
}

// EchoSystemRegister registers a (sub-)system to the Echo logger.
func EchoSystemRegister(systemID essence.UUID, systemConfig EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &systemConfig
}

// EchoSystemRegisterDefault registers a configuration to use when
// the echo system is used without registration (UUID not registered).
func EchoSystemRegisterDefault(config EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[defaultNamespaceUUID] = &config
}

// EchoSystemInternalLogLevelSet sets the log level to be used for internal messages.
// It defaults to INFO.
func EchoSystemInternalLogLevelSet(logLevel LogLevel) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	if cfg, ok := loggingConfiguration[echoNamespaceUUID]; ok {
		cfg.MinLogLevel = logLevel
	}
}

// EchoSystemConfigurationReplace replaces a given (sub-)system's configuration.
func EchoSystemConfigurationReplace(systemID essence.UUID, newConfiguration EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &newConfiguration
}

// ---------------------------------------------------------------------------
// Public Log Functions
// ---------------------------------------------------------------------------

// EchoLogTrace logs highly granular diagnostic data useful for tracing specific execution paths.
// Typically disabled in production due to volume.
func EchoLogTrace(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(TRACE, systemID, content, forceShow)
}

// EchoLogDebug logs information helpful during debugging.
// Safe to use in development and staging environments.
func EchoLogDebug(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(DEBUG, systemID, content, forceShow)
}

// EchoLogInfo logs standard informational messages such as startup, shutdown, or periodic reports.
func EchoLogInfo(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(INFO, systemID, content, forceShow)
}

// EchoLogNotice logs noteworthy but non-problematic events such as config reloads or recoveries.
func EchoLogNotice(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(NOTICE, systemID, content, forceShow)
}

// EchoLogWarning logs abnormal or potentially problematic situations
// that do not interrupt execution but may require attention.
func EchoLogWarning(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(WARNING, systemID, content, forceShow)
}

// EchoLogError logs failures that caused part of a subsystem to malfunction
// but where the application remains operational.
func EchoLogError(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(ERROR, systemID, content, forceShow)
}

// EchoLogCritical logs severe conditions such as unrecoverable errors or data loss.
// These typically precede program termination or emergency handling.
func EchoLogCritical(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(CRITICAL, systemID, content, forceShow)
}

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

//go:inline
func echoLogGeneric(level LogLevel, systemID essence.UUID, content string, forceShow bool) {
	loggingMu.RLock()
	config, ok := loggingConfiguration[systemID]
	loggingMu.RUnlock()

	if !ok {
		echoHandleMissingConfiguration(systemID, level, content, forceShow)
		return
	}
	if forceShow || echoCanLog(config, level) {
		echoLog(config.SystemPrefixes, content, level.String(), level)
	}
}

//go:inline
func echoHandleMissingConfiguration(systemID essence.UUID, logLevel LogLevel, content string, forceShow bool) {
	defaultMsg := fmt.Sprintf("could not find registered configuration '%s'; resorting to default", systemID.String())
	EchoLogDebug(echoNamespaceUUID, defaultMsg, false)

	loggingMu.RLock()
	defaultConfig, defaultOk := loggingConfiguration[defaultNamespaceUUID]
	loggingMu.RUnlock()

	missingMsg := fmt.Sprintf("could not find default configuration '%s'; skipping log", systemID.String())
	if !defaultOk {
		EchoLogWarning(echoNamespaceUUID, missingMsg, false)
	} else {
		if forceShow || echoCanLog(defaultConfig, logLevel) {
			echoLog(defaultConfig.SystemPrefixes, content, logLevel.String(), logLevel)
		}
	}
}

//go:inline
func echoCanLog(config *EchoSystemConfiguration, logLevel LogLevel) bool {
	return logLevel >= config.MinLogLevel
}

const echoTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

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

// updateFormatTemplate rebuilds the printf format string for all future logs.
// It is called automatically whenever relevant parameters change.
func updateFormatTemplate() {
	formatTemplate = fmt.Sprintf("%%s - [%%-%ds] %%-%ds | %%s\n",
		echoMaxPrefixLength, echoMaxLogLevelStringLen())
}
