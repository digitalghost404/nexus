// internal/config/config_merge_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMergesUserExcludeWithDefaults verifies that when a user provides
// custom exclude patterns in their config file, the result contains BOTH the
// user patterns AND all default patterns. No defaults must be dropped.
func TestLoadMergesUserExcludeWithDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
roots:
  - /tmp/projects
exclude:
  - "*/custom/*"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// The user pattern must be present.
	if !containsExclude(cfg.Exclude, "*/custom/*") {
		t.Errorf("expected '*/custom/*' in exclude list, got: %v", cfg.Exclude)
	}

	// All default patterns must also be present.
	defaults := Default()
	for _, d := range defaults.Exclude {
		if !containsExclude(cfg.Exclude, d) {
			t.Errorf("default exclude pattern %q was dropped; full list: %v", d, cfg.Exclude)
		}
	}

	// Spot-check the two patterns mentioned in the task spec explicitly.
	for _, required := range []string{"*/vendor/*", "*/node_modules/*"} {
		if !containsExclude(cfg.Exclude, required) {
			t.Errorf("expected %q to survive merge, got: %v", required, cfg.Exclude)
		}
	}
}

func containsExclude(patterns []string, target string) bool {
	for _, p := range patterns {
		if p == target {
			return true
		}
	}
	return false
}
