package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents a log level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s", s)
	}
}

// Fields is a map of log fields
type Fields map[string]interface{}

// Logger represents a logger instance
type Logger struct {
	level            Level
	format           string // json or text
	output           io.Writer
	componentLevels  map[string]Level
	sanitizePatterns []*regexp.Regexp
	mu               sync.RWMutex
}

// Entry represents a single log entry
type Entry struct {
	Timestamp     string                 `json:"timestamp"`
	Level         string                 `json:"level"`
	Component     string                 `json:"component,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Message       string                 `json:"message"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

var (
	globalLogger *Logger
	loggerMu     sync.RWMutex
)

// New creates a new logger instance
func New(level Level, format string, output io.Writer) *Logger {
	return &Logger{
		level:           level,
		format:          format,
		output:          output,
		componentLevels: make(map[string]Level),
	}
}

// Init initializes the global logger
func Init(level Level, format string, output io.Writer) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	globalLogger = New(level, format, output)
}

// Get returns the global logger
func Get() *Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return globalLogger
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetComponentLevel sets the log level for a specific component
func (l *Logger) SetComponentLevel(component string, level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.componentLevels[component] = level
}

// SetSanitizePatterns sets the regex patterns for field sanitization
func (l *Logger) SetSanitizePatterns(patterns []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sanitizePatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid sanitize pattern %s: %w", pattern, err)
		}
		l.sanitizePatterns = append(l.sanitizePatterns, re)
	}
	return nil
}

// shouldLog checks if a message should be logged based on level and component
func (l *Logger) shouldLog(level Level, component string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Check component-specific level first
	if componentLevel, ok := l.componentLevels[component]; ok {
		return level >= componentLevel
	}

	// Fall back to global level
	return level >= l.level
}

// sanitizeFields sanitizes sensitive fields
func (l *Logger) sanitizeFields(fields Fields) Fields {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.sanitizePatterns) == 0 {
		return fields
	}

	sanitized := make(Fields)
	for k, v := range fields {
		// Check if key matches any sanitize pattern
		shouldSanitize := false
		for _, pattern := range l.sanitizePatterns {
			if pattern.MatchString(k) {
				shouldSanitize = true
				break
			}
		}

		if shouldSanitize {
			// Redact the value
			if str, ok := v.(string); ok && len(str) > 4 {
				sanitized[k] = "***" + str[len(str)-4:]
			} else {
				sanitized[k] = "***"
			}
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

// log writes a log entry
func (l *Logger) log(level Level, component, correlationID, message string, fields Fields) {
	if !l.shouldLog(level, component) {
		return
	}

	entry := Entry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Level:         level.String(),
		Component:     component,
		CorrelationID: correlationID,
		Message:       message,
		Fields:        l.sanitizeFields(fields),
	}

	var output string
	if l.format == "json" {
		data, err := json.Marshal(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
			return
		}
		output = string(data) + "\n"
	} else {
		// Text format
		output = l.formatText(entry)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.output.Write([]byte(output))
}

// formatText formats a log entry as text
func (l *Logger) formatText(entry Entry) string {
	parts := []string{
		entry.Timestamp,
		entry.Level,
	}

	if entry.Component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.Component))
	}

	if entry.CorrelationID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.CorrelationID))
	}

	parts = append(parts, entry.Message)

	if len(entry.Fields) > 0 {
		fieldsStr := make([]string, 0, len(entry.Fields))
		for k, v := range entry.Fields {
			fieldsStr = append(fieldsStr, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, strings.Join(fieldsStr, " "))
	}

	return strings.Join(parts, " ") + "\n"
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...Fields) {
	f := mergeFields(fields...)
	l.log(DebugLevel, "", "", message, f)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...Fields) {
	f := mergeFields(fields...)
	l.log(InfoLevel, "", "", message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...Fields) {
	f := mergeFields(fields...)
	l.log(WarnLevel, "", "", message, f)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...Fields) {
	f := mergeFields(fields...)
	l.log(ErrorLevel, "", "", message, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields ...Fields) {
	f := mergeFields(fields...)
	l.log(FatalLevel, "", "", message, f)
	os.Exit(1)
}

// WithComponent creates a component logger
func (l *Logger) WithComponent(component string) *ComponentLogger {
	return &ComponentLogger{
		logger:    l,
		component: component,
	}
}

// ComponentLogger is a logger for a specific component
type ComponentLogger struct {
	logger    *Logger
	component string
}

// Debug logs a debug message for the component
func (cl *ComponentLogger) Debug(message string, fields ...Fields) {
	f := mergeFields(fields...)
	cl.logger.log(DebugLevel, cl.component, "", message, f)
}

// Info logs an info message for the component
func (cl *ComponentLogger) Info(message string, fields ...Fields) {
	f := mergeFields(fields...)
	cl.logger.log(InfoLevel, cl.component, "", message, f)
}

// Warn logs a warning message for the component
func (cl *ComponentLogger) Warn(message string, fields ...Fields) {
	f := mergeFields(fields...)
	cl.logger.log(WarnLevel, cl.component, "", message, f)
}

// Error logs an error message for the component
func (cl *ComponentLogger) Error(message string, fields ...Fields) {
	f := mergeFields(fields...)
	cl.logger.log(ErrorLevel, cl.component, "", message, f)
}

// Fatal logs a fatal message for the component and exits
func (cl *ComponentLogger) Fatal(message string, fields ...Fields) {
	f := mergeFields(fields...)
	cl.logger.log(FatalLevel, cl.component, "", message, f)
	os.Exit(1)
}

// WithCorrelationID creates a logger with correlation ID
func (cl *ComponentLogger) WithCorrelationID(correlationID string) *ContextLogger {
	return &ContextLogger{
		logger:        cl.logger,
		component:     cl.component,
		correlationID: correlationID,
	}
}

// ContextLogger is a logger with context (correlation ID)
type ContextLogger struct {
	logger        *Logger
	component     string
	correlationID string
}

// Debug logs a debug message with context
func (ctx *ContextLogger) Debug(message string, fields ...Fields) {
	f := mergeFields(fields...)
	ctx.logger.log(DebugLevel, ctx.component, ctx.correlationID, message, f)
}

// Info logs an info message with context
func (ctx *ContextLogger) Info(message string, fields ...Fields) {
	f := mergeFields(fields...)
	ctx.logger.log(InfoLevel, ctx.component, ctx.correlationID, message, f)
}

// Warn logs a warning message with context
func (ctx *ContextLogger) Warn(message string, fields ...Fields) {
	f := mergeFields(fields...)
	ctx.logger.log(WarnLevel, ctx.component, ctx.correlationID, message, f)
}

// Error logs an error message with context
func (ctx *ContextLogger) Error(message string, fields ...Fields) {
	f := mergeFields(fields...)
	ctx.logger.log(ErrorLevel, ctx.component, ctx.correlationID, message, f)
}

// Fatal logs a fatal message with context and exits
func (ctx *ContextLogger) Fatal(message string, fields ...Fields) {
	f := mergeFields(fields...)
	ctx.logger.log(FatalLevel, ctx.component, ctx.correlationID, message, f)
	os.Exit(1)
}

// mergeFields merges multiple Fields maps
func mergeFields(fields ...Fields) Fields {
	if len(fields) == 0 {
		return Fields{}
	}
	if len(fields) == 1 {
		return fields[0]
	}

	result := make(Fields)
	for _, f := range fields {
		for k, v := range f {
			result[k] = v
		}
	}
	return result
}

// Correlation ID context key
type contextKey string

const correlationIDKey contextKey = "correlation_id"

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if correlationID, ok := ctx.Value(correlationIDKey).(string); ok {
		return correlationID
	}
	return ""
}

// FromContext creates a logger from context with correlation ID
func FromContext(ctx context.Context, component string) *ContextLogger {
	correlationID := GetCorrelationID(ctx)
	return &ContextLogger{
		logger:        Get(),
		component:     component,
		correlationID: correlationID,
	}
}

// Global convenience functions
func Debug(message string, fields ...Fields) {
	if globalLogger != nil {
		globalLogger.Debug(message, fields...)
	}
}

func Info(message string, fields ...Fields) {
	if globalLogger != nil {
		globalLogger.Info(message, fields...)
	}
}

func Warn(message string, fields ...Fields) {
	if globalLogger != nil {
		globalLogger.Warn(message, fields...)
	}
}

func Error(message string, fields ...Fields) {
	if globalLogger != nil {
		globalLogger.Error(message, fields...)
	}
}

func Fatal(message string, fields ...Fields) {
	if globalLogger != nil {
		globalLogger.Fatal(message, fields...)
	}
}
