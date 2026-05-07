package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

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
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func ParseLogLevel(level string) LogLevel {
	switch level {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return INFO
	}
}

type Logger struct {
	mu       sync.Mutex
	level    LogLevel
	format   string
	output   io.Writer
	module   string
	fields   map[string]interface{}
}

func NewLogger(module string, level LogLevel, format string) *Logger {
	return &Logger{
		level:  level,
		format: format,
		output: os.Stdout,
		module: module,
		fields: make(map[string]interface{}),
	}
}

func DefaultLogger() *Logger {
	return NewLogger("viri", INFO, "text")
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = output
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	fields := make(map[string]interface{})
	for k, v := range l.fields {
		fields[k] = v
	}
	fields[key] = value

	return &Logger{
		level:  l.level,
		format: l.format,
		output: l.output,
		module: l.module,
		fields: fields,
	}
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	mergedFields := make(map[string]interface{})
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	return &Logger{
		level:  l.level,
		format: l.format,
		output: l.output,
		module: l.module,
		fields: mergedFields,
	}
}

func (l *Logger) Debug(msg string) {
	l.log(DEBUG, msg, nil)
}

func (l *Logger) Info(msg string) {
	l.log(INFO, msg, nil)
}

func (l *Logger) Warn(msg string) {
	l.log(WARN, msg, nil)
}

func (l *Logger) Error(msg string) {
	l.log(ERROR, msg, nil)
}

func (l *Logger) Fatal(msg string) {
	l.log(FATAL, msg, nil)
	os.Exit(1)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

func (l *Logger) log(level LogLevel, msg string, extraFields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	allFields := make(map[string]interface{})
	for k, v := range l.fields {
		allFields[k] = v
	}
	for k, v := range extraFields {
		allFields[k] = v
	}

	var logLine string
	if l.format == "json" {
		logLine = l.formatJSON(timestamp, level, msg, allFields)
	} else {
		logLine = l.formatText(timestamp, level, msg, allFields)
	}

	fmt.Fprintln(l.output, logLine)

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) formatText(timestamp string, level LogLevel, msg string, fields map[string]interface{}) string {
	line := fmt.Sprintf("%s [%s] %s: %s", timestamp, level.String(), l.module, msg)

	if len(fields) > 0 {
		line += " {"
		first := true
		for k, v := range fields {
			if !first {
				line += ", "
			}
			line += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		line += "}"
	}

	return line
}

func (l *Logger) formatJSON(timestamp string, level LogLevel, msg string, fields map[string]interface{}) string {
	line := fmt.Sprintf(`{"time":"%s","level":"%s","module":"%s","msg":"%s"`, timestamp, level.String(), l.module, msg)

	if len(fields) > 0 {
		line += `,"fields":{`
		first := true
		for k, v := range fields {
			if !first {
				line += ","
			}
			line += fmt.Sprintf(`"%s":"%v"`, k, v)
			first = false
		}
		line += "}"
	}

	line += "}"
	return line
}
