package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Config struct {
	Debug   bool
	Stderr  io.Writer
	LogFile string
	MaxSize int64
}

type Logger struct {
	cfg     Config
	stderr  io.Writer
	file    *os.File
	mu      sync.Mutex
	written int64
}

func New(cfg Config) *Logger {
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 1 << 20
	}

	l := &Logger{cfg: cfg, stderr: cfg.Stderr}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			l.file = f
			info, _ := f.Stat()
			if info != nil {
				l.written = info.Size()
			}
		}
	}

	return l
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if !l.cfg.Debug {
		return
	}
	formatted := fmt.Sprintf(msg, args...)
	fmt.Fprintf(l.stderr, "[DEBUG] %s\n", formatted)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.writeToFile(fmt.Sprintf("[ERROR] %s %s\n", time.Now().Format(time.RFC3339), formatted))
}

func (l *Logger) Info(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.writeToFile(fmt.Sprintf("[INFO]  %s %s\n", time.Now().Format(time.RFC3339), formatted))
}

func (l *Logger) writeToFile(line string) {
	if l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.written >= l.cfg.MaxSize {
		l.rotate()
	}

	n, _ := l.file.WriteString(line)
	l.written += int64(n)
}

func (l *Logger) rotate() {
	l.file.Close()
	l.file = nil
	os.Truncate(l.cfg.LogFile, 0)
	f, err := os.OpenFile(l.cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		l.file = f
		l.written = 0
	}
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
