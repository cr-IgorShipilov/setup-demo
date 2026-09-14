// Package logging provides a simple file+console logger used by every
// action so that each invocation of the CLI leaves a timestamped record
// on disk, regardless of what is also printed to the screen.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger writes structured lines to a log file while optionally also
// printing them to stdout for interactive use.
type Logger struct {
	file    *os.File
	action  string
	started time.Time
}

// New creates a new log file under logDir named "<action>_<timestamp>.log"
// and returns a Logger bound to it. The caller must call Close when done.
func New(logDir, action string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	ts := time.Now().Format("20060102-150405")
	path := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", action, ts))

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	l := &Logger{file: f, action: action, started: time.Now()}
	l.writeOnly(fmt.Sprintf("=== setup %s started at %s ===", action, l.started.Format(time.RFC3339)))
	return l, nil
}

// Path returns the path of the underlying log file.
func (l *Logger) Path() string {
	if l == nil || l.file == nil {
		return ""
	}
	return l.file.Name()
}

// Infof logs a message to the file only (used for verbose/raw output that
// would clutter the interactive screen, e.g. raw Ansible stdout).
func (l *Logger) Infof(format string, args ...any) {
	l.writeOnly(fmt.Sprintf(format, args...))
}

// Printf logs a message to both the log file and the screen. This is the
// main entry point actions should use for user-facing progress lines.
func (l *Logger) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	l.writeOnly(msg)
}

func (l *Logger) writeOnly(msg string) {
	if l == nil || l.file == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(l.file, "[%s] %s\n", ts, msg)
}

// Close finalizes the log with a summary line and closes the file handle.
func (l *Logger) Close(result string) error {
	if l == nil || l.file == nil {
		return nil
	}
	elapsed := time.Since(l.started).Round(time.Millisecond)
	l.writeOnly(fmt.Sprintf("=== setup %s finished: %s (elapsed %s) ===", l.action, result, elapsed))
	return l.file.Close()
}
