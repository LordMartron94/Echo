package echo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

/*
FileConfig configures the file output behavior and formatting.

FileConfig controls where log files are stored, how they are named, how many
old files to retain, and how entries are formatted. It embeds ConsoleConfig
for formatting control, allowing file and console output to share formatting
logic.

Use cases:
- Persistent log storage
- Log rotation and cleanup
- File-based log archiving
- Consistent formatting between console and file output

Performance:
- Creates new file per application session
- Automatic cleanup of old files in background
- Uses same formatting logic as console outputter
*/
type FileConfig struct {
	LogDirectory   string // Where to store logs
	Filename       string // Base filename (e.g., "app.log" or just "app")
	MaxFiles       int    // How many old log files to keep
	EmbeddedConfig ConsoleConfig
}

/*
EchoFileOutputterCreate creates a LogHook that writes log entries to timestamped files.

This function creates a LogHook that writes log entries to files with timestamped
names (e.g., "app-2024-01-01_12-00-00.log"). A new file is created each time the
application starts. Old files are automatically cleaned up in the background if
the total count exceeds MaxFiles.

Use cases:
- Persistent log storage with automatic rotation
- Per-session log files
- Log archiving and retention
- File-based log analysis

Time complexity: O(1) - file creation and hook registration
Space complexity: O(1) - creates file handle and hook closure

Prerequisites:
- config.LogDirectory must be writable (will be created if missing)
- config.Filename should be a valid filename (without extension)
- config.MaxFiles should be >= 0 (0 disables cleanup)

Edge cases:
- Creates log directory if it doesn't exist
- Returns error if directory creation fails
- Cleanup runs in background goroutine (non-blocking)
- File handle is kept open for the lifetime of the hook
- Always performs full evaluation (source and fields) for file output
- Thread-safe: uses mutex for concurrent writes
*/
func EchoFileOutputterCreate(config FileConfig) (LogHook, error) {
	if err := os.MkdirAll(config.LogDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	// 1. Create the new file for this session
	file, _, err := createSessionFile(config)
	if err != nil {
		return nil, err
	}

	// 2. Run cleanup in background (don't block startup)
	go cleanupOldLogs(config.LogDirectory, config.Filename, config.MaxFiles)

	// 3. Create the formatter (reusing your Console logic, but likely without color)
	// We use a mutex to ensure lines don't interleave if called concurrently.
	var mu sync.Mutex

	return func(log EchoLog) {
		mu.Lock()
		defer mu.Unlock()

		// Lazy evaluation: always get full information for file logging
		// File outputters typically want all logs regardless of CanShow
		EchoLogGetSource(&log)
		EchoLogGetMergedFields(&log)

		// Reuse the formatting logic we created for the console
		// This ensures file and console look identical (except for color)
		msg := formatLogLine(log, config.EmbeddedConfig)
		fmt.Fprint(file, msg)
	}, nil
}

// ---------------------------------------------------------------------------
// Internal Helpers
// ---------------------------------------------------------------------------

// createSessionFile generates a timestamped filename and opens it.
func createSessionFile(config FileConfig) (*os.File, string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	name := fmt.Sprintf("%s-%s.log", config.Filename, timestamp)
	fullPath := filepath.Join(config.LogDirectory, name)

	f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open log file: %w", err)
	}

	return f, fullPath, nil
}

// cleanupOldLogs finds files matching the pattern and deletes the oldest ones.
func cleanupOldLogs(dir, baseName string, maxFiles int) {
	if maxFiles <= 0 {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Echo: failed to read log dir for cleanup: %v\n", err)
		return
	}

	var logFiles []os.FileInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), baseName) && strings.HasSuffix(e.Name(), ".log") {
			info, err := e.Info()
			if err == nil {
				logFiles = append(logFiles, info)
			}
		}
	}

	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime().Before(logFiles[j].ModTime())
	})

	excess := len(logFiles) - maxFiles
	if excess > 0 {
		for i := 0; i < excess; i++ {
			path := filepath.Join(dir, logFiles[i].Name())
			os.Remove(path)
		}
	}
}
