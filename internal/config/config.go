package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	Idle  int `yaml:"idle"`
	Stale int `yaml:"stale"`
}

type Config struct {
	Roots         []string   `yaml:"roots"`
	Exclude       []string   `yaml:"exclude"`
	Thresholds    Thresholds `yaml:"thresholds"`
	ScanInterval  string     `yaml:"scan_interval"`
	Agent         string     `yaml:"agent"`
	ServePort     int        `yaml:"serve_port"`
	OllamaModel   string     `yaml:"ollama_model"`
	OllamaURL     string     `yaml:"ollama_url"`
	CaptureSource string     `yaml:"capture_source"`
	CaptureDir    string     `yaml:"capture_dir"`
	OfflineMode   bool       `yaml:"offline_mode"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Roots: []string{},
		Exclude: []string{
			"*/node_modules/*",
			"*/vendor/*",
			"*/.cache/*",
			"*/go/pkg/*",
			"*/snap/*",
			"*/.nvm/*",
		},
		Thresholds: Thresholds{
			Idle:  3,
			Stale: 14,
		},
		ScanInterval:  "30m",
		Agent:         "claude",
		ServePort:     7600,
		OllamaModel:   "nomic-embed-text",
		OllamaURL:     "http://localhost:11434",
		CaptureSource: "claude",
		CaptureDir:    filepath.Join(home, ".claude"),
		OfflineMode:   true,
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// After unmarshal, merge default exclusions with user-provided ones
	defaults := Default()
	if len(cfg.Exclude) > 0 {
		// Merge: keep defaults, add any user patterns not already in defaults
		merged := make(map[string]bool)
		for _, e := range defaults.Exclude {
			merged[e] = true
		}
		for _, e := range cfg.Exclude {
			merged[e] = true
		}
		cfg.Exclude = make([]string, 0, len(merged))
		for e := range merged {
			cfg.Exclude = append(cfg.Exclude, e)
		}
		sort.Strings(cfg.Exclude)
	} else {
		cfg.Exclude = defaults.Exclude
	}

	if cfg.Thresholds.Idle == 0 {
		cfg.Thresholds.Idle = 3
	}
	if cfg.Thresholds.Stale == 0 {
		cfg.Thresholds.Stale = 14
	}
	if cfg.ScanInterval == "" {
		cfg.ScanInterval = "30m"
	}

	for i, r := range cfg.Roots {
		cfg.Roots[i] = ExpandPath(r)
	}

	// Env var overrides (runtime config takes precedence over YAML)
	if v := os.Getenv("NEXUS_EMBEDDING_MODEL"); v != "" {
		cfg.OllamaModel = v
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func (cfg Config) NexusDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nexus", cfg.Agent)
}

func (cfg Config) ConfigPath() string {
	return filepath.Join(cfg.NexusDir(), "config.yaml")
}

func (cfg Config) DBPath() string {
	return filepath.Join(cfg.NexusDir(), "nexus.db")
}

func (cfg Config) LogPath() string {
	return filepath.Join(cfg.NexusDir(), "nexus.log")
}
