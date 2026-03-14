# echo

Lightweight, flexible, and structured logging library with context-aware logging, structured fields, middleware processors, and multiple output hooks.

## Overview

Echo provides a comprehensive logging solution for Go applications with support for structured logging, context-aware field extraction, per-system log level configuration, and extensible output destinations. The library emphasizes performance through lazy evaluation of expensive operations (source location detection, field merging) and provides both a fluent builder API and legacy convenience functions.

Key features:
- **Structured Logging**: Key-value field support with automatic context extraction
- **Per-System Configuration**: Independent log levels and prefixes for different subsystems
- **Lazy Evaluation**: Expensive operations (source detection, field merging) only when needed
- **Extensible Output**: Hook-based architecture for console, file, and custom destinations
- **Middleware Support**: Processors for log modification (PII masking, field injection)
- **Context Integration**: Automatic extraction of trace IDs, request IDs, and other context data

## Design Philosophy

- **Explicit Context**: All logging operations require explicit system identification via UUID, promoting clear ownership and configuration
- **Lazy Evaluation**: Expensive operations (runtime.Callers, field merging) are performed only when hooks actually need the information
- **Zero-Cost Abstractions**: Log level checks happen early, avoiding unnecessary work when logging is disabled
- **Separation of Concerns**: Output formatting, source detection, and field extraction are independent, composable components
- **Thread Safety**: All configuration and hook registration operations are thread-safe using read-write locks
- **Extensibility**: Hook-based architecture allows custom output destinations without modifying core library code

## Performance Characteristics

- **Lazy Source Detection**: Source file/line detection uses `runtime.Callers` only when hooks request it via `EchoLogGetSource`
- **Lazy Field Merging**: Context extraction and field merging occur only when hooks request it via `EchoLogGetMergedFields`
- **Early Filtering**: Log level checks happen before any expensive operations, allowing zero-cost disabled logging
- **Minimal Allocations**: Core logging path allocates only the log entry struct; fields map is reused when possible
- **Lock-Free Reads**: Configuration lookups use read locks, allowing concurrent reads from multiple goroutines
- **Inline Functions**: Critical path functions are marked with `//go:inline` for compiler optimization

## Integration

Echo is a standalone logging library with minimal dependencies:

```
echo
├── essence (UUID generation and management)
├── foundation (utility functions)
└── github.com/mattn/go-isatty (terminal detection)
```

Echo is designed to be integrated into any Go application requiring structured logging. It does not depend on other application-specific libraries and can be used as a drop-in replacement for standard library logging.

## Packages

### `echo`

The main package provides the core logging functionality, configuration management, and fluent API.

**Key Types:**
- `LogLevel` - Log severity levels (TRACE, DEBUG, INFO, NOTICE, WARNING, ERROR, CRITICAL)
- `EchoSystemConfiguration` - Per-system log level and prefix configuration
- `EchoLog` - Complete log entry with lazy-evaluated fields
- `EchoEvent` - Fluent API builder for constructing log entries
- `LogHook` - Output destination function type
- `LogProcessor` - Middleware function type for log modification
- `ContextExtractor` - Function type for extracting data from context.Context

**Configuration Functions:**
- `EchoSystemRegister` - Register logging configuration for a system UUID
- `EchoSystemRegisterDefault` - Set fallback configuration for unregistered systems
- `EchoSystemLogLevelEnabled` - Fast check if a log level is enabled (for conditional logging)
- `EchoConfigurationMaxPrefixLengthSet` - Set maximum prefix width in output
- `EchoConfigurationApplicationPrefixSet` - Set global application prefix

**Fluent API (Recommended):**
- `On` - Start fluent logging chain for a system UUID
- `Ctx` - Attach context for automatic field extraction
- `Field` / `Fields` - Add structured key-value pairs
- `Force` - Bypass log level filtering for critical messages
- `Trace`, `Debug`, `Info`, `Notice`, `Warning`, `Error`, `Critical` - Finalize and log at specific level

**Legacy API:**
- `EchoLogTrace`, `EchoLogDebug`, `EchoLogInfo`, `EchoLogNotice`, `EchoLogWarning`, `EchoLogError`, `EchoLogCritical` - Simple logging functions

**Hook and Processor Registration:**
- `EchoLogHookRegister` - Register output destination (console, file, remote)
- `EchoLogProcessorRegister` - Register middleware for log modification
- `EchoContextExtractorRegister` - Register context data extractor

**Lazy Evaluation Helpers:**
- `EchoLogGetSource` - Get source file and line (lazy evaluation)
- `EchoLogGetMergedFields` - Get merged fields with context extraction (lazy evaluation)

**Example - Basic Setup and Fluent API:**

```go
package main

import (
    "context"
    "echo"
    "essence"
    "os"
)

var mySystemUUID essence.UUID

func init() {
    // Generate or load your system UUID
    mySystemUUID, _ = essence.UUIDFromString("your-system-uuid-here")
    
    // Register system configuration
    echo.EchoSystemRegister(mySystemUUID, echo.EchoSystemConfiguration{
        MinLogLevel:    echo.INFO,
        SystemPrefixes: []string{"MyApp", "Auth"},
    })
    
    // Register console outputter
    config := echo.DefaultConsoleConfigCreate()
    hook := echo.EchoConsoleOutputterCreate(os.Stdout, config)
    echo.EchoLogHookRegister(hook)
}

func main() {
    ctx := context.Background()
    
    // Fluent API with context and fields
    echo.On(mySystemUUID).
        Ctx(ctx).
        Field("user_id", 123).
        Field("action", "login").
        Info("User logged in successfully")
    
    // Simple logging
    echo.On(mySystemUUID).Debug("Debug information")
    
    // Legacy API
    echo.EchoLogInfo(mySystemUUID, "System started", false)
}
```

**Example - Context Extraction:**

```go
package main

import (
    "context"
    "echo"
    "essence"
)

var mySystemUUID essence.UUID

func init() {
    mySystemUUID, _ = essence.UUIDFromString("your-system-uuid-here")
    
    // Register context extractor for trace IDs
    echo.EchoContextExtractorRegister(func(ctx context.Context) map[string]interface{} {
        if traceID := ctx.Value("trace_id"); traceID != nil {
            return map[string]interface{}{
                "trace_id": traceID,
            }
        }
        return nil
    })
}

func handleRequest(ctx context.Context) {
    // Context is automatically extracted and added to log fields
    echo.On(mySystemUUID).
        Ctx(ctx).
        Info("Request processed")
    // Log will include trace_id field if present in context
}
```

**Example - Log Processor (PII Masking):**

```go
package main

import (
    "echo"
    "strings"
)

func init() {
    // Register processor to mask sensitive fields
    echo.EchoLogProcessorRegister(func(log *echo.EchoLog) {
        fields := echo.EchoLogGetMergedFields(log)
        for key, value := range fields {
            if strings.Contains(strings.ToLower(key), "password") ||
               strings.Contains(strings.ToLower(key), "token") {
                fields[key] = "***MASKED***"
            }
        }
    })
}
```

**Example - File Outputter:**

```go
package main

import (
    "echo"
    "os"
)

func init() {
    // Create file outputter with rotation
    fileConfig := echo.FileConfig{
        LogDirectory:   "./logs",
        Filename:       "app",
        MaxFiles:       10, // Keep 10 old log files
        EmbeddedConfig: echo.DefaultConsoleConfigCreate(),
    }
    
    hook, err := echo.EchoFileOutputterCreate(fileConfig)
    if err != nil {
        panic(err)
    }
    
    echo.EchoLogHookRegister(hook)
    
    // Also register console outputter
    consoleHook := echo.EchoConsoleOutputterCreate(os.Stdout, echo.DefaultConsoleConfigCreate())
    echo.EchoLogHookRegister(consoleHook)
}
```

**Example - Conditional Logging:**

```go
package main

import (
    "echo"
    "essence"
    "fmt"
)

func expensiveOperation(systemID essence.UUID, data []byte) {
    // Fast check before expensive formatting
    if echo.EchoSystemLogLevelEnabled(systemID, echo.DEBUG) {
        // Only format if DEBUG is enabled
        echo.On(systemID).
            Field("data_size", len(data)).
            Field("data_hash", fmt.Sprintf("%x", data[:8])).
            Debug("Processing data")
    }
}
```

## Use Cases

- **Application Logging**: Structured logging for applications with per-module log level control
- **Microservice Logging**: Context-aware logging with trace ID propagation across services
- **Development Debugging**: Verbose logging during development with easy level adjustment
- **Production Monitoring**: Structured logs for log aggregation systems (Splunk, Elasticsearch, etc.)
- **Security Compliance**: PII masking via processors for sensitive data handling
- **Multi-Output Logging**: Simultaneous console and file output with different formatting
- **Performance-Critical Applications**: Zero-cost disabled logging with lazy evaluation of expensive operations

## Safety Guidelines

⚠️ **Important:**

1. **System Registration**: Always register your system UUID before logging. Unregistered systems fall back to default configuration, which may not be appropriate for your use case.

2. **Context Lifetime**: Contexts passed to `Ctx()` should remain valid until the log is processed. Context extraction happens during field merging (lazy evaluation), so the context must be valid at that time.

3. **Hook Panic Recovery**: Hooks are automatically wrapped with panic recovery. If a hook panics, the error is logged to stderr and logging continues with other hooks.

4. **Thread Safety**: Configuration changes (register, replace) use write locks and may block concurrent log operations. Avoid frequent configuration changes in hot paths.

5. **Lazy Evaluation**: Source detection and field merging are expensive operations. Hooks should only call `EchoLogGetSource` and `EchoLogGetMergedFields` when actually needed.

6. **File Outputter**: File outputters keep file handles open for the lifetime of the application. Ensure proper cleanup on application shutdown if needed.

7. **Log Level Filtering**: The `Force()` method bypasses log level filtering. Use sparingly to avoid log noise in production.

8. **Internal Prefixes**: If wrapping Echo in a utility library, register your package prefix via `EchoInternalPrefixRegister` to ensure source detection points to user code, not wrapper code.

## Implementation Notes

- **Source Detection**: Uses `runtime.Callers` with configurable skip depth. Internal frames (matching registered prefixes) are automatically skipped to point to user code.

- **Field Merging**: Fields are merged in order: manual fields (from `Field`/`Fields`) are added first, then context-extracted fields. Context extractors can overwrite manual fields if keys conflict.

- **Log Level Hierarchy**: Log levels are ordered as: TRACE < DEBUG < INFO < NOTICE < WARNING < ERROR < CRITICAL. A log is displayed if its level is >= the system's MinLogLevel.

- **Hook Execution**: Hooks are executed synchronously in registration order. Each hook receives a copy of the log entry, allowing safe concurrent modification.

- **Processor Execution**: Processors run before hooks and receive a pointer to the log entry, allowing in-place modification. Processors run in registration order.

- **Color Detection**: Console outputter automatically detects terminal capabilities via `go-isatty`. Respects `NO_COLOR` and `FORCE_COLOR` environment variables.

- **File Rotation**: File outputter creates timestamped files (e.g., `app-2024-01-01_12-00-00.log`) and automatically cleans up old files in a background goroutine.
