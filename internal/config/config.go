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
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
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
				Endpoint: "http://localhost:11434",
				Model:    "llama3",
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
