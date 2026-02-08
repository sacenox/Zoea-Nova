package provider

import (
	"testing"
)

// TestProviderNameFromConfig verifies that provider names come from config keys,
// not hardcoded values in factory implementations.
func TestProviderNameFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		configName   string
		factoryFunc  func(name string) ProviderFactory
		expectedName string
	}{
		{
			name:       "Ollama with custom name",
			configName: "ollama-qwen",
			factoryFunc: func(name string) ProviderFactory {
				return NewOllamaFactory(name, "http://localhost:11434", 1.0, 2)
			},
			expectedName: "ollama-qwen",
		},
		{
			name:       "Ollama with another custom name",
			configName: "ollama-llama",
			factoryFunc: func(name string) ProviderFactory {
				return NewOllamaFactory(name, "http://localhost:11434", 1.0, 2)
			},
			expectedName: "ollama-llama",
		},
		{
			name:       "OpenCode with custom name",
			configName: "zen-nano",
			factoryFunc: func(name string) ProviderFactory {
				return NewOpenCodeFactory(name, "https://opencode.ai/zen/v1", "test-key", 5.0, 3)
			},
			expectedName: "zen-nano",
		},
		{
			name:       "OpenCode with another custom name",
			configName: "zen-pickle",
			factoryFunc: func(name string) ProviderFactory {
				return NewOpenCodeFactory(name, "https://opencode.ai/zen/v1", "test-key", 5.0, 3)
			},
			expectedName: "zen-pickle",
		},
		{
			name:       "OpenCode with default name",
			configName: "opencode_zen",
			factoryFunc: func(name string) ProviderFactory {
				return NewOpenCodeFactory(name, "https://opencode.ai/zen/v1", "test-key", 5.0, 3)
			},
			expectedName: "opencode_zen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := tt.factoryFunc(tt.configName)

			// Verify factory name matches config name
			if factory.Name() != tt.expectedName {
				t.Errorf("Factory.Name() = %q, want %q", factory.Name(), tt.expectedName)
			}

			// Create a provider instance and verify it also returns the correct name
			provider := factory.Create("test-model", 0.7)
			if provider.Name() != tt.expectedName {
				t.Errorf("Provider.Name() = %q, want %q", provider.Name(), tt.expectedName)
			}
		})
	}
}

// TestProviderNameNotHardcoded verifies that different config names produce
// different provider names, proving the name is not hardcoded.
func TestProviderNameNotHardcoded(t *testing.T) {
	// Create two Ollama factories with different names
	factory1 := NewOllamaFactory("ollama-qwen", "http://localhost:11434", 1.0, 2)
	factory2 := NewOllamaFactory("ollama-llama", "http://localhost:11434", 1.0, 2)

	if factory1.Name() == factory2.Name() {
		t.Errorf("Different config names should produce different factory names, both returned %q", factory1.Name())
	}

	// Create providers from the factories
	provider1 := factory1.Create("qwen2.5:7b", 0.7)
	provider2 := factory2.Create("llama3.2:3b", 0.7)

	if provider1.Name() == provider2.Name() {
		t.Errorf("Different config names should produce different provider names, both returned %q", provider1.Name())
	}

	// Verify each provider returns its expected name
	if provider1.Name() != "ollama-qwen" {
		t.Errorf("provider1.Name() = %q, want %q", provider1.Name(), "ollama-qwen")
	}
	if provider2.Name() != "ollama-llama" {
		t.Errorf("provider2.Name() = %q, want %q", provider2.Name(), "ollama-llama")
	}

	// Same test for OpenCode providers
	factory3 := NewOpenCodeFactory("zen-nano", "https://opencode.ai/zen/v1", "key", 5.0, 3)
	factory4 := NewOpenCodeFactory("zen-pickle", "https://opencode.ai/zen/v1", "key", 5.0, 3)

	if factory3.Name() == factory4.Name() {
		t.Errorf("Different config names should produce different factory names, both returned %q", factory3.Name())
	}

	provider3 := factory3.Create("gpt-5-nano", 0.7)
	provider4 := factory4.Create("big-pickle", 0.7)

	if provider3.Name() == provider4.Name() {
		t.Errorf("Different config names should produce different provider names, both returned %q", provider3.Name())
	}

	if provider3.Name() != "zen-nano" {
		t.Errorf("provider3.Name() = %q, want %q", provider3.Name(), "zen-nano")
	}
	if provider4.Name() != "zen-pickle" {
		t.Errorf("provider4.Name() = %q, want %q", provider4.Name(), "zen-pickle")
	}
}
