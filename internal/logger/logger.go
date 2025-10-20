package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of a LogLevel
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogOutput represents where logs should be written
type LogOutput int

const (
	Console LogOutput = iota
	File
	Both
)

// LoggerConfig holds configuration for the logger
type LoggerConfig struct {
	Level       LogLevel
	Output      LogOutput
	FilePath    string
	IncludeTime bool
	Structured  bool
}

// Logger represents a structured logger with configurable levels and outputs
type Logger struct {
	config      LoggerConfig
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	file        *os.File
}

// NewLogger creates a new Logger instance with the provided configuration
func NewLogger(config LoggerConfig) (*Logger, error) {
	l := &Logger{
		config: config,
	}

	// Set up loggers for different levels
	l.debugLogger = log.New(io.Discard, "", 0)
	l.infoLogger = log.New(io.Discard, "", 0)
	l.warnLogger = log.New(io.Discard, "", 0)
	l.errorLogger = log.New(io.Discard, "", 0)

	// Configure loggers based on level
	switch config.Level {
	case DEBUG:
		l.debugLogger = log.New(os.Stdout, "", 0)
		fallthrough
	case INFO:
		l.infoLogger = log.New(os.Stdout, "", 0)
		fallthrough
	case WARN:
		l.warnLogger = log.New(os.Stdout, "", 0)
		fallthrough
	case ERROR:
		l.errorLogger = log.New(os.Stdout, "", 0)
	}

	// Configure file output if needed
	if config.Output == File || config.Output == Both {
		if config.FilePath == "" {
			config.FilePath = "crawlr.log"
		}

		file, err := os.OpenFile(config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		l.file = file

		// Set up file loggers
		fileDebugLogger := log.New(file, "", 0)
		fileInfoLogger := log.New(file, "", 0)
		fileWarnLogger := log.New(file, "", 0)
		fileErrorLogger := log.New(file, "", 0)

		// Configure file loggers based on level
		switch config.Level {
		case DEBUG:
			l.debugLogger = fileDebugLogger
			fallthrough
		case INFO:
			l.infoLogger = fileInfoLogger
			fallthrough
		case WARN:
			l.warnLogger = fileWarnLogger
			fallthrough
		case ERROR:
			l.errorLogger = fileErrorLogger
		}

		// If output is both, create multiwriters
		if config.Output == Both {
			l.debugLogger = log.New(io.MultiWriter(os.Stdout, fileDebugLogger.Writer()), "", 0)
			l.infoLogger = log.New(io.MultiWriter(os.Stdout, fileInfoLogger.Writer()), "", 0)
			l.warnLogger = log.New(io.MultiWriter(os.Stdout, fileWarnLogger.Writer()), "", 0)
			l.errorLogger = log.New(io.MultiWriter(os.Stdout, fileErrorLogger.Writer()), "", 0)
		}
	}

	return l, nil
}

// Close closes any open resources used by the logger
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// formatMessage formats a log message with optional timestamp and level
func (l *Logger) formatMessage(level LogLevel, message string) string {
	var parts []string

	if l.config.IncludeTime {
		parts = append(parts, time.Now().Format("2006-01-02 15:04:05"))
	}

	parts = append(parts, fmt.Sprintf("[%s]", level.String()))
	parts = append(parts, message)

	return strings.Join(parts, " ")
}

// getCallerInfo returns the file and line number of the caller
func getCallerInfo() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown:0"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	if l.config.Level > DEBUG {
		return
	}

	formatted := l.formatMessage(DEBUG, message)
	if l.config.Structured && len(fields) > 0 {
		formatted = l.formatStructured(DEBUG, message, fields[0])
	}

	l.debugLogger.Output(2, formatted)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.config.Level > DEBUG {
		return
	}

	message := fmt.Sprintf(format, args...)
	formatted := l.formatMessage(DEBUG, message)
	l.debugLogger.Output(2, formatted)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	if l.config.Level > INFO {
		return
	}

	formatted := l.formatMessage(INFO, message)
	if l.config.Structured && len(fields) > 0 {
		formatted = l.formatStructured(INFO, message, fields[0])
	}

	l.infoLogger.Output(2, formatted)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.config.Level > INFO {
		return
	}

	message := fmt.Sprintf(format, args...)
	formatted := l.formatMessage(INFO, message)
	l.infoLogger.Output(2, formatted)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	if l.config.Level > WARN {
		return
	}

	formatted := l.formatMessage(WARN, message)
	if l.config.Structured && len(fields) > 0 {
		formatted = l.formatStructured(WARN, message, fields[0])
	}

	l.warnLogger.Output(2, formatted)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.config.Level > WARN {
		return
	}

	message := fmt.Sprintf(format, args...)
	formatted := l.formatMessage(WARN, message)
	l.warnLogger.Output(2, formatted)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	if l.config.Level > ERROR {
		return
	}

	formatted := l.formatMessage(ERROR, message)
	if l.config.Structured && len(fields) > 0 {
		formatted = l.formatStructured(ERROR, message, fields[0])
	}

	l.errorLogger.Output(2, formatted)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.config.Level > ERROR {
		return
	}

	message := fmt.Sprintf(format, args...)
	formatted := l.formatMessage(ERROR, message)
	l.errorLogger.Output(2, formatted)
}

// ErrorWithStack logs an error message with stack trace
func (l *Logger) ErrorWithStack(err error, message string, fields ...map[string]interface{}) {
	if l.config.Level > ERROR {
		return
	}

	stackTrace := getStackTrace()
	baseMessage := fmt.Sprintf("%s: %v\n%s", message, err, stackTrace)
	formatted := l.formatMessage(ERROR, baseMessage)

	if l.config.Structured {
		mergedFields := map[string]interface{}{
			"error":      err.Error(),
			"stackTrace": stackTrace,
		}
		if len(fields) > 0 {
			for k, v := range fields[0] {
				mergedFields[k] = v
			}
		}
		formatted = l.formatStructured(ERROR, message, mergedFields)
	}

	l.errorLogger.Output(2, formatted)
}

// getStackTrace returns a formatted stack trace
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// formatStructured formats a log message in structured format
func (l *Logger) formatStructured(level LogLevel, message string, fields map[string]interface{}) string {
	baseFields := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level.String(),
		"message":   message,
		"caller":    getCallerInfo(),
	}

	// Merge user fields with base fields
	for k, v := range fields {
		baseFields[k] = v
	}

	// Convert to JSON-like string
	var parts []string
	for k, v := range baseFields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	return strings.Join(parts, " ")
}

// Progress logs progress information for long-running operations
func (l *Logger) Progress(operation string, current, total int, fields ...map[string]interface{}) {
	if l.config.Level > INFO {
		return
	}

	percentage := 0
	if total > 0 {
		percentage = (current * 100) / total
	}

	message := fmt.Sprintf("Progress: %s - %d/%d (%d%%)", operation, current, total, percentage)
	formatted := l.formatMessage(INFO, message)

	if l.config.Structured {
		progressFields := map[string]interface{}{
			"operation":  operation,
			"current":    current,
			"total":      total,
			"percentage": percentage,
		}
		if len(fields) > 0 {
			for k, v := range fields[0] {
				progressFields[k] = v
			}
		}
		formatted = l.formatStructured(INFO, message, progressFields)
	}

	l.infoLogger.Output(2, formatted)
}

// APIRequest logs information about an API request
func (l *Logger) APIRequest(method, url string, headers map[string]string, body interface{}) {
	if l.config.Level > DEBUG {
		return
	}

	message := fmt.Sprintf("API Request: %s %s", method, url)
	formatted := l.formatMessage(DEBUG, message)

	if l.config.Structured {
		requestFields := map[string]interface{}{
			"type":    "api_request",
			"method":  method,
			"url":     url,
			"headers": headers,
			"body":    body,
		}
		formatted = l.formatStructured(DEBUG, message, requestFields)
	}

	l.debugLogger.Output(2, formatted)
}

// APIResponse logs information about an API response
func (l *Logger) APIResponse(method, url string, statusCode int, headers map[string]string, body interface{}) {
	if l.config.Level > DEBUG {
		return
	}

	message := fmt.Sprintf("API Response: %s %s - Status: %d", method, url, statusCode)
	formatted := l.formatMessage(DEBUG, message)

	if l.config.Structured {
		responseFields := map[string]interface{}{
			"type":       "api_response",
			"method":     method,
			"url":        url,
			"statusCode": statusCode,
			"headers":    headers,
			"body":       body,
		}
		formatted = l.formatStructured(DEBUG, message, responseFields)
	}

	l.debugLogger.Output(2, formatted)
}
