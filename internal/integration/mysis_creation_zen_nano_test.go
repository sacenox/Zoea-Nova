package integration

import (
	"testing"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestCreateMysisWithZenNano reproduces exact production flow:
// User presses 'n', enters name "test", presses Enter, enters provider "zen-nano", presses Enter
func TestCreateMysisWithZenNano(t *testing.T) {
	// Step 1: Load actual config from file
	cfg, err := config.Load("../../config.toml")
	if err != nil {
		t.Fatalf("Load config failed: %v", err)
	}

	t.Logf("Config loaded with %d providers", len(cfg.Providers))
	for name := range cfg.Providers {
		t.Logf("  Config provider: %s", name)
	}

	// Verify zen-nano exists in config
	zenNanoCfg, ok := cfg.Providers["zen-nano"]
	if !ok {
		t.Fatal("zen-nano not found in config.toml - test setup error")
	}
	t.Logf("zen-nano config: endpoint=%s, model=%s", zenNanoCfg.Endpoint, zenNanoCfg.Model)

	// Step 2: Initialize store
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory failed: %v", err)
	}
	defer s.Close()

	// Step 3: Initialize provider registry (EXACT production code from main.go)
	registry := provider.NewRegistry()

	// Mock credentials with API key
	creds := &config.Credentials{}
	creds.SetAPIKey("opencode_zen", "test-api-key-12345")

	// Run EXACT initProviders logic
	for name, provCfg := range cfg.Providers {
		t.Logf("Processing provider: %s (endpoint: %s)", name, provCfg.Endpoint)

		if provCfg.Endpoint == "http://localhost:11434" {
			factory := provider.NewOllamaFactory(name, provCfg.Endpoint, provCfg.RateLimit, provCfg.RateBurst)
			registry.RegisterFactory(name, factory)
			t.Logf("  ✓ Registered as Ollama: %s", name)
		} else if provCfg.Endpoint == "https://opencode.ai/zen/v1" {
			apiKey := creds.GetAPIKey("opencode_zen")
			t.Logf("  API key for opencode_zen: %s", apiKey)
			if apiKey != "" {
				factory := provider.NewOpenCodeFactory(name, provCfg.Endpoint, apiKey, provCfg.RateLimit, provCfg.RateBurst)
				registry.RegisterFactory(name, factory)
				t.Logf("  ✓ Registered as OpenCode: %s", name)
			} else {
				t.Logf("  ✗ Skipped (no API key): %s", name)
			}
		} else {
			t.Logf("  ✗ Skipped (unknown endpoint): %s", name)
		}
	}

	// Step 4: Check what's in registry
	registeredProviders := registry.List()
	t.Logf("Registry contains %d providers:", len(registeredProviders))
	for _, name := range registeredProviders {
		t.Logf("  - %s", name)
	}

	// Step 5: Create commander (EXACT production code)
	bus := core.NewEventBus(100)
	commander := core.NewCommander(s, registry, bus, cfg)

	// Step 6: Attempt to create mysis with zen-nano (EXACT user action)
	mysis, err := commander.CreateMysis("test-nano", "zen-nano")
	if err != nil {
		t.Fatalf("FAILED: CreateMysis with zen-nano failed: %v\n\nThis reproduces the production bug.", err)
	}

	if mysis == nil {
		t.Fatal("Mysis is nil despite no error")
	}

	t.Logf("SUCCESS: Created mysis with zen-nano provider")
}
