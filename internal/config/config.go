package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultHost = "https://play.kunjani.co"

type Config struct {
	Host  string `yaml:"host,omitempty"`
	Token string `yaml:"token,omitempty"`

	path string
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".kunjani", "config.yml")
}

func Load(path string) (*Config, error) {
	c := &Config{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) == 0 {
		return c, nil
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

func (c *Config) Path() string {
	return c.path
}

func (c *Config) EffectiveHost() string {
	if c.Host == "" {
		return DefaultHost
	}
	return c.Host
}

func (c *Config) Configured() bool {
	return c.Token != ""
}

func (c *Config) Save() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", c.path, err)
	}
	if err := os.Chmod(c.path, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", c.path, err)
	}
	return nil
}
