// Package echo provides a lightweight, flexible, and structured logging library.
// It supports context-aware logging, structured fields, middleware (processors),
// and multiple output hooks.
//
// Usage:
//
//	// 1. Simple logging (Legacy/Wrapper style)
//	echo.EchoLogInfo(mySystemID, "System started", false)
//
//	// 2. Fluent/Builder logging (Recommended)
//	echo.On(mySystemID).
//	    Ctx(ctx).
//	    Field("user_id", 123).
//	    Info("User logged in")
package echo

import (
	"context"
	"essence"
	"fmt"
	"log"
	"os"
	"runtime"
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

	// internalPrefixes defines packages to skip during source detection.
	internalPrefixes = []string{"echo"}
)

func init() {
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

// LogLevel represents the severity of a log entry.
type LogLevel uint8

const (
	// TRACE is for extremely fine-grained details used for tracing execution paths.
	// Typically disabled in production.
	TRACE LogLevel = iota

	// DEBUG is for information useful during development and debugging.
	DEBUG

	// INFO is for general operational information: startup, shutdown, config loaded, etc.
	INFO

	// NOTICE is for noteworthy but expected events: successful reloads, recoveries, or transitions.
	NOTICE

	// WARNING is for something unexpected that occurred, but the system can continue running safely.
	WARNING

	// ERROR is for when an error occurred that affected functionality; likely needs developer attention.
	ERROR

	// CRITICAL is for severe errors that compromise program stability; immediate attention required.
	CRITICAL
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

// EchoSystemConfiguration defines the configuration (Log Level, Prefixes) for a specific (sub-)system.
type EchoSystemConfiguration struct {
	// MinLogLevel determines the minimum severity required for a log to be processed.
	MinLogLevel LogLevel
	// SystemPrefixes are strings prefixed to logs to identify the subsystem (e.g., "Auth", "DB").
	SystemPrefixes []string
}

// ---------------------------------------------------------------------------
// Configuration Functions
// ---------------------------------------------------------------------------

// EchoConfigurationMaxPrefixLengthSet sets the maximum visual length for prefixes in formatted output.
// It automatically recalculates alignment templates.
func EchoConfigurationMaxPrefixLengthSet(maxPrefixLength uint8) {
	echoMaxPrefixLength = maxPrefixLength
}

// EchoConfigurationApplicationPrefixSet sets a global application prefix (e.g., "MyApp").
// If empty, it is ignored. Updates formatting templates automatically.
func EchoConfigurationApplicationPrefixSet(applicationPrefix string) {
	echoApplicationPrefix = applicationPrefix
}

// EchoSystemRegister registers a configuration for a specific system UUID.
// If a system is not registered, it falls back to the Default configuration.
func EchoSystemRegister(systemID essence.UUID, systemConfig EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &systemConfig
}

// EchoSystemRegisterDefault registers the fallback configuration used when
// a system UUID has not been explicitly registered.
func EchoSystemRegisterDefault(config EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[defaultNamespaceUUID] = &config
}

// EchoSystemInternalLogLevelSet sets the log level for the Echo library's own internal messages.
// Defaults to INFO.
func EchoSystemInternalLogLevelSet(logLevel LogLevel) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	if cfg, ok := loggingConfiguration[echoNamespaceUUID]; ok {
		cfg.MinLogLevel = logLevel
	}
}

// EchoSystemConfigurationReplace hot-swaps the configuration for a given system ID.
// Thread-safe.
func EchoSystemConfigurationReplace(systemID essence.UUID, newConfiguration EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &newConfiguration
}

// EchoInternalPrefixRegister registers a package path prefix to be treated as "internal".
// Internal frames are skipped when detecting the source file/line number of a log.
// Usage Hint: Use this if you wrap Echo in another utility library.
func EchoInternalPrefixRegister(prefix string) {
	internalPrefixes = append(internalPrefixes, prefix)
}

// ---------------------------------------------------------------------------
// Hooks & Processors
// ---------------------------------------------------------------------------

// EchoLog represents a single log entry, containing all metadata, structure, and context.
type EchoLog struct {
	Level      LogLevel
	Message    string
	ForceShow  bool
	CanShow    bool
	Id         essence.UUID
	Time       time.Time
	Prefixes   []string
	Fields     map[string]interface{} // Structured key-value pairs
	SourceFile string                 // File path where the log originated
	SourceLine int                    // Line number where the log originated
}

// LogHook is a function that outputs a log (e.g., to Console, File, Splunk, Database).
type LogHook func(log EchoLog)

// LogProcessor is a middleware function that modifies a log *before* it is sent to hooks.
// Usage Hint: Use this for masking PII or injecting global environment variables.
type LogProcessor func(log *EchoLog)

// ContextExtractor is a function that extracts structured data from a context.Context.
// Usage Hint: Use this to auto-extract TraceIDs or RequestIDs.
type ContextExtractor func(ctx context.Context) map[string]interface{}

var (
	logHooks          = make([]LogHook, 0)
	logProcessors     = make([]LogProcessor, 0)
	contextExtractors = make([]ContextExtractor, 0)
	hookMu            sync.RWMutex
)

// EchoLogHookRegister registers a new output destination.
// Hooks are executed synchronously in the order they were registered.
func EchoLogHookRegister(outputter LogHook) {
	hookMu.Lock()
	defer hookMu.Unlock()
	logHooks = append(logHooks, outputter)
}

// EchoLogProcessorRegister registers a middleware processor.
// Processors run before Hooks and can modify the EchoLog object.
func EchoLogProcessorRegister(processor LogProcessor) {
	hookMu.Lock()
	defer hookMu.Unlock()
	logProcessors = append(logProcessors, processor)
}

// EchoContextExtractorRegister registers a function that pulls data from context.Context.
// These run when .Ctx() is used in the builder.
func EchoContextExtractorRegister(extractor ContextExtractor) {
	hookMu.Lock()
	defer hookMu.Unlock()
	contextExtractors = append(contextExtractors, extractor)
}

// ---------------------------------------------------------------------------
// Fluent Interface (Builder Pattern)
// ---------------------------------------------------------------------------

// EchoEvent is a builder struct for constructing rich, context-aware log entries.
type EchoEvent struct {
	id        essence.UUID
	ctx       context.Context
	fields    map[string]interface{}
	forceShow bool
}

// On initiates a fluent logging chain for the given system ID.
//
// Usage:
//
//	echo.On(sysID).Ctx(ctx).Info("Message")
func On(systemID essence.UUID) *EchoEvent {
	return &EchoEvent{
		id:     systemID,
		fields: make(map[string]interface{}),
	}
}

// Ctx attaches a context to the log entry.
// Registered ContextExtractors will run against this context to populate log fields.
func (e *EchoEvent) Ctx(ctx context.Context) *EchoEvent {
	e.ctx = ctx
	return e
}

// Field adds a single key-value pair to the structured log data.
func (e *EchoEvent) Field(key string, value interface{}) *EchoEvent {
	e.fields[key] = value
	return e
}

// Fields adds a map of key-value pairs to the structured log data.
func (e *EchoEvent) Fields(fields map[string]interface{}) *EchoEvent {
	for k, v := range fields {
		e.fields[k] = v
	}
	return e
}

// Force flags the log to be displayed regardless of the current Minimum Log Level configuration.
// Usage Hint: Use cautiously for debugging specific paths in production without enabling global Debug.
func (e *EchoEvent) Force() *EchoEvent {
	e.forceShow = true
	return e
}

// --- Terminators (Output) ---

// Trace finalizes the event and logs it at TRACE level.
//
// Use TRACE for extremely fine-grained information, such as loop iterations,
// full payload dumps, or detailed step-by-step execution flows.
//
// Recommendation: Keep disabled in production unless diagnosing a specific,
// hard-to-reproduce issue due to high volume and performance impact.
func (e *EchoEvent) Trace(content string) {
	echoLogInternal(TRACE, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Debug finalizes the event and logs it at DEBUG level.
//
// Use DEBUG for diagnostic information useful to developers, such as
// internal state snapshots, variable values, or boolean logic results.
//
// Recommendation: Enable in development/staging. In production, this is usually
// disabled to reduce noise, but safe to enable temporarily for troubleshooting.
func (e *EchoEvent) Debug(content string) {
	echoLogInternal(DEBUG, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Info finalizes the event and logs it at INFO level.
//
// Use INFO for general operational milestones that confirm the system is working
// as expected (e.g., "Service started", "Job completed", "Connection established").
//
// Recommendation: This is the standard log level for production environments.
// Messages here should be meaningful to operators monitoring the system health.
func (e *EchoEvent) Info(content string) {
	echoLogInternal(INFO, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Notice finalizes the event and logs it at NOTICE level.
//
// Use NOTICE for events that are unusual or significant but strictly non-error
// conditions (e.g., "Configuration reloaded", "Auto-healing triggered", "Switching to backup").
//
// Recommendation: Use this to flag events that are "normal" but rare enough
// that a human administrator might want to be aware of them.
func (e *EchoEvent) Notice(content string) {
	echoLogInternal(NOTICE, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Warning finalizes the event and logs it at WARNING level.
//
// Use WARNING for unexpected situations that do not halt execution or fail a
// request, but may indicate a potential problem (e.g., "Slow query",
// "Disk 85% full", "Deprecated API usage", "Retryable network error").
//
// Recommendation: These logs should be monitored to prevent future errors,
// but they do not require immediate middle-of-the-night intervention.
func (e *EchoEvent) Warning(content string) {
	echoLogInternal(WARNING, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Error finalizes the event and logs it at ERROR level.
//
// Use ERROR when a specific operation or request has failed and cannot be
// completed (e.g., "Database commit failed", "File not found", "HTTP 500").
//
// Recommendation: These indicate a bug or an infrastructure issue that
// likely affects the user experience and requires developer attention.
func (e *EchoEvent) Error(content string) {
	echoLogInternal(ERROR, e.id, content, e.fields, e.ctx, e.forceShow)
}

// Critical finalizes the event and logs it at CRITICAL level.
//
// Use CRITICAL for severe failures that compromise the stability or integrity
// of the entire system (e.g., "Data corruption detected", "Security breach",
// "Main loop panic", "Cannot connect to primary database").
//
// Recommendation: These events imply the application is unusable or dangerous.
// They should trigger immediate alerts (pages) to on-call staff.
func (e *EchoEvent) Critical(content string) {
	echoLogInternal(CRITICAL, e.id, content, e.fields, e.ctx, e.forceShow)
}

// ---------------------------------------------------------------------------
// Public Log Functions (Legacy / Simple Wrappers)
// ---------------------------------------------------------------------------

// EchoLogTrace logs a message at TRACE level.
// It is a convenience wrapper for echo.On(id).Trace(content).
func EchoLogTrace(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(TRACE, systemID, content, nil, nil, forceShow)
}

// EchoLogDebug logs a message at DEBUG level.
// It is a convenience wrapper for echo.On(id).Debug(content).
func EchoLogDebug(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(DEBUG, systemID, content, nil, nil, forceShow)
}

// EchoLogInfo logs a message at INFO level.
// It is a convenience wrapper for echo.On(id).Info(content).
func EchoLogInfo(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(INFO, systemID, content, nil, nil, forceShow)
}

// EchoLogNotice logs a message at NOTICE level.
// It is a convenience wrapper for echo.On(id).Notice(content).
func EchoLogNotice(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(NOTICE, systemID, content, nil, nil, forceShow)
}

// EchoLogWarning logs a message at WARNING level.
// It is a convenience wrapper for echo.On(id).Warning(content).
func EchoLogWarning(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(WARNING, systemID, content, nil, nil, forceShow)
}

// EchoLogError logs a message at ERROR level.
// It is a convenience wrapper for echo.On(id).Error(content).
func EchoLogError(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(ERROR, systemID, content, nil, nil, forceShow)
}

// EchoLogCritical logs a message at CRITICAL level.
// It is a convenience wrapper for echo.On(id).Critical(content).
func EchoLogCritical(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(CRITICAL, systemID, content, nil, nil, forceShow)
}

// ---------------------------------------------------------------------------
// Internal Logic
// ---------------------------------------------------------------------------

//go:inline
func echoLogInternal(level LogLevel, systemID essence.UUID, content string, fields map[string]interface{}, ctx context.Context, forceShow bool) {
	loggingMu.RLock()
	config, ok := loggingConfiguration[systemID]
	loggingMu.RUnlock()

	if !ok {
		echoHandleMissingConfiguration(systemID, level, content, forceShow)
		return
	}
	processLog(config, systemID, level, content, fields, ctx, forceShow)
}

//go:inline
func echoHandleMissingConfiguration(systemID essence.UUID, logLevel LogLevel, content string, forceShow bool) {
	loggingMu.RLock()
	defaultConfig, defaultOk := loggingConfiguration[defaultNamespaceUUID]
	loggingMu.RUnlock()

	if !defaultOk {
		msg := fmt.Sprintf("Missing config for '%s' and missing default config", systemID.String())
		fmt.Fprintf(os.Stderr, "ECHO CRITICAL FAILURE: %s\n", msg)
	} else {
		// Log a debug message about the fallback, then proceed
		EchoLogDebug(echoNamespaceUUID, fmt.Sprintf("Config missing for '%s', using default", systemID.String()), false)
		processLog(defaultConfig, systemID, logLevel, content, nil, nil, forceShow)
	}
}

//go:inline
func processLog(config *EchoSystemConfiguration, systemID essence.UUID, logLevel LogLevel, content string, fields map[string]interface{}, ctx context.Context, forceShow bool) {
	canLog := echoCanLog(config, logLevel)

	finalFields := mergeFieldsAndContext(fields, ctx)
	file, line := captureSource()

	logEntry := EchoLog{
		Level:      logLevel,
		Message:    content,
		ForceShow:  forceShow,
		CanShow:    canLog,
		Id:         systemID,
		Time:       time.Now().Local(),
		Prefixes:   config.SystemPrefixes,
		Fields:     finalFields,
		SourceFile: file,
		SourceLine: line,
	}

	applyProcessors(&logEntry)
	dispatchToHooks(logEntry)
}

//go:inline
func captureSource() (string, int) {
	const maxDepth = 32
	var pcs [maxDepth]uintptr

	// Skip 2: runtime.Callers + captureSource
	n := runtime.Callers(2, pcs[:])
	if n == 0 {
		return "unknown", 0
	}

	frames := runtime.CallersFrames(pcs[:n])

	for {
		frame, more := frames.Next()

		if !isInternalFrame(frame.Function) {
			return frame.File, frame.Line
		}

		if !more {
			break
		}
	}

	return "unknown", 0
}

//go:inline
func isInternalFrame(fn string) bool {
	if fn == "" {
		return true
	}

	for _, p := range internalPrefixes {
		if strings.Contains(fn, p) {
			return true
		}
	}
	return false
}

//go:inline
func mergeFieldsAndContext(fields map[string]interface{}, ctx context.Context) map[string]interface{} {
	// Initialize with a capacity hint if possible, but map resizing is efficient enough for this scale.
	finalFields := make(map[string]interface{})

	// 1. Copy manual fields
	for k, v := range fields {
		finalFields[k] = v
	}

	// 2. Extract context fields (Global Processors)
	if ctx != nil {
		hookMu.RLock()
		extractors := append([]ContextExtractor(nil), contextExtractors...)
		hookMu.RUnlock()

		for _, extractor := range extractors {
			ctxFields := extractor(ctx)
			for k, v := range ctxFields {
				finalFields[k] = v
			}
		}
	}
	return finalFields
}

//go:inline
func applyProcessors(logEntry *EchoLog) {
	hookMu.RLock()
	processors := append([]LogProcessor(nil), logProcessors...)
	hookMu.RUnlock()

	for _, p := range processors {
		p(logEntry)
	}
}

//go:inline
func dispatchToHooks(logEntry EchoLog) {
	hookMu.RLock()
	hooks := append([]LogHook(nil), logHooks...)
	hookMu.RUnlock()

	for _, hook := range hooks {
		safeExecuteHook(hook, logEntry)
	}
}

func safeExecuteHook(hook LogHook, logEntry EchoLog) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Echo: hook panicked: %v\n", r)
		}
	}()
	hook(logEntry)
}

//go:inline
func echoCanLog(config *EchoSystemConfiguration, logLevel LogLevel) bool {
	return logLevel >= config.MinLogLevel
}
