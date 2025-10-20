package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorType represents the type of error
type ErrorType int

const (
	// ConfigurationError represents errors related to configuration
	ConfigurationError ErrorType = iota
	// NetworkError represents errors related to network operations
	NetworkError
	// StorageError represents errors related to file storage
	StorageError
	// APIError represents errors returned by APIs
	APIError
	// ValidationError represents validation errors
	ValidationError
	// CrawlerError represents errors specific to the crawler
	CrawlerError
)

// String returns the string representation of an ErrorType
func (e ErrorType) String() string {
	switch e {
	case ConfigurationError:
		return "ConfigurationError"
	case NetworkError:
		return "NetworkError"
	case StorageError:
		return "StorageError"
	case APIError:
		return "APIError"
	case ValidationError:
		return "ValidationError"
	case CrawlerError:
		return "CrawlerError"
	default:
		return "UnknownError"
	}
}

// CrawlrError represents a custom error with additional context
type CrawlrError struct {
	Type     ErrorType
	Message  string
	Err      error
	Context  map[string]interface{}
	Stack    string
	File     string
	Line     int
	Function string
}

// Error implements the error interface
func (e *CrawlrError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[%s]", e.Type.String()))
	parts = append(parts, e.Message)

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", e.Err))
	}

	if len(e.Context) > 0 {
		var contextParts []string
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("context: %s", strings.Join(contextParts, ", ")))
	}

	return strings.Join(parts, " ")
}

// Unwrap returns the underlying error
func (e *CrawlrError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target error
func (e *CrawlrError) Is(target error) bool {
	if other, ok := target.(*CrawlrError); ok {
		return e.Type == other.Type
	}
	return false
}

// New creates a new CrawlrError with the specified type and message
func New(errorType ErrorType, message string) *CrawlrError {
	err := &CrawlrError{
		Type:    errorType,
		Message: message,
		Context: make(map[string]interface{}),
	}
	err.captureStack()
	return err
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errorType ErrorType, message string) *CrawlrError {
	crawlrErr := &CrawlrError{
		Type:    errorType,
		Message: message,
		Err:     err,
		Context: make(map[string]interface{}),
	}
	crawlrErr.captureStack()
	return crawlrErr
}

// Wrapf wraps an existing error with additional context and formatted message
func Wrapf(err error, errorType ErrorType, format string, args ...interface{}) *CrawlrError {
	message := fmt.Sprintf(format, args...)
	return Wrap(err, errorType, message)
}

// WithContext adds context to an error
func (e *CrawlrError) WithContext(key string, value interface{}) *CrawlrError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithContextMap adds multiple context key-value pairs to an error
func (e *CrawlrError) WithContextMap(context map[string]interface{}) *CrawlrError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	for k, v := range context {
		e.Context[k] = v
	}
	return e
}

// captureStack captures the stack trace for the error
func (e *CrawlrError) captureStack() {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var stackBuilder strings.Builder
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		if e.Function == "" {
			e.Function = frame.Function
		}
		if e.File == "" {
			e.File = frame.File
			e.Line = frame.Line
		}
		stackBuilder.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
	}
	e.Stack = stackBuilder.String()
}

// GetType returns the type of the error
func GetType(err error) ErrorType {
	if crawlrErr, ok := err.(*CrawlrError); ok {
		return crawlrErr.Type
	}
	return -1 // Unknown type
}

// GetContext returns the context of the error
func GetContext(err error) map[string]interface{} {
	if crawlrErr, ok := err.(*CrawlrError); ok {
		return crawlrErr.Context
	}
	return nil
}

// GetStack returns the stack trace of the error
func GetStack(err error) string {
	if crawlrErr, ok := err.(*CrawlrError); ok {
		return crawlrErr.Stack
	}
	return ""
}

// IsType checks if the error is of the specified type
func IsType(err error, errorType ErrorType) bool {
	if crawlrErr, ok := err.(*CrawlrError); ok {
		return crawlrErr.Type == errorType
	}
	return false
}

// IsConfigurationError checks if the error is a configuration error
func IsConfigurationError(err error) bool {
	return IsType(err, ConfigurationError)
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	return IsType(err, NetworkError)
}

// IsStorageError checks if the error is a storage error
func IsStorageError(err error) bool {
	return IsType(err, StorageError)
}

// IsAPIError checks if the error is an API error
func IsAPIError(err error) bool {
	return IsType(err, APIError)
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	return IsType(err, ValidationError)
}

// IsCrawlerError checks if the error is a crawler error
func IsCrawlerError(err error) bool {
	return IsType(err, CrawlerError)
}

// HandleError handles an error based on its type
func HandleError(err error) error {
	if err == nil {
		return nil
	}

	if crawlrErr, ok := err.(*CrawlrError); ok {
		switch crawlrErr.Type {
		case ConfigurationError:
			return HandleConfigurationError(crawlrErr)
		case NetworkError:
			return HandleNetworkError(crawlrErr)
		case StorageError:
			return HandleStorageError(crawlrErr)
		case APIError:
			return HandleAPIError(crawlrErr)
		case ValidationError:
			return HandleValidationError(crawlrErr)
		case CrawlerError:
			return HandleCrawlerError(crawlrErr)
		default:
			return err
		}
	}

	return err
}

// HandleConfigurationError handles configuration errors
func HandleConfigurationError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for configuration errors
	return err.WithContext("recovery", "Check configuration files and environment variables")
}

// HandleNetworkError handles network errors
func HandleNetworkError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for network errors
	return err.WithContext("recovery", "Check network connectivity and server status")
}

// HandleStorageError handles storage errors
func HandleStorageError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for storage errors
	return err.WithContext("recovery", "Check disk space and file permissions")
}

// HandleAPIError handles API errors
func HandleAPIError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for API errors
	return err.WithContext("recovery", "Check API documentation and request parameters")
}

// HandleValidationError handles validation errors
func HandleValidationError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for validation errors
	return err.WithContext("recovery", "Check input parameters and data formats")
}

// HandleCrawlerError handles crawler errors
func HandleCrawlerError(err *CrawlrError) error {
	// Add additional context or perform recovery actions for crawler errors
	return err.WithContext("recovery", "Check crawler configuration and target website accessibility")
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	*CrawlrError
	MaxRetries int
	RetryCount int
}

// NewRetryableError creates a new retryable error
func NewRetryableError(errorType ErrorType, message string, maxRetries int) *RetryableError {
	return &RetryableError{
		CrawlrError: New(errorType, message),
		MaxRetries:  maxRetries,
		RetryCount:  0,
	}
}

// WrapRetryableError wraps an existing error as a retryable error
func WrapRetryableError(err error, errorType ErrorType, message string, maxRetries int) *RetryableError {
	return &RetryableError{
		CrawlrError: Wrap(err, errorType, message),
		MaxRetries:  maxRetries,
		RetryCount:  0,
	}
}

// CanRetry checks if the error can be retried
func (e *RetryableError) CanRetry() bool {
	return e.RetryCount < e.MaxRetries
}

// IncrementRetry increments the retry count
func (e *RetryableError) IncrementRetry() {
	e.RetryCount++
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	_, ok := err.(*RetryableError)
	return ok
}

// GetRetryCount returns the retry count for a retryable error
func GetRetryCount(err error) int {
	if retryErr, ok := err.(*RetryableError); ok {
		return retryErr.RetryCount
	}
	return 0
}
