package logging

import (
	"fmt"
	"io"
	"log"
	"os"
)

const (
	TRACE = iota
	DEBUG
	INFO
	WARN
	ERROR

	DefaultLevel = INFO
	DefaultFlags = log.Lmsgprefix | log.Ltime | log.Ldate | log.Lshortfile
)

type LogLevel = int8

type Log struct {
	*log.Logger
	level LogLevel
}

func (l *Log) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Log) Trace(format string, v ...interface{}) {
	if l.level <= TRACE {
		l.SetPrefix("[TRACE] ")
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Debug(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.SetPrefix("[DEBUG] ")
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Info(format string, v ...interface{}) {
	if l.level <= INFO {
		l.SetPrefix("[INFO] ")
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Warn(format string, v ...interface{}) {
	if l.level <= WARN {
		l.SetPrefix("[WARN] ")
		l.Output(2, fmt.Sprintf(format, v...))

	}
}

func (l *Log) Error(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.SetPrefix("[ERROR] ")
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Fatal(err error) {
	l.SetPrefix("[FATAL] ")
	l.Output(2, fmt.Sprintf(err.Error()))
	os.Exit(1)
}

func NewLog() *Log {
	return &Log{
		log.New(os.Stderr, "", DefaultFlags),
		DefaultLevel,
	}
}

func NewLogToWriter(writer io.Writer) *Log {
	return &Log{
		log.New(writer, "", DefaultFlags),
		DefaultLevel,
	}
}
