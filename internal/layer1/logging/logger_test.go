package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level  LogLevel
		expect string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
	}

	for _, tt := range tests {
		if tt.level.String() != tt.expect {
			t.Errorf("Expected %s, got %s", tt.expect, tt.level.String())
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input  string
		expect LogLevel
	}{
		{"debug", DEBUG},
		{"info", INFO},
		{"warn", WARN},
		{"error", ERROR},
		{"fatal", FATAL},
		{"invalid", INFO},
	}

	for _, tt := range tests {
		result := ParseLogLevel(tt.input)
		if result != tt.expect {
			t.Errorf("ParseLogLevel(%s): expected %v, got %v", tt.input, tt.expect, result)
		}
	}
}

func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "text")
	log.SetOutput(&buf)

	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Error("Output should contain log level")
	}
	if !strings.Contains(output, "test-module") {
		t.Error("Output should contain module name")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Output should contain message")
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "json")
	log.SetOutput(&buf)

	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Error("JSON output should contain log level")
	}
	if !strings.Contains(output, `"module":"test-module"`) {
		t.Error("JSON output should contain module name")
	}
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Error("JSON output should contain message")
	}
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "text")
	log.SetOutput(&buf)

	log.WithField("key", "value").Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Error("Output should contain the field")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "text")
	log.SetOutput(&buf)

	log.WithFields(map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}).Info("test message")

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Error("Output should contain key1")
	}
	if !strings.Contains(output, "key2=value2") {
		t.Error("Output should contain key2")
	}
}

func TestLoggerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", WARN, "text")
	log.SetOutput(&buf)

	log.Debug("should not appear")
	log.Info("should not appear")

	output := buf.String()
	if output != "" {
		t.Error("DEBUG and INFO should not appear when level is WARN")
	}

	log.Warn("should appear")
	output = buf.String()
	if !strings.Contains(output, "[WARN]") {
		t.Error("WARN should appear")
	}
}

func TestLoggerFormatFunctions(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "text")
	log.SetOutput(&buf)

	log.Infof("formatted %s %d", "message", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted message 42") {
		t.Error("Output should contain formatted message")
	}
}

func TestDefaultLogger(t *testing.T) {
	log := DefaultLogger()
	if log == nil {
		t.Fatal("DefaultLogger returned nil")
	}
	if log.level != INFO {
		t.Errorf("Expected INFO level, got %v", log.level)
	}
}

func TestLoggerChainedFields(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("test-module", DEBUG, "text")
	log.SetOutput(&buf)

	log.WithField("first", "a").WithField("second", "b").Info("chained")

	output := buf.String()
	if !strings.Contains(output, "first=a") {
		t.Error("Output should contain first field")
	}
	if !strings.Contains(output, "second=b") {
		t.Error("Output should contain second field")
	}
}
