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
	MaxMyses int `toml:"max_myses"`
}

// ProviderConfig holds LLM provider settings.
type ProviderConfig struct {
	Endpoint    string  `toml:"endpoint"`
	Model       string  `toml:"model"`
	Temperature float64 `toml:"temperature"`
	RateLimit   float64 `toml:"rate_limit"`
	RateBurst   int     `toml:"rate_burst"`
}

// MCPConfig holds MCP proxy settings.
type MCPConfig struct {
	Upstream string `toml:"upstream"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Swarm: SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]ProviderConfig{
			"ollama": {
				Endpoint:    "http://localhost:11434",
				Model:       "llama3",
				Temperature: 0.7,
				RateLimit:   2.0,
				RateBurst:   3,
			},
			"opencode_zen": {
				Endpoint:    "https://api.opencode.ai/v1",
				Model:       "gpt-5-nano",
				Temperature: 0.7,
				RateLimit:   10.0,
				RateBurst:   5,
			},
		},
		MCP: MCPConfig{
			Upstream: "https://game.spacemolt.com/mcp",
		},
	}
}

// Load reads configuration from a TOML file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	defaults := DefaultConfig()
	cfg := DefaultConfig()
	var meta toml.MetaData
	var loaded bool

	// Load from file if it exists
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			decoded, err := toml.DecodeFile(path, cfg)
			if err != nil {
				return nil, err
			}
			meta = decoded
			loaded = true
		}
	}

	if loaded {
		applyProviderDefaults(cfg, defaults, meta)
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

	if cfg.RateLimit <= 0 {
		errs = append(errs, fmt.Errorf("providers.%s.rate_limit=%v must be greater than 0", name, cfg.RateLimit))
	}
	if cfg.RateBurst < 1 {
		errs = append(errs, fmt.Errorf("providers.%s.rate_burst=%d must be at least 1", name, cfg.RateBurst))
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

func applyProviderDefaults(cfg *Config, defaults *Config, meta toml.MetaData) {
	if cfg == nil || defaults == nil {
		return
	}

	for name, fallback := range defaults.Providers {
		current, ok := cfg.Providers[name]
		if !ok {
			cfg.Providers[name] = fallback
			continue
		}

		if !meta.IsDefined("providers", name, "endpoint") {
			current.Endpoint = fallback.Endpoint
		}
		if !meta.IsDefined("providers", name, "model") {
			current.Model = fallback.Model
		}
		if !meta.IsDefined("providers", name, "temperature") {
			current.Temperature = fallback.Temperature
		}
		if !meta.IsDefined("providers", name, "rate_limit") {
			current.RateLimit = fallback.RateLimit
		}
		if !meta.IsDefined("providers", name, "rate_burst") {
			current.RateBurst = fallback.RateBurst
		}

		cfg.Providers[name] = current
	}
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func applyEnvOverrides(cfg *Config) {
	updateProvider := func(name string, update func(ProviderConfig) ProviderConfig) {
		providerCfg, ok := cfg.Providers[name]
		if !ok {
			return
		}
		cfg.Providers[name] = update(providerCfg)
	}

	applyInt := func(value string, set func(int)) {
		if value == "" {
			return
		}
		if n, err := strconv.Atoi(value); err == nil {
			set(n)
		}
	}

	applyFloat := func(value string, set func(float64)) {
		if value == "" {
			return
		}
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			set(f)
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
		{"ZOEA_OLLAMA_ENDPOINT", func(v string) {
			if v != "" {
				updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.Endpoint = v; return p })
			}
		}},
		{"ZOEA_OLLAMA_MODEL", func(v string) {
			if v != "" {
				updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.Model = v; return p })
			}
		}},
		{"ZOEA_OLLAMA_TEMPERATURE", func(v string) {
			applyFloat(v, func(f float64) {
				updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.Temperature = f; return p })
			})
		}},
		{"ZOEA_OLLAMA_RATE_LIMIT", func(v string) {
			applyFloat(v, func(f float64) {
				updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.RateLimit = f; return p })
			})
		}},
		{"ZOEA_OLLAMA_RATE_BURST", func(v string) {
			applyInt(v, func(n int) {
				updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.RateBurst = n; return p })
			})
		}},
		{"ZOEA_OPENCODE_ENDPOINT", func(v string) {
			if v != "" {
				updateProvider("opencode_zen", func(p ProviderConfig) ProviderConfig { p.Endpoint = v; return p })
			}
		}},
		{"ZOEA_OPENCODE_MODEL", func(v string) {
			if v != "" {
				updateProvider("opencode_zen", func(p ProviderConfig) ProviderConfig { p.Model = v; return p })
			}
		}},
		{"ZOEA_OPENCODE_TEMPERATURE", func(v string) {
			applyFloat(v, func(f float64) {
				updateProvider("opencode_zen", func(p ProviderConfig) ProviderConfig { p.Temperature = f; return p })
			})
		}},
		{"ZOEA_OPENCODE_RATE_LIMIT", func(v string) {
			applyFloat(v, func(f float64) {
				updateProvider("opencode_zen", func(p ProviderConfig) ProviderConfig { p.RateLimit = f; return p })
			})
		}},
		{"ZOEA_OPENCODE_RATE_BURST", func(v string) {
			applyInt(v, func(n int) {
				updateProvider("opencode_zen", func(p ProviderConfig) ProviderConfig { p.RateBurst = n; return p })
			})
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
