// Package echo provides a small utility library to facilitate logging in all my software.
package echo

import (
	"essence"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const defaultNamespaceStringRepresentation = "46cea14a-fbd4-4cbd-9a92-f291d7425f03"
const echoNamespaceStringRepresentation = "427c17ba-cdff-40ac-8da9-39aa0757968d"

var defaultNamespaceUUID essence.UUID
var echoNamespaceUUID essence.UUID

var echoMaxPrefixLength uint8 = 30
var echoApplicationPrefix string = ""

var (
	loggingConfiguration = make(map[essence.UUID]*EchoSystemConfiguration)
	loggingMu            sync.RWMutex
)

var formatTemplate string

func init() {
	namespaceUUIDGenerated, err := essence.UUIDFromString(echoNamespaceStringRepresentation)
	if err != nil {
		log.Fatalf("could not generate UUID from namespace: '%s'", echoNamespaceStringRepresentation)
	}

	echoNamespaceUUID = namespaceUUIDGenerated

	defaultNamespaceUUIDGenerated, err := essence.UUIDFromString(defaultNamespaceStringRepresentation)
	if err != nil {
		log.Fatalf("could not generate UUID from namespace: '%s'", defaultNamespaceStringRepresentation)
	}

	defaultNamespaceUUID = defaultNamespaceUUIDGenerated

	EchoSystemRegister(echoNamespaceUUID, EchoSystemConfiguration{
		MinLogLevel:    INFO,
		SystemPrefixes: []string{"Echo"},
	})

	updateFormatTemplate()
}

// LogLevel represents the level for logging.
type LogLevel uint8

const (
	TRACE    LogLevel = iota // Extremely fine-grained details used for tracing execution paths.
	DEBUG                    // Debug information useful during development and debugging.
	INFO                     // General operational information: system startup, shutdown, config loaded, etc.
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

// EchoConfigurationMaxPrefixLengthSet sets the maximum prefix length.
// Defaults to 30.
func EchoConfigurationMaxPrefixLengthSet(maxPrefixLength uint8) {
	echoMaxPrefixLength = maxPrefixLength
	updateFormatTemplate()
}

// EchoConfigurationApplicationPrefixSet sets the application prefix.
// If it is empty (default) it won't be used.
// All prefixes get concatenated with dots.
func EchoConfigurationApplicationPrefixSet(applicationPrefix string) {
	echoApplicationPrefix = applicationPrefix
}

// EchoSystemRegister registers a (sub-)system to the Echo logger.
func EchoSystemRegister(systemID essence.UUID, systemConfig EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &systemConfig
}

// EchoSystemRegisterDefault registers a configuration to use when
// the echo system is used without registration.
// In other words, when the UUID is not registered.
func EchoSystemRegisterDefault(config EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[defaultNamespaceUUID] = &config
}

// EchoSystemInternalLogLevelSet sets the log level to be used for internal messages.
// It defaults to info.
func EchoSystemInternalLogLevelSet(logLevel LogLevel) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[echoNamespaceUUID].MinLogLevel = logLevel
}

// EchoSystemConfigurationReplace replaces a given (sub-)system's configuration.
func EchoSystemConfigurationReplace(systemID essence.UUID, newConfiguration EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &newConfiguration
}

// EchoLogTrace logs highly granular diagnostic data useful for tracing specific execution paths,
// variable states, or iteration details. Typically disabled in production due to volume.
func EchoLogTrace(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(TRACE, systemID, content, forceShow)
}

// EchoLogDebug logs information helpful when debugging, such as internal state transitions,
// configuration values, or decisions made by the system. Safe to use in dev environments.
func EchoLogDebug(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(DEBUG, systemID, content, forceShow)
}

// EchoLogInfo logs standard informational messages indicating normal, expected operations such as
// startup, shutdown, or periodic reports.
func EchoLogInfo(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(INFO, systemID, content, forceShow)
}

// EchoLogNotice logs noteworthy but non-problematic events such as configuration reloads,
// cache refreshes, or graceful recoveries after transient errors.
func EchoLogNotice(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(NOTICE, systemID, content, forceShow)
}

// EchoLogWarning logs abnormal or potentially problematic situations that do not interrupt execution
// but may require observation or future corrective action.
func EchoLogWarning(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(WARNING, systemID, content, forceShow)
}

// EchoLogError logs failures that caused part of a subsystem to malfunction or abort an operation,
// but where the application remains operational.
func EchoLogError(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(ERROR, systemID, content, forceShow)
}

// EchoLogCritical logs severe conditions such as data loss, corrupted state, or unrecoverable errors.
// These typically precede program termination or emergency handling.
func EchoLogCritical(systemID essence.UUID, content string, forceShow bool) {
	echoLogGeneric(CRITICAL, systemID, content, forceShow)
}

// --------------------------------------------- PRIVATE HELPERS

//go:inline
//go:nosplit
func echoLogGeneric(level LogLevel, systemID essence.UUID, content string, forceShow bool) {
	loggingMu.RLock()
	config, ok := loggingConfiguration[systemID]
	loggingMu.RUnlock()

	if !ok {
		echoHandleMissingConfiguration(systemID, level, content, forceShow)
		return
	}
	if forceShow || echoCanLog(config, level) {
		echoLog(config.SystemPrefixes, content, level.String())
	}
}

//go:inline
//go:nosplit
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
			echoLog(defaultConfig.SystemPrefixes, content, logLevel.String())
		}
	}
}

//go:inline
//go:nosplit
func echoCanLog(config *EchoSystemConfiguration, logLevel LogLevel) bool {
	return logLevel >= config.MinLogLevel
}

//go:inline
//go:nosplit
func echoLog(prefixes []string, content, logType string) {
	currentTime := time.Now().Local().Format(time.RFC3339Nano)
	prefixesFormatted := strings.Join(prefixes, ".")
	var completePrefix string

	if echoApplicationPrefix != "" {
		completePrefix = echoApplicationPrefix + "." + prefixesFormatted
	} else {
		completePrefix = prefixesFormatted
	}

	if len(completePrefix) > int(echoMaxPrefixLength) {
		completePrefix = completePrefix[:int(echoMaxPrefixLength)]
	}

	fmt.Printf(formatTemplate, currentTime, completePrefix, logType, content)
}

func updateFormatTemplate() {
	formatTemplate = fmt.Sprintf("%%s - [%%-%ds] %%-%ds | %%s\n",
		echoMaxPrefixLength, echoMaxLogLevelStringLen())
}
