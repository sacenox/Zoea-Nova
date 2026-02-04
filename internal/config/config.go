// Package config handles configuration loading from TOML files and environment variables.
package config

import (
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
				Model:       "glm-4.7-free",
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
	cfg := DefaultConfig()

	// Load from file if it exists
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, cfg); err != nil {
				return nil, err
			}
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ZOEA_MAX_MYSES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Swarm.MaxMyses = n
		}
	}

	if v := os.Getenv("ZOEA_MCP_ENDPOINT"); v != "" {
		cfg.MCP.Upstream = v
	}

	if v := os.Getenv("ZOEA_OLLAMA_ENDPOINT"); v != "" {
		if p, ok := cfg.Providers["ollama"]; ok {
			p.Endpoint = v
			cfg.Providers["ollama"] = p
		}
	}

	if v := os.Getenv("ZOEA_OLLAMA_MODEL"); v != "" {
		if p, ok := cfg.Providers["ollama"]; ok {
			p.Model = v
			cfg.Providers["ollama"] = p
		}
	}

	if v := os.Getenv("ZOEA_OLLAMA_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if p, ok := cfg.Providers["ollama"]; ok {
				p.Temperature = f
				cfg.Providers["ollama"] = p
			}
		}
	}

	if v := os.Getenv("ZOEA_OLLAMA_RATE_LIMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if p, ok := cfg.Providers["ollama"]; ok {
				p.RateLimit = f
				cfg.Providers["ollama"] = p
			}
		}
	}

	if v := os.Getenv("ZOEA_OLLAMA_RATE_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if p, ok := cfg.Providers["ollama"]; ok {
				p.RateBurst = n
				cfg.Providers["ollama"] = p
			}
		}
	}

	if v := os.Getenv("ZOEA_OPENCODE_ENDPOINT"); v != "" {
		if p, ok := cfg.Providers["opencode_zen"]; ok {
			p.Endpoint = v
			cfg.Providers["opencode_zen"] = p
		}
	}

	if v := os.Getenv("ZOEA_OPENCODE_MODEL"); v != "" {
		if p, ok := cfg.Providers["opencode_zen"]; ok {
			p.Model = v
			cfg.Providers["opencode_zen"] = p
		}
	}

	if v := os.Getenv("ZOEA_OPENCODE_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if p, ok := cfg.Providers["opencode_zen"]; ok {
				p.Temperature = f
				cfg.Providers["opencode_zen"] = p
			}
		}
	}

	if v := os.Getenv("ZOEA_OPENCODE_RATE_LIMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if p, ok := cfg.Providers["opencode_zen"]; ok {
				p.RateLimit = f
				cfg.Providers["opencode_zen"] = p
			}
		}
	}

	if v := os.Getenv("ZOEA_OPENCODE_RATE_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if p, ok := cfg.Providers["opencode_zen"]; ok {
				p.RateBurst = n
				cfg.Providers["opencode_zen"] = p
			}
		}
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
