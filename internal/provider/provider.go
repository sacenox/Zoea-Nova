// Package provider defines the LLM provider interface and implementations.
package provider

import (
	"context"
	"errors"
)

// ErrProviderNotFound is returned when a requested provider doesn't exist.
var ErrProviderNotFound = errors.New("provider not found")

// Message represents a chat message.
type Message struct {
	Role    string
	Content string
}

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider's identifier.
	Name() string

	// Chat sends messages and returns the complete response.
	Chat(ctx context.Context, messages []Message) (string, error)

	// Stream sends messages and returns a channel that streams response chunks.
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}

// StreamChunk represents a chunk of streamed response.
type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

// Registry holds available providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get retrieves a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return p, nil
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
