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
    "bufio"
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
    NONE // To entirely disable logging for the Logger

    DefaultLevel = INFO
    DefaultFlags = log.Lmsgprefix | log.Ltime | log.Ldate | log.Lshortfile
)

type LogLevel = int8

type Log struct {
    *log.Logger
    Level LogLevel
    Name  string
}

func (l *Log) shouldDebug(level LogLevel) bool {
    return l.Level <= level
}

func (l *Log) Trace(format string, v ...interface{}) {
    if l.shouldDebug(TRACE) {
        format = l.Name + "[TRACE] " + format
        l.Output(2, fmt.Sprintf(format, v...))
    }
}

func (l *Log) Debug(format string, v ...interface{}) {
    if l.shouldDebug(DEBUG) {
        format = l.Name + "[DEBUG] " + format
        l.Output(2, fmt.Sprintf(format, v...))
    }
}

func (l *Log) Info(format string, v ...interface{}) {
    if l.shouldDebug(INFO) {
        format = l.Name + "[INFO] " + format
        l.Output(2, fmt.Sprintf(format, v...))
    }
}

func (l *Log) Warn(format string, v ...interface{}) {
    if l.shouldDebug(WARN) {
        format = l.Name + "[WARN] " + format
        l.Output(2, fmt.Sprintf(format, v...))
    }
}

func (l *Log) Error(format string, v ...interface{}) {
    if l.shouldDebug(ERROR) {
        format = l.Name + "[ERROR] " + format
        l.Output(2, fmt.Sprintf(format, v...))
    }
}

func (l *Log) Fatal(err error) {
    l.Output(2, fmt.Sprintf("[FATAL] %s", err.Error()))
    os.Exit(1)
}

func NewLog(name string) *Log {
    return &Log{
        log.New(os.Stderr, "", DefaultFlags),
        DefaultLevel,
        name,
    }
}

func NewLogToWriter(writer io.Writer, name string) *Log {
    return &Log{
        log.New(writer, "", DefaultFlags),
        DefaultLevel,
        name,
    }
}

// RootLog is the default log
var RootLog = NewLog("ROOT")

var none, _ = os.Open(os.DevNull)

// NullLog A log to nowhere.
//
// **NOTE** this is NOT the same as a `NONE` LogLevel,
// logs will still be generated and emitted, but they won't be visible anywhere.
var NullLog = NewLogToWriter(bufio.NewWriter(none), "nil")

// A Loggable type is one that has a Log and exposes it to its clients
type Loggable interface {
    SetLogLevel(level LogLevel)
}
