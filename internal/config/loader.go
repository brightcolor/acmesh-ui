package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses config.yaml at path, applying defaults for any unset
// fields, then validates the result.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	// Decode on top of defaults so omitted keys keep their default value.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	applyDerivedDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyDerivedDefaults fills in fields that depend on other fields.
func applyDerivedDefaults(c *Config) {
	if c.Server.Port == 0 {
		c.Server.Port = 8090
	}
	if c.Server.Bind == "" {
		c.Server.Bind = "127.0.0.1"
	}
	if c.Auth.Mode == "" {
		c.Auth.Mode = "none"
	}
	if c.Jobs.MaxParallel <= 0 {
		c.Jobs.MaxParallel = 2
	}
	if c.Jobs.TimeoutSeconds <= 0 {
		c.Jobs.TimeoutSeconds = 1800
	}
	if c.Jobs.LogRetentionDays <= 0 {
		c.Jobs.LogRetentionDays = 30
	}
	if c.Certs.ExpiringSoonDays <= 0 {
		c.Certs.ExpiringSoonDays = 30
	}
	if c.Data.Dir == "" {
		c.Data.Dir = "/var/lib/acmesh-ui"
	}
	if c.UI.Title == "" {
		c.UI.Title = "acmesh-ui"
	}
}

// Marshal renders the config back to YAML (used by `init`).
func Marshal(c Config) ([]byte, error) {
	return yaml.Marshal(c)
}
