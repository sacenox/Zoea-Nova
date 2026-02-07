package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Swarm.MaxMyses != 16 {
		t.Errorf("expected max_myses=16, got %d", cfg.Swarm.MaxMyses)
	}
	if _, ok := cfg.Providers["ollama"]; !ok {
		t.Error("expected ollama provider in defaults")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[swarm]
max_myses = 32

[providers.ollama]
endpoint = "http://custom:11434"
model = "mistral"

[mcp]
upstream = "https://custom.mcp/endpoint"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Swarm.MaxMyses != 32 {
		t.Errorf("expected max_myses=32, got %d", cfg.Swarm.MaxMyses)
	}
	if cfg.Providers["ollama"].Endpoint != "http://custom:11434" {
		t.Errorf("expected custom ollama endpoint, got %s", cfg.Providers["ollama"].Endpoint)
	}
	if cfg.MCP.Upstream != "https://custom.mcp/endpoint" {
		t.Errorf("expected custom mcp upstream, got %s", cfg.MCP.Upstream)
	}
}

func TestLoadWithTemperature(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.5
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Providers["ollama"].Temperature != 0.5 {
		t.Errorf("expected temperature=0.5, got %v", cfg.Providers["ollama"].Temperature)
	}
}

func TestLoadWithRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
rate_limit = 3.5
rate_burst = 4
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Providers["ollama"].RateLimit != 3.5 {
		t.Errorf("expected rate_limit=3.5, got %v", cfg.Providers["ollama"].RateLimit)
	}
	if cfg.Providers["ollama"].RateBurst != 4 {
		t.Errorf("expected rate_burst=4, got %d", cfg.Providers["ollama"].RateBurst)
	}
}

func TestLoadInvalidTemperature(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 3.5
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected validation error for invalid temperature")
	}
	if !strings.Contains(err.Error(), "temperature") {
		t.Fatalf("expected temperature validation error, got %v", err)
	}
}

func TestLoadMissingProviderFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[providers.ollama]
endpoint = ""
model = ""
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected validation error for missing provider fields")
	}
	if !strings.Contains(err.Error(), "providers.ollama.endpoint") {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "providers.ollama.model") {
		t.Fatalf("expected model validation error, got %v", err)
	}
}

func TestLoadWithEnvOverrides(t *testing.T) {
	// Set env vars
	os.Setenv("ZOEA_MAX_MYSES", "10")
	os.Setenv("ZOEA_MCP_ENDPOINT", "https://env.mcp/endpoint")
	defer func() {
		os.Unsetenv("ZOEA_MAX_MYSES")
		os.Unsetenv("ZOEA_MCP_ENDPOINT")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Swarm.MaxMyses != 10 {
		t.Errorf("expected env override max_myses=10, got %d", cfg.Swarm.MaxMyses)
	}
	if cfg.MCP.Upstream != "https://env.mcp/endpoint" {
		t.Errorf("expected env override mcp endpoint, got %s", cfg.MCP.Upstream)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("Load() should not error for non-existent file: %v", err)
	}

	// Should return defaults
	if cfg.Swarm.MaxMyses != 16 {
		t.Errorf("expected max_myses=16, got %d", cfg.Swarm.MaxMyses)
	}
}

func TestCredentials(t *testing.T) {
	creds := &Credentials{
		Providers: make(map[string]ProviderCredentials),
	}

	// Test GetAPIKey on empty
	if key := creds.GetAPIKey("opencode_zen"); key != "" {
		t.Errorf("expected empty key, got %s", key)
	}

	// Test SetAPIKey
	creds.SetAPIKey("opencode_zen", "test-api-key")
	if key := creds.GetAPIKey("opencode_zen"); key != "test-api-key" {
		t.Errorf("expected test-api-key, got %s", key)
	}
}

func TestCredentialsSaveLoad(t *testing.T) {
	// Override home dir for test
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create and save credentials
	creds := &Credentials{}
	creds.SetAPIKey("opencode_zen", "secret-key-123")

	if err := SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials() error: %v", err)
	}

	// Verify file permissions
	path := filepath.Join(tmpDir, ".zoea-nova", "credentials.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("credentials file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}

	// Load credentials back
	loaded, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error: %v", err)
	}
	if key := loaded.GetAPIKey("opencode_zen"); key != "secret-key-123" {
		t.Errorf("expected secret-key-123, got %s", key)
	}
}

func TestLoadCredentialsNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	creds, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() should not error for non-existent file: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
}

func TestLoadEnvOverridePrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[providers.ollama]
endpoint = "http://file-endpoint"
model = "file-model"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	os.Setenv("ZOEA_OLLAMA_MODEL", "env-model")
	os.Setenv("ZOEA_OLLAMA_ENDPOINT", "http://env-endpoint")
	defer func() {
		os.Unsetenv("ZOEA_OLLAMA_MODEL")
		os.Unsetenv("ZOEA_OLLAMA_ENDPOINT")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Providers["ollama"].Model != "env-model" {
		t.Errorf("expected env model override, got %s", cfg.Providers["ollama"].Model)
	}
	if cfg.Providers["ollama"].Endpoint != "http://env-endpoint" {
		t.Errorf("expected env endpoint override, got %s", cfg.Providers["ollama"].Endpoint)
	}
}

func TestLoadIgnoresInvalidEnvOverrides(t *testing.T) {
	defaults := DefaultConfig()

	os.Setenv("ZOEA_MAX_MYSES", "not-a-number")
	os.Setenv("ZOEA_OLLAMA_TEMPERATURE", "bad")
	os.Setenv("ZOEA_OLLAMA_RATE_LIMIT", "bad")
	os.Setenv("ZOEA_OLLAMA_RATE_BURST", "bad")
	defer func() {
		os.Unsetenv("ZOEA_MAX_MYSES")
		os.Unsetenv("ZOEA_OLLAMA_TEMPERATURE")
		os.Unsetenv("ZOEA_OLLAMA_RATE_LIMIT")
		os.Unsetenv("ZOEA_OLLAMA_RATE_BURST")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Swarm.MaxMyses != defaults.Swarm.MaxMyses {
		t.Errorf("expected max_myses=%d, got %d", defaults.Swarm.MaxMyses, cfg.Swarm.MaxMyses)
	}
	if cfg.Providers["ollama"].Temperature != defaults.Providers["ollama"].Temperature {
		t.Errorf("expected temperature=%v, got %v", defaults.Providers["ollama"].Temperature, cfg.Providers["ollama"].Temperature)
	}
	if cfg.Providers["ollama"].RateLimit != defaults.Providers["ollama"].RateLimit {
		t.Errorf("expected rate_limit=%v, got %v", defaults.Providers["ollama"].RateLimit, cfg.Providers["ollama"].RateLimit)
	}
	if cfg.Providers["ollama"].RateBurst != defaults.Providers["ollama"].RateBurst {
		t.Errorf("expected rate_burst=%d, got %d", defaults.Providers["ollama"].RateBurst, cfg.Providers["ollama"].RateBurst)
	}
}

func TestLoadInvalidURLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "missing_scheme",
			content: `
[providers.ollama]
endpoint = "localhost:11434"
model = "qwen3:4b"
`,
			expected: "missing scheme or host",
		},
		{
			name: "missing_host",
			content: `
[providers.ollama]
endpoint = "http://"
model = "qwen3:4b"
`,
			expected: "missing scheme or host",
		},
		{
			name: "invalid_url",
			content: `
[providers.ollama]
endpoint = "ht!tp://localhost:11434"
model = "qwen3:4b"
`,
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected validation error for invalid URL format")
			}
			if !strings.Contains(err.Error(), tt.expected) {
				t.Fatalf("expected error containing %q, got %v", tt.expected, err)
			}
		})
	}
}

func TestLoadMaxMysesBounds(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	tests := []struct {
		name     string
		maxMyses int
	}{
		{"below_minimum", 0},
		{"negative", -1},
		{"above_maximum", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := fmt.Sprintf(`
[swarm]
max_myses = %d

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
`, tt.maxMyses)

			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected validation error for max_myses out of bounds")
			}
			if !strings.Contains(err.Error(), "max_myses") {
				t.Fatalf("expected max_myses validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), "between 1 and 100") {
				t.Fatalf("expected bounds message, got %v", err)
			}
		})
	}
}

func TestLoadRateLimitBounds(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	tests := []struct {
		name      string
		rateLimit float64
	}{
		{"zero", 0.0},
		{"negative", -1.0},
		{"negative_fraction", -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := fmt.Sprintf(`
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
rate_limit = %v
`, tt.rateLimit)

			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected validation error for rate_limit <= 0")
			}
			if !strings.Contains(err.Error(), "rate_limit") {
				t.Fatalf("expected rate_limit validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), "must be greater than 0") {
				t.Fatalf("expected bounds message, got %v", err)
			}
		})
	}
}

func TestLoadRateBurstBounds(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	tests := []struct {
		name      string
		rateBurst int
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := fmt.Sprintf(`
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
rate_burst = %d
`, tt.rateBurst)

			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected validation error for rate_burst < 1")
			}
			if !strings.Contains(err.Error(), "rate_burst") {
				t.Fatalf("expected rate_burst validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), "must be at least 1") {
				t.Fatalf("expected bounds message, got %v", err)
			}
		})
	}
}

func TestLoadDefaultProviderModel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[swarm]
max_myses = 16
default_provider = "ollama"
default_model = "qwen3:8b"

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:8b"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Swarm.DefaultProvider != "ollama" {
		t.Errorf("expected default_provider=ollama, got %s", cfg.Swarm.DefaultProvider)
	}

	if cfg.Swarm.DefaultModel != "qwen3:8b" {
		t.Errorf("expected default_model=qwen3:8b, got %s", cfg.Swarm.DefaultModel)
	}
}
