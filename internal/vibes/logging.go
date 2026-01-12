package vibes

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents severity of log entries.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log record.
type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	VibeName  string         `json:"vibe_name"`
	Hook      Hook           `json:"hook,omitempty"`
	Message   string         `json:"message"`
	Duration  *time.Duration `json:"duration_ms,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// Logger handles logging for Vibe execution.
type Logger struct {
	mu       sync.RWMutex
	entries  []LogEntry
	maxSize  int
	writers  []func(LogEntry)
	minLevel LogLevel
	dataDir  string
}

// NewLogger creates a new Vibe logger.
func NewLogger(dataDir string, maxSize int) *Logger {
	return &Logger{
		entries:  make([]LogEntry, 0, maxSize),
		maxSize:  maxSize,
		writers:  make([]func(LogEntry), 0),
		minLevel: LogInfo,
		dataDir:  dataDir,
	}
}

// SetMinLevel sets the minimum log level.
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	l.minLevel = level
	l.mu.Unlock()
}

// AddWriter adds a custom log writer.
func (l *Logger) AddWriter(w func(LogEntry)) {
	l.mu.Lock()
	l.writers = append(l.writers, w)
	l.mu.Unlock()
}

// Log records a log entry.
func (l *Logger) Log(level LogLevel, vibeName string, msg string) {
	l.log(LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		VibeName:  vibeName,
		Message:   msg,
	})
}

// LogHook records a hook execution.
func (l *Logger) LogHook(level LogLevel, vibeName string, hook Hook, msg string, duration time.Duration) {
	dur := duration
	l.log(LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		VibeName:  vibeName,
		Hook:      hook,
		Message:   msg,
		Duration:  &dur,
	})
}

// LogError records an error.
func (l *Logger) LogError(vibeName string, hook Hook, err error) {
	l.log(LogEntry{
		Timestamp: time.Now(),
		Level:     LogError,
		VibeName:  vibeName,
		Hook:      hook,
		Error:     err.Error(),
	})
}

func (l *Logger) log(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Level < l.minLevel {
		return
	}

	// Append entry
	l.entries = append(l.entries, entry)

	// Trim if over capacity
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}

	// Notify writers
	for _, w := range l.writers {
		go w(entry)
	}
}

// Entries returns recent log entries.
func (l *Logger) Entries(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}

	start := len(l.entries) - limit
	result := make([]LogEntry, limit)
	copy(result, l.entries[start:])
	return result
}

// EntriesForVibe returns log entries for a specific Vibe.
func (l *Logger) EntriesForVibe(vibeName string, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []LogEntry
	for i := len(l.entries) - 1; i >= 0 && len(result) < limit; i-- {
		if l.entries[i].VibeName == vibeName {
			result = append(result, l.entries[i])
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// ErrorCount returns the number of errors for a Vibe.
func (l *Logger) ErrorCount(vibeName string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	count := 0
	for _, entry := range l.entries {
		if entry.VibeName == vibeName && entry.Level == LogError {
			count++
		}
	}
	return count
}

// Clear removes all log entries.
func (l *Logger) Clear() {
	l.mu.Lock()
	l.entries = make([]LogEntry, 0, l.maxSize)
	l.mu.Unlock()
}

// Export writes logs to a file.
func (l *Logger) Export(filename string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	path := filepath.Join(l.dataDir, filename)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range l.entries {
		line := fmt.Sprintf("[%s] %s %-20s %s",
			entry.Timestamp.Format(time.RFC3339),
			entry.Level.String(),
			entry.VibeName,
			entry.Message,
		)
		if entry.Error != "" {
			line += fmt.Sprintf(" ERROR: %s", entry.Error)
		}
		if entry.Duration != nil {
			line += fmt.Sprintf(" (%dms)", entry.Duration.Milliseconds())
		}
		fmt.Fprintln(file, line)
	}

	return nil
}

// Telemetry aggregates Vibe execution statistics.
type Telemetry struct {
	mu    sync.RWMutex
	stats map[string]*VibeStats
}

// VibeStats holds statistics for a single Vibe.
type VibeStats struct {
	TotalRuns      int           `json:"total_runs"`
	SuccessfulRuns int           `json:"successful_runs"`
	FailedRuns     int           `json:"failed_runs"`
	TotalDuration  time.Duration `json:"total_duration_ms"`
	AvgDuration    time.Duration `json:"avg_duration_ms"`
	LastRun        *time.Time    `json:"last_run,omitempty"`
	LastError      *string       `json:"last_error,omitempty"`
}

// NewTelemetry creates a new telemetry tracker.
func NewTelemetry() *Telemetry {
	return &Telemetry{
		stats: make(map[string]*VibeStats),
	}
}

// RecordSuccess records a successful Vibe execution.
func (t *Telemetry) RecordSuccess(vibeName string, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := t.getOrCreate(vibeName)
	stats.TotalRuns++
	stats.SuccessfulRuns++
	stats.TotalDuration += duration
	stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalRuns)
	now := time.Now()
	stats.LastRun = &now
}

// RecordFailure records a failed Vibe execution.
func (t *Telemetry) RecordFailure(vibeName string, duration time.Duration, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := t.getOrCreate(vibeName)
	stats.TotalRuns++
	stats.FailedRuns++
	stats.TotalDuration += duration
	stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalRuns)
	now := time.Now()
	stats.LastRun = &now
	errStr := err.Error()
	stats.LastError = &errStr
}

func (t *Telemetry) getOrCreate(vibeName string) *VibeStats {
	if stats, ok := t.stats[vibeName]; ok {
		return stats
	}
	stats := &VibeStats{}
	t.stats[vibeName] = stats
	return stats
}

// GetStats returns statistics for a Vibe.
func (t *Telemetry) GetStats(vibeName string) *VibeStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats[vibeName]
}

// AllStats returns statistics for all Vibes.
func (t *Telemetry) AllStats() map[string]*VibeStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*VibeStats)
	for k, v := range t.stats {
		result[k] = v
	}
	return result
}
