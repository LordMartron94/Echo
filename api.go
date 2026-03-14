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
	"foundation"
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

/*
EchoSystemConfiguration defines the configuration for a specific system or subsystem.

The configuration controls log level filtering and prefix display for logs originating
from a registered system UUID. Each system can have independent log level thresholds
and display prefixes.

Use cases:
- Per-module log level control (e.g., verbose auth logs, minimal DB logs)
- System identification in log output via prefixes
- Hierarchical logging (application.system.subsystem)
*/
type EchoSystemConfiguration struct {
	// MinLogLevel determines the minimum severity required for a log to be processed.
	MinLogLevel LogLevel
	// SystemPrefixes are strings prefixed to logs to identify the subsystem (e.g., "Auth", "DB").
	SystemPrefixes []string
}

// ---------------------------------------------------------------------------
// Configuration Functions
// ---------------------------------------------------------------------------

/*
EchoConfigurationMaxPrefixLengthSet sets the maximum visual length for prefixes in formatted output.

This function controls the width allocated for system prefixes in log output. Prefixes
longer than the maximum are truncated with "..." appended. The formatting templates
are automatically recalculated after this call.

Use cases:
- Aligning log output across different prefix lengths
- Controlling log line width for readability
- Truncating long system names to fit terminal width

Time complexity: O(1) - simple assignment
Space complexity: O(1) - no allocations

Prerequisites:
- maxPrefixLength should be at least 3 to allow truncation display

Edge cases:
- Prefixes longer than maxPrefixLength are truncated to (maxPrefixLength - 3) + "..."
- If maxPrefixLength <= 3, truncation may result in very short prefixes
*/
func EchoConfigurationMaxPrefixLengthSet(maxPrefixLength uint8) {
	echoMaxPrefixLength = maxPrefixLength
}

/*
EchoConfigurationApplicationPrefixSet sets a global application prefix for all log output.

The application prefix is prepended to all system prefixes in log output, creating
a hierarchical naming scheme (e.g., "MyApp.Auth", "MyApp.DB"). If empty, no prefix
is added. Formatting templates are automatically recalculated.

Use cases:
- Multi-application environments where logs from different apps need identification
- Service identification in microservice architectures
- Log aggregation and filtering by application name

Time complexity: O(1) - simple assignment
Space complexity: O(1) - no allocations

Prerequisites:
- None

Edge cases:
- Empty string disables application prefix
- Application prefix is combined with system prefixes using "." separator
*/
func EchoConfigurationApplicationPrefixSet(applicationPrefix string) {
	echoApplicationPrefix = applicationPrefix
}

/*
EchoSystemRegister registers a configuration for a specific system UUID.

This function associates a system UUID with its logging configuration (log level
and prefixes). If a system is not registered, it falls back to the default
configuration. Registration is thread-safe and can be called at any time.

Use cases:
- Initializing logging for a new module or subsystem
- Per-system log level configuration
- System identification via prefixes

Time complexity: O(1) - map insertion with lock
Space complexity: O(1) - stores pointer to configuration

Prerequisites:
- systemID should be a valid UUID (typically from essence package)
- systemConfig should have valid MinLogLevel and SystemPrefixes

Edge cases:
- Overwrites existing configuration if systemID is already registered
- Thread-safe: uses write lock for concurrent access
- Unregistered systems fall back to default configuration
*/
func EchoSystemRegister(systemID essence.UUID, systemConfig EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &systemConfig
}

/*
EchoSystemRegisterFromString registers a configuration for a system UUID parsed from a string.

This function parses a UUID string and registers the configuration, returning the
parsed UUID for convenience. It is a convenience wrapper around UUID parsing and
EchoSystemRegister.

Use cases:
- Registering systems from configuration files (string-based UUIDs)
- Dynamic system registration from external sources
- Simplified registration when UUID is already in string format

Time complexity: O(1) - UUID parsing and map insertion
Space complexity: O(1) - stores pointer to configuration

Prerequisites:
- systemID must be a valid UUID string format
- systemConfig should have valid MinLogLevel and SystemPrefixes

Edge cases:
- Returns error if systemID is not a valid UUID string
- Overwrites existing configuration if systemID is already registered
- Thread-safe: uses read-write lock for concurrent access
*/
func EchoSystemRegisterFromString(systemID string, systemConfig EchoSystemConfiguration) (essence.UUID, error) {
	idGen, err := essence.UUIDFromString(systemID)
	if err != nil {
		return idGen, err
	}

	foundation.WithRWLock(&loggingMu, func() {
		loggingConfiguration[idGen] = &systemConfig
	})

	return idGen, nil
}

/*
EchoSystemRegisterDefault registers the fallback configuration for unregistered systems.

This function sets the default configuration that is used when a system UUID has
not been explicitly registered via EchoSystemRegister. All unregistered systems
will use this configuration until they are explicitly registered.

Use cases:
- Setting global default log level for all unregistered systems
- Providing fallback behavior for systems that haven't initialized logging
- Global log level control

Time complexity: O(1) - map insertion with lock
Space complexity: O(1) - stores pointer to configuration

Prerequisites:
- config should have valid MinLogLevel and SystemPrefixes

Edge cases:
- Overwrites existing default configuration
- Thread-safe: uses write lock for concurrent access
- Should be called during application initialization
*/
func EchoSystemRegisterDefault(config EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[defaultNamespaceUUID] = &config
}

/*
EchoSystemInternalLogLevelSet sets the log level for Echo library's internal messages.

This function controls the verbosity of Echo's own logging (e.g., configuration
warnings, fallback notifications). The Echo system is automatically registered
during package initialization with INFO level.

Use cases:
- Reducing Echo internal log noise during debugging
- Enabling verbose Echo diagnostics when troubleshooting logging issues
- Controlling library verbosity independently from application logs

Time complexity: O(1) - map lookup and assignment with lock
Space complexity: O(1) - no allocations

Prerequisites:
- Echo system must be registered (happens automatically in init())

Edge cases:
- No-op if Echo system configuration is not found
- Thread-safe: uses write lock for concurrent access
- Defaults to INFO if not explicitly set
*/
func EchoSystemInternalLogLevelSet(logLevel LogLevel) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	if cfg, ok := loggingConfiguration[echoNamespaceUUID]; ok {
		cfg.MinLogLevel = logLevel
	}
}

/*
EchoSystemConfigurationReplace hot-swaps the configuration for a given system ID.

This function replaces the existing configuration for a system with a new one,
allowing runtime reconfiguration without restarting the application. The operation
is atomic and thread-safe.

Use cases:
- Runtime log level changes (e.g., via admin API or configuration reload)
- Dynamic prefix updates
- Hot-reloading logging configuration

Time complexity: O(1) - map assignment with lock
Space complexity: O(1) - stores pointer to new configuration

Prerequisites:
- systemID should be a registered system UUID
- newConfiguration should have valid MinLogLevel and SystemPrefixes

Edge cases:
- Creates new entry if systemID is not registered (same as EchoSystemRegister)
- Thread-safe: uses write lock for concurrent access
- Changes take effect immediately for new log entries
*/
func EchoSystemConfigurationReplace(systemID essence.UUID, newConfiguration EchoSystemConfiguration) {
	loggingMu.Lock()
	defer loggingMu.Unlock()
	loggingConfiguration[systemID] = &newConfiguration
}

/*
EchoInternalPrefixRegister registers a package path prefix to be treated as "internal".

Internal prefixes are used to skip frames during source file/line detection. When
Echo walks the call stack to find the origin of a log, it skips frames from packages
matching registered internal prefixes. This is useful when wrapping Echo in utility
libraries to ensure source detection points to user code, not wrapper code.

Use cases:
- Wrapping Echo in utility libraries (source should point to caller, not wrapper)
- Creating logging abstractions that should be transparent in source detection
- Filtering out framework code from source location

Time complexity: O(1) - slice append operation
Space complexity: O(1) - appends to shared slice

Prerequisites:
- prefix should match the package path prefix (e.g., "mylib/logger")

Edge cases:
- Multiple prefixes can be registered
- Prefix matching uses substring search (strings.Contains)
- "echo" is registered by default
*/
func EchoInternalPrefixRegister(prefix string) {
	internalPrefixes = append(internalPrefixes, prefix)
}

/*
EchoSystemLogLevelEnabled checks if a log level is enabled for a given system ID.

This function provides a fast way to check if logging at a specific level will be processed
for a system, allowing callers to avoid expensive string formatting operations when logging
is disabled.

Use cases:
- Avoiding expensive fmt.Sprintf() calls when logging is disabled
- Conditional logic based on log level availability
- Performance optimization in hot paths

Time complexity: O(1) - map lookup and simple comparison
Space complexity: O(1) - no allocations

Prerequisites:
- systemID should be a registered system UUID (falls back to default if not found)

Edge cases:
- Returns false if systemID is not registered and no default config exists
- Returns true if logLevel >= MinLogLevel for the system
- Thread-safe (uses read lock)
*/
//go:inline
func EchoSystemLogLevelEnabled(systemID essence.UUID, logLevel LogLevel) bool {
	loggingMu.RLock()
	config, ok := loggingConfiguration[systemID]
	loggingMu.RUnlock()

	if !ok {
		loggingMu.RLock()
		defaultConfig, defaultOk := loggingConfiguration[defaultNamespaceUUID]
		loggingMu.RUnlock()

		if !defaultOk {
			return false
		}
		return echoCanLog(defaultConfig, logLevel)
	}

	return echoCanLog(config, logLevel)
}

// ---------------------------------------------------------------------------
// Hooks & Processors
// ---------------------------------------------------------------------------

/*
EchoLog represents a single log entry with all metadata, structure, and context.

EchoLog contains all information about a log entry, including level, message, timing,
system identification, structured fields, and source location. Expensive operations
(source file detection, field merging) are performed lazily via helper functions
to optimize performance when information is not needed.

Use cases:
- Log hooks that need full log information
- Custom output formatters
- Log processors that modify log entries

Performance:
- Source file/line detection is lazy (only when EchoLogGetSource is called)
- Field merging is lazy (only when EchoLogGetMergedFields is called)
- Hooks receive a copy of the log (safe for concurrent modification)
*/
type EchoLog struct {
	Level      LogLevel
	Message    string
	ForceShow  bool
	CanShow    bool
	Id         essence.UUID
	Time       time.Time
	Prefixes   []string
	Fields     map[string]interface{} // Structured key-value pairs (lazily populated)
	SourceFile string                 // File path where the log originated (lazily populated)
	SourceLine int                    // Line number where the log originated (lazily populated)

	// Raw data for lazy evaluation
	RawFields  map[string]interface{} // Original fields before merging
	RawCtx     context.Context        // Original context for field extraction
	CallerSkip int                    // Number of frames to skip for source capture
}

/*
LogHook is a function type that outputs a log entry to a destination.

LogHook functions are called synchronously for each log entry that passes the
minimum log level filter. Hooks receive a copy of the EchoLog, allowing safe
concurrent access. Common implementations include console output, file writing,
and remote logging services.

Use cases:
- Console output formatting
- File logging with rotation
- Remote logging (Splunk, Elasticsearch, etc.)
- Database logging
- Custom output destinations

Performance:
- Hooks are called synchronously in registration order
- Panic recovery is handled automatically
- Lazy evaluation helpers should be used for expensive operations
*/
type LogHook func(log EchoLog)

// ---------------------------------------------------------------------------
// Lazy Evaluation Helpers
// ---------------------------------------------------------------------------

/*
EchoLogGetSource lazily captures the source file and line number for a log entry.

This function performs the expensive runtime.Callers operation on-demand, caching
the result in the log entry. Subsequent calls return the cached value.

Use cases:
- Hooks that need source location information for debugging
- File outputters that want full log details
- Conditional source capture based on log level

Time complexity: O(n) where n is call stack depth - only on first call, O(1) for cached calls
Space complexity: O(1) - allocates fixed-size array for callers

Prerequisites:
- log.CallerSkip must be set correctly by the orchestrator

Edge cases:
- Returns "unknown", 0 if call stack cannot be determined
- Caches result in log.SourceFile and log.SourceLine
- Safe for concurrent use: hooks receive their own copy of the log (passed by value)
*/
func EchoLogGetSource(log *EchoLog) (string, int) {
	if log.SourceFile != "" || log.SourceLine != 0 {
		return log.SourceFile, log.SourceLine
	}

	file, line := captureSourceWithSkip(log.CallerSkip)
	log.SourceFile = file
	log.SourceLine = line
	return file, line
}

/*
EchoLogGetMergedFields lazily merges raw fields and context extractors for a log entry.

This function performs the expensive field merging and context extraction on-demand,
caching the result in the log entry. Subsequent calls return the cached value.

Use cases:
- Hooks that need structured field data
- Outputters that format fields for display
- Conditional field processing based on log level

Time complexity: O(n+m) where n is field count, m is context extractor count - only on first call, O(1) for cached calls
Space complexity: O(n) - allocates map for merged fields

Prerequisites:
- log.RawFields and log.RawCtx must be set by the orchestrator

Edge cases:
- Returns empty map if no fields and no context
- Caches result in log.Fields
- Safe for concurrent use: hooks receive their own copy of the log (passed by value)
*/
func EchoLogGetMergedFields(log *EchoLog) map[string]interface{} {
	if log.Fields != nil {
		return log.Fields
	}

	fields := mergeFieldsAndContext(log.RawFields, log.RawCtx)
	log.Fields = fields
	return fields
}

/*
LogProcessor is a middleware function that modifies a log entry before it is sent to hooks.

LogProcessor functions run before hooks and can modify the EchoLog in-place. They
receive a pointer to the log, allowing field modification, masking, or injection.
Processors run in registration order and are called for all logs that pass the
minimum log level filter.

Use cases:
- Masking PII (personally identifiable information)
- Injecting global environment variables
- Adding correlation IDs
- Sanitizing sensitive data
- Enriching logs with additional context

Performance:
- Processors run synchronously before hooks
- Modifications affect all subsequent hooks
- Should be fast to avoid blocking log output
*/
type LogProcessor func(log *EchoLog)

/*
ContextExtractor is a function that extracts structured data from a context.Context.

ContextExtractor functions are called when a log entry includes a context (via
the fluent API's Ctx method). They extract key-value pairs from the context
and merge them into the log's structured fields. Multiple extractors can be
registered and all will be called.

Use cases:
- Extracting trace IDs from OpenTelemetry context
- Extracting request IDs from HTTP context
- Extracting user IDs from authentication context
- Extracting correlation IDs from request context

Performance:
- Extractors are called during field merging (lazy evaluation)
- Should be fast to avoid blocking log output
- Results are cached in the log entry
*/
type ContextExtractor func(ctx context.Context) map[string]interface{}

var (
	logHooks          = make([]LogHook, 0)
	logProcessors     = make([]LogProcessor, 0)
	contextExtractors = make([]ContextExtractor, 0)
	hookMu            sync.RWMutex
)

/*
EchoLogHookRegister registers a new output destination for log entries.

This function adds a LogHook to the list of hooks that receive log entries.
Hooks are executed synchronously in registration order. Each hook receives
a copy of the log entry, allowing safe concurrent access.

Use cases:
- Registering console outputter
- Registering file outputter
- Registering remote logging services
- Multiple output destinations (console + file + remote)

Time complexity: O(1) - slice append with lock
Space complexity: O(1) - appends function to slice

Prerequisites:
- outputter should be a valid LogHook function

Edge cases:
- Hooks are called in registration order
- Panic recovery is handled automatically for each hook
- Thread-safe: uses write lock for concurrent access
*/
func EchoLogHookRegister(outputter LogHook) {
	hookMu.Lock()
	defer hookMu.Unlock()
	logHooks = append(logHooks, outputter)
}

/*
EchoLogProcessorRegister registers a middleware processor for log entries.

This function adds a LogProcessor to the list of processors that modify log
entries before they are sent to hooks. Processors run in registration order
and can modify the log in-place.

Use cases:
- Registering PII masking processors
- Registering field injection processors
- Registering sanitization processors
- Multiple processors for different concerns

Time complexity: O(1) - slice append with lock
Space complexity: O(1) - appends function to slice

Prerequisites:
- processor should be a valid LogProcessor function

Edge cases:
- Processors run in registration order before hooks
- Modifications affect all subsequent hooks
- Thread-safe: uses write lock for concurrent access
*/
func EchoLogProcessorRegister(processor LogProcessor) {
	hookMu.Lock()
	defer hookMu.Unlock()
	logProcessors = append(logProcessors, processor)
}

/*
EchoContextExtractorRegister registers a function that extracts data from context.Context.

This function adds a ContextExtractor to the list of extractors that pull structured
data from context when a log entry includes a context. Extractors are called during
field merging and their results are cached in the log entry.

Use cases:
- Registering trace ID extractors
- Registering request ID extractors
- Registering user ID extractors
- Multiple extractors for different context types

Time complexity: O(1) - slice append with lock
Space complexity: O(1) - appends function to slice

Prerequisites:
- extractor should be a valid ContextExtractor function

Edge cases:
- Extractors are called during field merging (lazy evaluation)
- All registered extractors are called for each context
- Thread-safe: uses write lock for concurrent access
*/
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

/*
On initiates a fluent logging chain for the given system ID.

This function starts the builder pattern for creating log entries. It returns
an EchoEvent that can be chained with context, fields, and log level methods
to build rich, structured log entries.

Use cases:
- Fluent API for structured logging
- Context-aware logging
- Field-based structured data
- Recommended logging pattern (preferred over legacy functions)

Time complexity: O(1) - struct allocation
Space complexity: O(1) - allocates EchoEvent struct

Prerequisites:
- systemID should be a registered system UUID (falls back to default if not found)

Edge cases:
- Returns empty EchoEvent if systemID is not registered (uses default config)
- Fields map is initialized empty (can be populated via Field/Fields methods)
- Thread-safe: configuration lookup uses read lock
*/
func On(systemID essence.UUID) *EchoEvent {
	return &EchoEvent{
		id:     systemID,
		fields: make(map[string]interface{}),
	}
}

/*
Ctx attaches a context to the log entry.

This method attaches a context.Context to the log entry. When the log is
finalized, all registered ContextExtractor functions will be called to extract
structured data from the context and merge it into the log's fields.

Use cases:
- Passing request context for trace ID extraction
- Passing authentication context for user ID extraction
- Passing correlation context for request tracking
- Context-aware logging in HTTP handlers

Time complexity: O(1) - simple assignment
Space complexity: O(1) - stores context reference

Prerequisites:
- ctx should be a valid context.Context (can be nil)

Edge cases:
- Nil context is allowed (no extraction performed)
- Context extraction happens during field merging (lazy evaluation)
- Multiple extractors can extract different fields from the same context
*/
func (e *EchoEvent) Ctx(ctx context.Context) *EchoEvent {
	e.ctx = ctx
	return e
}

/*
Field adds a single key-value pair to the structured log data.

This method adds a key-value pair to the log entry's structured fields. Fields
are merged with context-extracted fields and included in the final log output.
Multiple fields can be added by chaining Field calls.

Use cases:
- Adding user IDs, request IDs, or correlation IDs
- Adding performance metrics (duration, size, count)
- Adding business context (order ID, transaction ID)
- Adding diagnostic information

Time complexity: O(1) - map insertion
Space complexity: O(1) - stores key-value in map

Prerequisites:
- key should be a non-empty string
- value can be any type (will be formatted by outputters)

Edge cases:
- Overwrites existing field if key already exists
- Fields are merged with context-extracted fields during output
- Thread-safe: each EchoEvent is independent
*/
func (e *EchoEvent) Field(key string, value interface{}) *EchoEvent {
	e.fields[key] = value
	return e
}

/*
Fields adds a map of key-value pairs to the structured log data.

This method adds multiple key-value pairs to the log entry's structured fields
in a single call. It is more efficient than chaining multiple Field calls when
adding many fields at once.

Use cases:
- Adding multiple fields from a single source (e.g., request metadata)
- Bulk field addition from maps or structs
- Copying fields from another log entry
- Efficient field population when many fields are known upfront

Time complexity: O(n) where n is the number of fields in the map
Space complexity: O(n) - stores all key-value pairs in map

Prerequisites:
- fields should be a valid map (can be nil or empty)

Edge cases:
- Overwrites existing fields if keys already exist
- Nil or empty map is allowed (no-op)
- Fields are merged with context-extracted fields during output
*/
func (e *EchoEvent) Fields(fields map[string]interface{}) *EchoEvent {
	for k, v := range fields {
		e.fields[k] = v
	}
	return e
}

/*
Force flags the log to be displayed regardless of the current minimum log level configuration.

This method forces a log entry to be displayed even if it is below the configured
minimum log level for the system. This is useful for critical debugging information
that must be visible without changing global log level settings.

Use cases:
- Critical debugging in production without enabling global debug
- Important diagnostic information that must always be visible
- Temporary logging for specific code paths
- Emergency visibility for troubleshooting

Time complexity: O(1) - simple assignment
Space complexity: O(1) - sets boolean flag

Prerequisites:
- None

Edge cases:
- Bypasses log level filtering for this specific entry
- Should be used sparingly to avoid log noise
- Thread-safe: each EchoEvent is independent
*/
func (e *EchoEvent) Force() *EchoEvent {
	e.forceShow = true
	return e
}

// --- Terminators (Output) ---

/*
Trace finalizes the event and logs it at TRACE level.

TRACE is the most verbose log level, intended for extremely fine-grained
information about program execution. This includes loop iterations, full
payload dumps, detailed step-by-step execution flows, and function entry/exit.

Use cases:
- Tracing execution paths through complex algorithms
- Full payload logging for debugging
- Step-by-step execution flow documentation
- Detailed state dumps

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Typically disabled in production due to high volume
- Use Force() method to bypass log level filtering if needed
- High performance impact if enabled in hot paths
*/
func (e *EchoEvent) Trace(content string) {
	echoLogInternal(TRACE, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Debug finalizes the event and logs it at DEBUG level.

DEBUG is used for diagnostic information useful to developers during development
and debugging. This includes internal state snapshots, variable values, boolean
logic results, and intermediate computation results.

Use cases:
- Development and debugging
- Internal state inspection
- Variable value logging
- Conditional branch tracking

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Usually disabled in production to reduce noise
- Safe to enable temporarily for troubleshooting
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Debug(content string) {
	echoLogInternal(DEBUG, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Info finalizes the event and logs it at INFO level.

INFO is used for general operational milestones that confirm the system is
working as expected. This includes service startup, job completion, connection
establishment, and other normal operational events.

Use cases:
- Service lifecycle events (start, stop, reload)
- Successful operation confirmations
- Normal operational milestones
- System health indicators

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Standard log level for production environments
- Messages should be meaningful to operators
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Info(content string) {
	echoLogInternal(INFO, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Notice finalizes the event and logs it at NOTICE level.

NOTICE is used for events that are unusual or significant but strictly non-error
conditions. These are events that are technically "normal" but rare enough that
a human administrator might want to be aware of them.

Use cases:
- Configuration reloads
- Auto-healing triggers
- Failover events
- Significant state changes

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Indicates noteworthy but expected events
- Should be used for events requiring operator awareness
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Notice(content string) {
	echoLogInternal(NOTICE, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Warning finalizes the event and logs it at WARNING level.

WARNING is used for unexpected situations that do not halt execution or fail a
request, but may indicate a potential problem. These are conditions that should
be monitored to prevent future errors, but do not require immediate intervention.

Use cases:
- Performance degradation (slow queries, high latency)
- Resource warnings (disk space, memory usage)
- Deprecated API usage
- Retryable errors

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Indicates potential problems that don't stop execution
- Should be monitored to prevent future errors
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Warning(content string) {
	echoLogInternal(WARNING, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Error finalizes the event and logs it at ERROR level.

ERROR is used when a specific operation or request has failed and cannot be
completed. These indicate bugs or infrastructure issues that likely affect
the user experience and require developer attention.

Use cases:
- Operation failures (database commit failed, file not found)
- HTTP error responses (500, 502, 503)
- Validation failures
- Business logic errors

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Indicates failures that affect user experience
- Requires developer attention
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Error(content string) {
	echoLogInternal(ERROR, e.id, content, e.fields, e.ctx, e.forceShow)
}

/*
Critical finalizes the event and logs it at CRITICAL level.

CRITICAL is used for severe failures that compromise the stability or integrity
of the entire system. These events imply the application is unusable or dangerous
and should trigger immediate alerts to on-call staff.

Use cases:
- Data corruption detection
- Security breaches
- System-wide failures
- Unrecoverable errors

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- EchoEvent should be properly initialized via On()

Edge cases:
- Indicates system-wide failures requiring immediate attention
- Should trigger alerting/paging systems
- Use Force() method to bypass log level filtering if needed
*/
func (e *EchoEvent) Critical(content string) {
	echoLogInternal(CRITICAL, e.id, content, e.fields, e.ctx, e.forceShow)
}

// ---------------------------------------------------------------------------
// Public Log Functions (Legacy / Simple Wrappers)
// ---------------------------------------------------------------------------

/*
EchoLogTrace logs a message at TRACE level using the legacy API.

This function is a convenience wrapper for echo.On(id).Trace(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Trace())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick debugging statements

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Trace()) for new code
*/
func EchoLogTrace(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(TRACE, systemID, content, nil, nil, forceShow)
}

/*
EchoLogDebug logs a message at DEBUG level using the legacy API.

This function is a convenience wrapper for echo.On(id).Debug(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Debug())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick debugging statements

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Debug()) for new code
*/
func EchoLogDebug(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(DEBUG, systemID, content, nil, nil, forceShow)
}

/*
EchoLogInfo logs a message at INFO level using the legacy API.

This function is a convenience wrapper for echo.On(id).Info(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Info())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick operational logging

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Info()) for new code
*/
func EchoLogInfo(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(INFO, systemID, content, nil, nil, forceShow)
}

/*
EchoLogNotice logs a message at NOTICE level using the legacy API.

This function is a convenience wrapper for echo.On(id).Notice(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Notice())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick notice logging

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Notice()) for new code
*/
func EchoLogNotice(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(NOTICE, systemID, content, nil, nil, forceShow)
}

/*
EchoLogWarning logs a message at WARNING level using the legacy API.

This function is a convenience wrapper for echo.On(id).Warning(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Warning())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick warning logging

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Warning()) for new code
*/
func EchoLogWarning(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(WARNING, systemID, content, nil, nil, forceShow)
}

/*
EchoLogError logs a message at ERROR level using the legacy API.

This function is a convenience wrapper for echo.On(id).Error(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Error())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick error logging

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Error()) for new code
*/
func EchoLogError(systemID essence.UUID, content string, forceShow bool) {
	echoLogInternal(ERROR, systemID, content, nil, nil, forceShow)
}

/*
EchoLogCritical logs a message at CRITICAL level using the legacy API.

This function is a convenience wrapper for echo.On(id).Critical(content). It
provides a simple, non-fluent API for logging. The fluent API (On().Critical())
is recommended for new code as it supports context and structured fields.

Use cases:
- Simple logging without context or fields
- Legacy code compatibility
- Quick critical logging

Time complexity: O(1) - delegates to echoLogInternal
Space complexity: O(1) - creates log entry struct

Prerequisites:
- systemID should be a registered system UUID

Edge cases:
- No support for context or structured fields
- forceShow bypasses log level filtering
- Prefer fluent API (On().Critical()) for new code
*/
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
		return
	}

	// Log a debug message about the fallback, then proceed
	EchoLogDebug(echoNamespaceUUID, fmt.Sprintf("Config missing for '%s', using default", systemID.String()), false)
	processLog(defaultConfig, systemID, logLevel, content, nil, nil, forceShow)
}

//go:inline
func processLog(config *EchoSystemConfiguration, systemID essence.UUID, logLevel LogLevel, content string, fields map[string]interface{}, ctx context.Context, forceShow bool) {
	canLog := echoCanLog(config, logLevel)

	// Calculate caller skip depth for lazy source capture
	// Call stack from EchoLogGetSource: runtime.Callers -> EchoLogGetSource -> hook -> safeExecuteHook -> dispatchToHooks -> processLog -> echoLogInternal -> user code
	// We need to skip: runtime.Callers(1) + EchoLogGetSource(1) + hook(1) + safeExecuteHook(1) + dispatchToHooks(1) + processLog(1) + echoLogInternal(1) = 7
	const callerSkipDepth = 7

	logEntry := EchoLog{
		Level:      logLevel,
		Message:    content,
		ForceShow:  forceShow,
		CanShow:    canLog,
		Id:         systemID,
		Time:       time.Now().Local(),
		Prefixes:   config.SystemPrefixes,
		Fields:     nil, // Lazily populated via EchoLogGetMergedFields()
		SourceFile: "",  // Lazily populated via EchoLogGetSource()
		SourceLine: 0,   // Lazily populated via EchoLogGetSource()

		// Raw data for lazy evaluation
		RawFields:  fields,
		RawCtx:     ctx,
		CallerSkip: callerSkipDepth,
	}

	applyProcessors(&logEntry)
	dispatchToHooks(logEntry)
}

//go:inline
func captureSourceWithSkip(skip int) (string, int) {
	const maxDepth = 32
	var pcs [maxDepth]uintptr

	// skip accounts for: runtime.Callers (1) + captureSourceWithSkip (1) + additional skip
	n := runtime.Callers(skip, pcs[:])
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
