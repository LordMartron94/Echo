// Package echo provides a small utility library to facilitate logging in all my software.
package echo

import (
	"essence"
	"fmt"
	"log"
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
)

func init() {
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
// Public Hooks
// ---------------------------------------------------------------------------

// EchoLog represents a single log entry used for processing inside custom hooks.
type EchoLog struct {
	Level     LogLevel
	Message   string
	ForceShow bool
	CanShow   bool
	Id        essence.UUID
	Time      time.Time
	Prefixes  []string
}

// LogHook represents a hook that processes a log and outputs it in a certain way.
type LogHook func(log EchoLog)

var logHooks = make([]LogHook, 0)
var logHookMu = &sync.RWMutex{}

// EchoLogHookRegister registers a hook executing on every log.
func EchoLogHookRegister(outputter LogHook) {
	logHookMu.Lock()
	logHooks = append(logHooks, outputter)
	logHookMu.Unlock()
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
	processLog(config, systemID, level, content, forceShow)
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
		processLog(defaultConfig, systemID, logLevel, content, forceShow)
	}
}

//go:inline
func processLog(config *EchoSystemConfiguration, systemID essence.UUID, logLevel LogLevel, content string, forceShow bool) {
	canLog := echoCanLog(config, logLevel)
	log := EchoLog{
		Level:     logLevel,
		Message:   content,
		ForceShow: forceShow,
		CanShow:   canLog,
		Id:        systemID,
		Time:      time.Now().Local(),
		Prefixes:  config.SystemPrefixes,
	}

	logHookMu.RLock()
	hooks := append([]LogHook(nil), logHooks...)
	logHookMu.RUnlock()

	for _, hook := range hooks {
		hook(log)
	}
}

//go:inline
func echoCanLog(config *EchoSystemConfiguration, logLevel LogLevel) bool {
	return logLevel >= config.MinLogLevel
}
