// Package config handles configuration loading from TOML files and environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration structure.
type Config struct {
	Swarm     SwarmConfig               `toml:"swarm"`
	Providers map[string]ProviderConfig `toml:"providers"`
	MCP       MCPConfig                 `toml:"mcp"`
}

// SwarmConfig holds swarm-related settings.
type SwarmConfig struct {
	MaxMyses        int    `toml:"max_myses"`
	DefaultProvider string `toml:"default_provider"`
}

// ProviderConfig holds LLM provider settings.
type ProviderConfig struct {
	Endpoint    string  `toml:"endpoint"`
	Model       string  `toml:"model"`
	APIKeyName  string  `toml:"api_key_name"`
	Temperature float64 `toml:"temperature"`
}

// MCPConfig holds MCP proxy settings.
type MCPConfig struct {
	Upstream string `toml:"upstream"`
}

// Load reads configuration from a TOML file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Providers: make(map[string]ProviderConfig),
	}

	// Config file is required
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// File must exist
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Load from file
	_, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate returns an error if the configuration is invalid.
func (c *Config) Validate() error {
	var errs []error

	if c.Swarm.MaxMyses < 1 || c.Swarm.MaxMyses > 100 {
		errs = append(errs, fmt.Errorf("swarm.max_myses=%d must be between 1 and 100", c.Swarm.MaxMyses))
	}

	if len(c.Providers) == 0 {
		errs = append(errs, errors.New("providers: at least one provider must be configured"))
	} else {
		for name, providerCfg := range c.Providers {
			errs = append(errs, validateProviderConfig(name, providerCfg)...)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func validateProviderConfig(name string, cfg ProviderConfig) []error {
	var errs []error
	if cfg.Endpoint == "" {
		errs = append(errs, fmt.Errorf("providers.%s.endpoint is required", name))
	} else if err := validateEndpoint(cfg.Endpoint); err != nil {
		errs = append(errs, fmt.Errorf("providers.%s.endpoint=%q is invalid: %v", name, cfg.Endpoint, err))
	}

	if cfg.Model == "" {
		errs = append(errs, fmt.Errorf("providers.%s.model is required", name))
	}

	if cfg.Temperature < 0.0 || cfg.Temperature > 2.0 {
		errs = append(errs, fmt.Errorf("providers.%s.temperature=%v must be between 0.0 and 2.0", name, cfg.Temperature))
	}

	return errs
}

func validateEndpoint(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("missing scheme or host")
	}
	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func applyEnvOverrides(cfg *Config) {
	applyInt := func(value string, set func(int)) {
		if value == "" {
			return
		}
		if n, err := strconv.Atoi(value); err == nil {
			set(n)
		}
	}

	for _, setter := range []struct {
		env   string
		apply func(string)
	}{
		{"ZOEA_MAX_MYSES", func(v string) { applyInt(v, func(n int) { cfg.Swarm.MaxMyses = n }) }},
		{"ZOEA_MCP_ENDPOINT", func(v string) {
			if v != "" {
				cfg.MCP.Upstream = v
			}
		}},
	} {
		setter.apply(os.Getenv(setter.env))
	}
}

// DataDir returns the path to the Zoea data directory (~/.zoea-nova).
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zoea-nova"), nil
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
