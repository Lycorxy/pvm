package logger

import (
	"fmt"
	"os"
)

// Level represents the logging verbosity level
type Level int

const (
	LevelQuiet   Level = 0
	LevelNormal  Level = 1
	LevelVerbose Level = 2
)

var currentLevel = LevelNormal

// SetLevel sets the global log level
func SetLevel(level Level) {
	currentLevel = level
}

// GetLevel returns the current log level
func GetLevel() Level {
	return currentLevel
}

// Info prints a message at normal level
func Info(format string, args ...interface{}) {
	if currentLevel >= LevelNormal {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

// Verbose prints a message at verbose level
func Verbose(format string, args ...interface{}) {
	if currentLevel >= LevelVerbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

// Error prints an error message (always shown)
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// ProgressF
// ProgressF prints a progress line (replaces current line) at normal level
func ProgressF(format string, args ...interface{}) {
	if currentLevel >= LevelNormal {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// Warn prints a warning message at normal level
func Warn(format string, args ...interface{}) {
	if currentLevel >= LevelNormal {
		fmt.Fprintf(os.Stderr, "[warning] "+format+"\n", args...)
	}
}
