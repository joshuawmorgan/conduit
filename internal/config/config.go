// Package config loads layered Conduit configuration using Koanf. Precedence
// (low to high): embedded defaults -> conduit.yaml -> CONDUIT_* environment.
package config

import (
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the typed Conduit configuration.
type Config struct {
	Concurrency int      `koanf:"concurrency"`
	StateDir    string   `koanf:"state_dir"`
	PluginDirs  []string `koanf:"plugin_dirs"`
	Output      string   `koanf:"output"` // table|json|yaml
	Telemetry   bool     `koanf:"telemetry"`
	LogLevel    string   `koanf:"log_level"`
}

// Defaults returns the built-in defaults.
func Defaults() map[string]any {
	return map[string]any{
		"concurrency": 0, // 0 => NumCPU
		"state_dir":   ".conduit/runs",
		"plugin_dirs": []string{".conduit/plugins"},
		"output":      "table",
		"telemetry":   false,
		"log_level":   "info",
	}
}

// Load builds a Config from defaults, an optional YAML file, and environment.
// A missing config file is not an error.
func Load(path string) (*Config, error) {
	k := koanf.New(".")
	_ = k.Load(confmap.Provider(Defaults(), "."), nil)

	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			// Ignore not-found; surface parse errors.
			if !strings.Contains(err.Error(), "no such file") &&
				!strings.Contains(err.Error(), "cannot find the file") &&
				!strings.Contains(err.Error(), "The system cannot find") {
				return nil, err
			}
		}
	}

	// CONDUIT_CONCURRENCY -> concurrency, CONDUIT_STATE_DIR -> state_dir, ...
	_ = k.Load(env.Provider("CONDUIT_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "CONDUIT_")), "__", ".")
	}), nil)

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
