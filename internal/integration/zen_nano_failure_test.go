package integration

import (
	"testing"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestZenNanoFailure reproduces the exact production error:
// "Error: create provider: provider not found"
//
// This test uses the ACTUAL config.toml and credentials from production.
func TestZenNanoWithProductionConfig(t *testing.T) {
	// Step 1: Load ACTUAL production config
	cfg, err := config.Load("../../config.toml")
	if err != nil {
		t.Fatalf("Failed to load config.toml: %v", err)
	}

	// Step 2: Load ACTUAL production credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		t.Logf("WARNING: Failed to load credentials: %v", err)
		t.Skip("Skipping test - no credentials file (expected in CI)")
	}

	// Step 3: Check if zen-nano exists in config
	_, hasZenNano := cfg.Providers["zen-nano"]
	t.Logf("zen-nano in config: %v", hasZenNano)
	if !hasZenNano {
		t.Fatal("zen-nano not in config.toml - test cannot reproduce bug")
	}

	// Step 4: Check API key
	apiKey := creds.GetAPIKey("opencode_zen")
	t.Logf("API key exists: %v (length: %d)", apiKey != "", len(apiKey))

	// Step 5: Run EXACT initProviders code from main.go
	registry := provider.NewRegistry()

	registeredCount := 0
	skippedCount := 0

	for name, provCfg := range cfg.Providers {
		if provCfg.Endpoint == "http://localhost:11434" {
			factory := provider.NewOllamaFactory(provCfg.Endpoint, provCfg.RateLimit, provCfg.RateBurst)
			registry.RegisterFactory(name, factory)
			registeredCount++
			t.Logf("Registered Ollama: %s", name)
		} else if provCfg.Endpoint == "https://opencode.ai/zen/v1" {
			apiKey := creds.GetAPIKey("opencode_zen")
			if apiKey != "" {
				factory := provider.NewOpenCodeFactory(provCfg.Endpoint, apiKey, provCfg.RateLimit, provCfg.RateBurst)
				registry.RegisterFactory(name, factory)
				registeredCount++
				t.Logf("Registered OpenCode: %s", name)
			} else {
				skippedCount++
				t.Logf("SKIPPED (no API key): %s", name)
			}
		} else {
			skippedCount++
			t.Logf("SKIPPED (unknown endpoint %s): %s", provCfg.Endpoint, name)
		}
	}

	t.Logf("Registration summary: %d registered, %d skipped", registeredCount, skippedCount)

	// Step 6: Check if zen-nano is in registry
	registeredList := registry.List()
	zenNanoRegistered := false
	for _, name := range registeredList {
		if name == "zen-nano" {
			zenNanoRegistered = true
			break
		}
	}

	if !zenNanoRegistered {
		t.Fatalf("BUG REPRODUCED: zen-nano in config but NOT in registry.\nRegistered: %v", registeredList)
	}

	// Step 7: Initialize store and create mysis (EXACT user flow)
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	defer s.Close()

	bus := core.NewEventBus(100)
	commander := core.NewCommander(s, registry, bus, cfg)

	// Step 8: Create mysis (what happens when user completes TUI flow)
	mysis, err := commander.CreateMysis("test-zen", "zen-nano")
	if err != nil {
		t.Fatalf("BUG REPRODUCED: %v", err)
	}

	if mysis == nil {
		t.Fatal("Mysis is nil")
	}

	t.Log("SUCCESS: zen-nano mysis created")
}
