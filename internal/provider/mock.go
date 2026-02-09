package provider

import (
	"context"
	"sync"
	"time"
)

// MockProvider is a test provider that returns predefined responses.
type MockProvider struct {
	mu sync.RWMutex

	name      string
	response  string
	toolCalls []ToolCall
	streamErr error
	chatErr   error
	reasoning string
	delay     time.Duration
}

// NewMock creates a new mock provider.
func NewMock(name, response string) *MockProvider {
	return &MockProvider{
		name:     name,
		response: response,
	}
}

type MockFactory struct {
	name     string
	response string
}

func NewMockFactory(name, response string) *MockFactory {
	return &MockFactory{name: name, response: response}
}

func (f *MockFactory) Name() string { return f.name }

func (f *MockFactory) Create(model string, temperature float64) Provider {
	return NewMock(f.name, f.response)
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

func (p *MockProvider) WithReasoning(reasoning string) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reasoning = reasoning
	return p
}

func (p *MockProvider) SetDelay(delay time.Duration) *MockProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.delay = delay
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
	if err := p.waitDelay(ctx); err != nil {
		return "", err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.chatErr != nil {
		return "", p.chatErr
	}
	return p.response, nil
}

// ChatWithTools returns the predefined response or tool calls.
func (p *MockProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	if err := p.waitDelay(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.chatErr != nil {
		return nil, p.chatErr
	}
	return &ChatResponse{
		Content:   p.response,
		ToolCalls: p.toolCalls,
		Reasoning: p.reasoning,
	}, nil
}

// Stream returns the predefined response as a single chunk.
func (p *MockProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if err := p.waitDelay(ctx); err != nil {
		return nil, err
	}

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

func (p *MockProvider) waitDelay(ctx context.Context) error {
	p.mu.RLock()
	delay := p.delay
	p.mu.RUnlock()
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Close is a no-op for mock provider (no resources to clean up).
func (p *MockProvider) Close() error {
	return nil
}
