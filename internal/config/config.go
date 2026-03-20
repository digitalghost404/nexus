package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	Idle  int `yaml:"idle"`
	Stale int `yaml:"stale"`
}

type Config struct {
	Roots        []string   `yaml:"roots"`
	Exclude      []string   `yaml:"exclude"`
	Thresholds   Thresholds `yaml:"thresholds"`
	ScanInterval string     `yaml:"scan_interval"`
}

func Default() Config {
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
		ScanInterval: "30m",
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

	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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

func NexusDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nexus")
}

func ConfigPath() string {
	return filepath.Join(NexusDir(), "config.yaml")
}

func DBPath() string {
	return filepath.Join(NexusDir(), "nexus.db")
}

func LogPath() string {
	return filepath.Join(NexusDir(), "nexus.log")
}
