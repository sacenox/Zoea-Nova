package provider

import (
	"context"
)

// MockProvider is a test provider that returns predefined responses.
type MockProvider struct {
	name      string
	response  string
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
	p.chatErr = err
	return p
}

// WithStreamError sets an error to return from Stream.
func (p *MockProvider) WithStreamError(err error) *MockProvider {
	p.streamErr = err
	return p
}

// Name returns the provider identifier.
func (p *MockProvider) Name() string {
	return p.name
}

// Chat returns the predefined response or error.
func (p *MockProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.chatErr != nil {
		return "", p.chatErr
	}
	return p.response, nil
}

// Stream returns the predefined response as a single chunk.
func (p *MockProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if p.streamErr != nil {
		return nil, p.streamErr
	}

	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Content: p.response}
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}
