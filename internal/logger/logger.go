package logger

import (
	"log"
	"os"
)

var (
	isProduction bool
	debugLogger  *log.Logger
	infoLogger   *log.Logger
	errorLogger  *log.Logger
)

func init() {
	// Check if PRODUCTION environment variable is set
	isProduction = os.Getenv("PRODUCTION") != ""

	// Initialize loggers with prefixes
	debugLogger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile)
	infoLogger = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
	errorLogger = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile)
}

// Debug logs debug messages only when not in production
func Debug(format string, v ...interface{}) {
	if !isProduction {
		debugLogger.Printf(format, v...)
	}
}

// Debugf is an alias for Debug
func Debugf(format string, v ...interface{}) {
	Debug(format, v...)
}

// Info logs informational messages
func Info(format string, v ...interface{}) {
	infoLogger.Printf(format, v...)
}

// Infof is an alias for Info
func Infof(format string, v ...interface{}) {
	Info(format, v...)
}

// Error logs error messages
func Error(format string, v ...interface{}) {
	errorLogger.Printf(format, v...)
}

// Errorf is an alias for Error
func Errorf(format string, v ...interface{}) {
	Error(format, v...)
}

// Fatal logs a fatal error and exits
func Fatal(format string, v ...interface{}) {
	errorLogger.Fatalf(format, v...)
}

// Fatalf is an alias for Fatal
func Fatalf(format string, v ...interface{}) {
	Fatal(format, v...)
}

// IsProduction returns true if running in production mode
func IsProduction() bool {
	return isProduction
}
