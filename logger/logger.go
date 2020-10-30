package logger

import (
	"fmt"
	"log"
)

const (
	LvOff = iota
	LvErr
	LvWarn
	LvInfo
	LvDebug
)

type Logger struct {
	name  string
	level int
}

func New(name string, level int) *Logger {
	return &Logger{name, level}
}

func (l *Logger) SetLevel(level int) {
	l.level = level
}

func (c *Logger) Log(level, f string, v ...interface{}) {
	f = fmt.Sprintf("[%s] %s: %s", level, c.name, f)
	log.Printf(f, v...)
}

func (c *Logger) Err(f string, v ...interface{}) {
	if c.level >= LvErr {
		c.Log("E", f, v...)
	}
}

func (c *Logger) Warn(f string, v ...interface{}) {
	if c.level >= LvWarn {
		c.Log("W", f, v...)
	}
}

func (c *Logger) Info(f string, v ...interface{}) {
	if c.level >= LvInfo {
		c.Log("I", f, v...)
	}
}

func (c *Logger) Debug(f string, v ...interface{}) {
	if c.level >= LvDebug {
		c.Log("D", f, v...)
	}
}
