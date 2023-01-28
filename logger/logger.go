package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	LvDisabled = iota
	LvFatal
	LvErr
	LvWarn
	LvInfo
	LvDebug
)

// Logger is a logger
type Logger struct {
	name  string
	level int
	file  *log.Logger
}

// New creates a new logger
func New(name string, level int, dirPath, fName string) (*Logger, error) {
	var lFile *log.Logger

	if fName == "" {
		fName = name + ".log"
	}

	if dirPath != "" {
		f, err := os.OpenFile(filepath.Join(dirPath, fName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		lFile = log.New(f, "", log.LstdFlags)
	}

	return &Logger{name, level, lFile}, nil
}

// Copy creates a copy of the logger
func (l *Logger) Copy(name string, level int) *Logger {
	return &Logger{
		name:  name,
		level: level,
		file:  l.file,
	}
}

// Name returns logger name
func (l *Logger) Name() string {
	return l.name
}

// SetName sets logger name
func (l *Logger) SetName(name string) {
	l.name = name
}

// Level returns logging level
func (l *Logger) Level() int {
	return l.level
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

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(f string, v ...interface{}) {
	if l.level >= LvFatal {
		l.Log("F", f, v...)
	}
	os.Exit(1)
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
