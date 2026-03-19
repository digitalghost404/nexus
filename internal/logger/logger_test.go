package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugWritesToStderr(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Debug: true, Stderr: &buf})
	l.Debug("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected debug output, got: %s", buf.String())
	}
}

func TestDebugSilentWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Debug: false, Stderr: &buf})
	l.Debug("test message")
	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}

func TestFileLogging(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nexus.log")
	l := New(Config{LogFile: logPath})
	l.Error("file error message")
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(string(data), "file error message") {
		t.Errorf("expected log content, got: %s", string(data))
	}
}

func TestLogRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nexus.log")
	l := New(Config{LogFile: logPath, MaxSize: 100})

	for i := 0; i < 20; i++ {
		l.Error("this is a long enough message to fill the log file quickly")
	}
	l.Close()

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file missing: %v", err)
	}
	if info.Size() > 200 {
		t.Errorf("log file too large after rotation: %d bytes", info.Size())
	}
}
