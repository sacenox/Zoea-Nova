package config

import (
	"os"
	"path/filepath"
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
