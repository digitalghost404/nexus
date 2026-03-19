package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if len(cfg.Roots) != 0 {
		t.Errorf("expected no default roots, got %v", cfg.Roots)
	}
	if cfg.Thresholds.Idle != 3 {
		t.Errorf("expected idle=3, got %d", cfg.Thresholds.Idle)
	}
	if cfg.Thresholds.Stale != 14 {
		t.Errorf("expected stale=14, got %d", cfg.Thresholds.Stale)
	}
	if cfg.ScanInterval != "30m" {
		t.Errorf("expected scan_interval=30m, got %s", cfg.ScanInterval)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
roots:
  - /tmp/projects
exclude:
  - "*/node_modules/*"
thresholds:
  idle: 5
  stale: 21
scan_interval: 15m
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "/tmp/projects" {
		t.Errorf("unexpected roots: %v", cfg.Roots)
	}
	if cfg.Thresholds.Idle != 5 {
		t.Errorf("expected idle=5, got %d", cfg.Thresholds.Idle)
	}
	if cfg.ScanInterval != "15m" {
		t.Errorf("expected 15m, got %s", cfg.ScanInterval)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Thresholds.Idle != 3 {
		t.Errorf("expected default idle=3, got %d", cfg.Thresholds.Idle)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/projects")
	expected := filepath.Join(home, "projects")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
