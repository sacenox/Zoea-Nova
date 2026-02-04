package provider

import (
	"context"
	"sync"
)

// MockProvider is a test provider that returns predefined responses.
type MockProvider struct {
	mu sync.RWMutex

	name      string
	response  string
	toolCalls []ToolCall
	streamErr error
	chatErr   error
}

// NewMock creates a new mock provider.
func NewMock(name, response string) *MockProvider {
	return &MockProvider{
		name:     name,
		response: response,
	}
}

// WithChatError sets an error to return from Chat.
func (p *MockProvider) WithChatError(err error) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chatErr = err
	return p
}

// WithStreamError sets an error to return from Stream.
func (p *MockProvider) WithStreamError(err error) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.streamErr = err
	return p
}

// WithToolCalls sets tool calls to return from ChatWithTools.
func (p *MockProvider) WithToolCalls(calls []ToolCall) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.toolCalls = calls
	return p
}

// WithResponse sets the predefined response to return from Chat.
func (p *MockProvider) WithResponse(response string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.response = response
	return p
}

// Name returns the provider identifier.
func (p *MockProvider) Name() string {
	return p.name
}

// Chat returns the predefined response or error.
func (p *MockProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.chatErr != nil {
		return "", p.chatErr
	}
	return p.response, nil
}

// ChatWithTools returns the predefined response or tool calls.
func (p *MockProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return &ChatResponse{
		Content:   p.response,
		ToolCalls: p.toolCalls,
	}, nil
}

// Stream returns the predefined response as a single chunk.
func (p *MockProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.streamErr != nil {
		return nil, p.streamErr
	}

	ch := make(chan StreamChunk, 2)
	response := p.response
	go func() {
		defer close(ch)
		ch <- StreamChunk{Content: response}
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}
