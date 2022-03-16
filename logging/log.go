/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

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
	level   LogLevel
	Enabled bool
}

func (l *Log) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Log) Enable(enabled bool) {
	l.Enabled = enabled
}

func (l *Log) shouldDebug(level LogLevel) bool {
	return l.Enabled && l.level <= level
}

func (l *Log) Trace(format string, v ...interface{}) {
	if l.shouldDebug(TRACE) {
		format = "[TRACE] " + format
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Debug(format string, v ...interface{}) {
	if l.shouldDebug(DEBUG) {
		format = "[DEBUG] " + format
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Info(format string, v ...interface{}) {
	if l.shouldDebug(INFO) {
		format = "[INFO] " + format
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Warn(format string, v ...interface{}) {
	if l.shouldDebug(WARN) {
		format = "[WARN] " + format
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Error(format string, v ...interface{}) {
	if l.shouldDebug(ERROR) {
		format = "[ERROR] " + format
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

func (l *Log) Fatal(err error) {
	l.Output(2, fmt.Sprintf("[FATAL] %s", err.Error()))
	os.Exit(1)
}

func NewLog() *Log {
	return &Log{
		log.New(os.Stderr, "", DefaultFlags),
		DefaultLevel,
		true,
	}
}

func NewLogToWriter(writer io.Writer) *Log {
	return &Log{
		log.New(writer, "", DefaultFlags),
		DefaultLevel,
		true,
	}
}
