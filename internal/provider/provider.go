// Package provider defines the LLM provider interface and implementations.
package provider

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrProviderNotFound is returned when a requested provider doesn't exist.
var ErrProviderNotFound = errors.New("provider not found")

// Message represents a chat message.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall // For assistant messages with tool calls
	ToolCallID string     // For tool result messages
}

// Tool represents a tool/function definition for the LLM.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolCall represents a tool call made by the LLM.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ChatResponse represents the response from a chat completion.
type ChatResponse struct {
	Content   string     // Text content (may be empty if tool calls)
	ToolCalls []ToolCall // Tool calls (may be empty if text response)
	Reasoning string     // Model reasoning content (optional)
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider's identifier.
	Name() string

	// Chat sends messages and returns the complete response.
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatWithTools sends messages with available tools and returns response with potential tool calls.
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)

	// Stream sends messages and returns a channel that streams response chunks.
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}

type ProviderFactory interface {
	Name() string
	Create(model string, temperature float64) Provider
}

// StreamChunk represents a chunk of streamed response.
type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

// Registry holds available providers.
type Registry struct {
	factories map[string]ProviderFactory
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
	}
}

func (r *Registry) RegisterFactory(f ProviderFactory) {
	r.factories[f.Name()] = f
}

func (r *Registry) Create(name, model string, temperature float64) (Provider, error) {
	f, ok := r.factories[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return f.Create(model, temperature), nil
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}
