package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	// LvOff logging disabled
	LvOff = iota
	// LvErr errors
	LvErr
	// LvWarn warnings
	LvWarn
	// LvInfo information
	LvInfo
	// LvDebug debug
	LvDebug
)

// Logger is a logger
type Logger struct {
	name  string
	level int
	file  *log.Logger
}

// New creates a new logger
func New(name string, level int, dirPath string) (*Logger, error) {
	var lFile *log.Logger

	if dirPath != "" {
		f, err := os.Create(filepath.Join(dirPath, name+".log"))
		if err != nil {
			return nil, err
		}
		lFile = log.New(f, "", log.LstdFlags)
	}

	return &Logger{name, level, lFile}, nil
}

// SetLevel sets logging level
func (l *Logger) SetLevel(level int) {
	l.level = level
}

// Log logs a message
func (l *Logger) Log(level, f string, v ...interface{}) {
	f = fmt.Sprintf("[%s] %s: %s", level, l.name, f)
	log.Printf(f, v...)
	if l.file != nil {
		l.file.Printf(f, v...)
	}
}

// Err logs an error
func (l *Logger) Err(f string, v ...interface{}) {
	if l.level >= LvErr {
		l.Log("E", f, v...)
	}
}

// Warn logs a warning
func (l *Logger) Warn(f string, v ...interface{}) {
	if l.level >= LvWarn {
		l.Log("W", f, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(f string, v ...interface{}) {
	if l.level >= LvInfo {
		l.Log("I", f, v...)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(f string, v ...interface{}) {
	if l.level >= LvDebug {
		l.Log("D", f, v...)
	}
}
