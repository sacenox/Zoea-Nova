package provider

import "testing"

func TestOllamaFactorySharesLimiter(t *testing.T) {
	factory := NewOllamaFactory("ollama-test", "http://example.com", 1.0, 2)

	p1 := factory.Create("model-a", 0.7)
	p2 := factory.Create("model-b", 0.5)

	ollama1, ok := p1.(*OllamaProvider)
	if !ok {
		t.Fatalf("expected OllamaProvider, got %T", p1)
	}
	ollama2, ok := p2.(*OllamaProvider)
	if !ok {
		t.Fatalf("expected OllamaProvider, got %T", p2)
	}

	if ollama1.limiter == nil || ollama2.limiter == nil {
		t.Fatal("expected non-nil rate limiters")
	}
	if ollama1.limiter != ollama2.limiter {
		t.Fatal("expected shared limiter across Ollama providers")
	}
}

func TestOpenCodeFactorySharesLimiter(t *testing.T) {
	factory := NewOpenCodeFactory("zen-test", "https://api.opencode.ai/v1", "test-key", 5.0, 3)

	p1 := factory.Create("model-a", 0.7)
	p2 := factory.Create("model-b", 0.5)

	opencode1, ok := p1.(*OpenCodeProvider)
	if !ok {
		t.Fatalf("expected OpenCodeProvider, got %T", p1)
	}
	opencode2, ok := p2.(*OpenCodeProvider)
	if !ok {
		t.Fatalf("expected OpenCodeProvider, got %T", p2)
	}

	if opencode1.limiter == nil || opencode2.limiter == nil {
		t.Fatal("expected non-nil rate limiters")
	}
	if opencode1.limiter != opencode2.limiter {
		t.Fatal("expected shared limiter across OpenCode providers")
	}
}
