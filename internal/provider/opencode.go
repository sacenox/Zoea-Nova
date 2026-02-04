package provider

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// OpenCodeProvider implements the Provider interface for OpenCode Zen.
type OpenCodeProvider struct {
	client *openai.Client
	model  string
}

// NewOpenCode creates a new OpenCode Zen provider.
func NewOpenCode(endpoint, model, apiKey string) *OpenCodeProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = endpoint

	return &OpenCodeProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

// Name returns the provider identifier.
func (p *OpenCodeProvider) Name() string {
	return "opencode_zen"
}

// Chat sends messages and returns the complete response.
func (p *OpenCodeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: toOpenAIMessages(messages),
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no response choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// Stream sends messages and returns a channel that streams response chunks.
func (p *OpenCodeProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: toOpenAIMessages(messages),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer stream.Close()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				ch <- StreamChunk{Done: true}
				return
			}
			if err != nil {
				ch <- StreamChunk{Err: err}
				return
			}

			if len(resp.Choices) > 0 {
				ch <- StreamChunk{Content: resp.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}
