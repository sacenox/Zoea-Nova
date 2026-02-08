package provider

import (
	"strings"
	"testing"

	"github.com/xonecas/zoea-nova/internal/config"
)

// TestZenNanoRegistration reproduces the production bug where zen-nano provider
// is defined in config but fails with "provider not found".
func TestZenNanoRegistration(t *testing.T) {
	// Simulate exact production config
	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses:        16,
			DefaultProvider: "zen-nano",
			DefaultModel:    "gpt-5-nano",
		},
		Providers: map[string]config.ProviderConfig{
			"zen-nano": {
				Endpoint:    "https://opencode.ai/zen/v1",
				Model:       "gpt-5-nano",
				Temperature: 0.7,
				RateLimit:   1.0,
				RateBurst:   2,
			},
		},
	}

	// Simulate initProviders logic from main.go
	registry := NewRegistry()
	creds := &config.Credentials{}
	creds.SetAPIKey("opencode_zen", "test-api-key")

	// This is EXACTLY what initProviders does
	for name, provCfg := range cfg.Providers {
		if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
			factory := NewOllamaFactory(name, provCfg.Endpoint, provCfg.RateLimit, provCfg.RateBurst)
			registry.RegisterFactory(name, factory)
			t.Logf("Registered Ollama provider: %s", name)
		} else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
			apiKey := creds.GetAPIKey("opencode_zen")
			if apiKey != "" {
				factory := NewOpenCodeFactory(name, provCfg.Endpoint, apiKey, provCfg.RateLimit, provCfg.RateBurst)
				registry.RegisterFactory(name, factory)
				t.Logf("Registered OpenCode provider: %s", name)
			}
		}
	}

	// List what's in registry
	registeredProviders := registry.List()
	t.Logf("Registry has %d providers", len(registeredProviders))
	for _, name := range registeredProviders {
		t.Logf("  Registry contains: %s", name)
	}

	// This is what Commander.CreateMysis does
	_, err := registry.Create("zen-nano", "gpt-5-nano", 0.7)
	if err != nil {
		t.Fatalf("REPRODUCTION: zen-nano provider creation failed: %v\nThis reproduces the production bug.", err)
	}

	t.Log("SUCCESS: zen-nano provider created")
}
