package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultConfig removed - config file is now required

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
temperature = 0.7
rate_limit = 2.0
rate_burst = 3

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
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.5
rate_limit = 2.0
rate_burst = 3
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
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
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
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3

[mcp]
upstream = "https://default.mcp/endpoint"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set env vars
	os.Setenv("ZOEA_MAX_MYSES", "10")
	os.Setenv("ZOEA_MCP_ENDPOINT", "https://env.mcp/endpoint")
	defer func() {
		os.Unsetenv("ZOEA_MAX_MYSES")
		os.Unsetenv("ZOEA_MCP_ENDPOINT")
	}()

	cfg, err := Load(configPath)
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
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("Load() should error for non-existent file")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("expected 'config file not found' error, got %v", err)
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
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://file-endpoint"
model = "file-model"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3
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
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

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

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Invalid env overrides should be ignored, file values should remain
	if cfg.Swarm.MaxMyses != 16 {
		t.Errorf("expected max_myses=16, got %d", cfg.Swarm.MaxMyses)
	}
	if cfg.Providers["ollama"].Temperature != 0.7 {
		t.Errorf("expected temperature=0.7, got %v", cfg.Providers["ollama"].Temperature)
	}
	if cfg.Providers["ollama"].RateLimit != 2.0 {
		t.Errorf("expected rate_limit=2.0, got %v", cfg.Providers["ollama"].RateLimit)
	}
	if cfg.Providers["ollama"].RateBurst != 3 {
		t.Errorf("expected rate_burst=3, got %d", cfg.Providers["ollama"].RateBurst)
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
temperature = 0.7
rate_limit = 2.0
rate_burst = 3
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

func TestLoadAllEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3

[providers.opencode_zen]
endpoint = "https://zen.example.com"
model = "gpt-5-nano"
temperature = 0.8
rate_limit = 3.0
rate_burst = 4

[mcp]
upstream = "https://mcp.example.com"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Set all env vars
	os.Setenv("ZOEA_MAX_MYSES", "32")
	os.Setenv("ZOEA_MCP_ENDPOINT", "https://env-mcp.example.com")
	os.Setenv("ZOEA_OLLAMA_ENDPOINT", "http://env-ollama:11434")
	os.Setenv("ZOEA_OLLAMA_MODEL", "env-model")
	os.Setenv("ZOEA_OLLAMA_TEMPERATURE", "0.5")
	os.Setenv("ZOEA_OLLAMA_RATE_LIMIT", "5.0")
	os.Setenv("ZOEA_OLLAMA_RATE_BURST", "10")
	os.Setenv("ZOEA_OPENCODE_ENDPOINT", "https://env-zen.example.com")
	os.Setenv("ZOEA_OPENCODE_MODEL", "env-zen-model")
	os.Setenv("ZOEA_OPENCODE_TEMPERATURE", "0.6")
	os.Setenv("ZOEA_OPENCODE_RATE_LIMIT", "6.0")
	os.Setenv("ZOEA_OPENCODE_RATE_BURST", "12")
	defer func() {
		os.Unsetenv("ZOEA_MAX_MYSES")
		os.Unsetenv("ZOEA_MCP_ENDPOINT")
		os.Unsetenv("ZOEA_OLLAMA_ENDPOINT")
		os.Unsetenv("ZOEA_OLLAMA_MODEL")
		os.Unsetenv("ZOEA_OLLAMA_TEMPERATURE")
		os.Unsetenv("ZOEA_OLLAMA_RATE_LIMIT")
		os.Unsetenv("ZOEA_OLLAMA_RATE_BURST")
		os.Unsetenv("ZOEA_OPENCODE_ENDPOINT")
		os.Unsetenv("ZOEA_OPENCODE_MODEL")
		os.Unsetenv("ZOEA_OPENCODE_TEMPERATURE")
		os.Unsetenv("ZOEA_OPENCODE_RATE_LIMIT")
		os.Unsetenv("ZOEA_OPENCODE_RATE_BURST")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify all overrides
	if cfg.Swarm.MaxMyses != 32 {
		t.Errorf("expected max_myses=32, got %d", cfg.Swarm.MaxMyses)
	}
	if cfg.MCP.Upstream != "https://env-mcp.example.com" {
		t.Errorf("expected env mcp endpoint, got %s", cfg.MCP.Upstream)
	}
	if cfg.Providers["ollama"].Endpoint != "http://env-ollama:11434" {
		t.Errorf("expected env ollama endpoint, got %s", cfg.Providers["ollama"].Endpoint)
	}
	if cfg.Providers["ollama"].Model != "env-model" {
		t.Errorf("expected env ollama model, got %s", cfg.Providers["ollama"].Model)
	}
	if cfg.Providers["ollama"].Temperature != 0.5 {
		t.Errorf("expected env ollama temperature=0.5, got %v", cfg.Providers["ollama"].Temperature)
	}
	if cfg.Providers["ollama"].RateLimit != 5.0 {
		t.Errorf("expected env ollama rate_limit=5.0, got %v", cfg.Providers["ollama"].RateLimit)
	}
	if cfg.Providers["ollama"].RateBurst != 10 {
		t.Errorf("expected env ollama rate_burst=10, got %d", cfg.Providers["ollama"].RateBurst)
	}
	if cfg.Providers["opencode_zen"].Endpoint != "https://env-zen.example.com" {
		t.Errorf("expected env zen endpoint, got %s", cfg.Providers["opencode_zen"].Endpoint)
	}
	if cfg.Providers["opencode_zen"].Model != "env-zen-model" {
		t.Errorf("expected env zen model, got %s", cfg.Providers["opencode_zen"].Model)
	}
	if cfg.Providers["opencode_zen"].Temperature != 0.6 {
		t.Errorf("expected env zen temperature=0.6, got %v", cfg.Providers["opencode_zen"].Temperature)
	}
	if cfg.Providers["opencode_zen"].RateLimit != 6.0 {
		t.Errorf("expected env zen rate_limit=6.0, got %v", cfg.Providers["opencode_zen"].RateLimit)
	}
	if cfg.Providers["opencode_zen"].RateBurst != 12 {
		t.Errorf("expected env zen rate_burst=12, got %d", cfg.Providers["opencode_zen"].RateBurst)
	}
}

func TestLoadPartialEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Override only model and temperature
	os.Setenv("ZOEA_OLLAMA_MODEL", "partial-model")
	os.Setenv("ZOEA_OLLAMA_TEMPERATURE", "0.9")
	defer func() {
		os.Unsetenv("ZOEA_OLLAMA_MODEL")
		os.Unsetenv("ZOEA_OLLAMA_TEMPERATURE")
	}()

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify partial overrides
	if cfg.Providers["ollama"].Model != "partial-model" {
		t.Errorf("expected partial model override, got %s", cfg.Providers["ollama"].Model)
	}
	if cfg.Providers["ollama"].Temperature != 0.9 {
		t.Errorf("expected partial temperature override=0.9, got %v", cfg.Providers["ollama"].Temperature)
	}
	// Verify file values remain
	if cfg.Providers["ollama"].Endpoint != "http://localhost:11434" {
		t.Errorf("expected file endpoint, got %s", cfg.Providers["ollama"].Endpoint)
	}
	if cfg.Providers["ollama"].RateLimit != 2.0 {
		t.Errorf("expected file rate_limit=2.0, got %v", cfg.Providers["ollama"].RateLimit)
	}
}

func TestLoadInvalidTOMLSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "unclosed_bracket",
			content: `
[swarm
max_myses = 16
`,
		},
		{
			name: "invalid_assignment",
			content: `
[swarm]
max_myses = = 16
`,
		},
		{
			name: "unclosed_string",
			content: `
[providers.ollama]
endpoint = "http://localhost:11434
model = "qwen3:4b"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("expected parse error for invalid TOML syntax")
			}
			if !strings.Contains(err.Error(), "failed to parse config") {
				t.Errorf("expected parse error message, got %v", err)
			}
		})
	}
}
