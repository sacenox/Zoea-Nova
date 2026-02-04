package provider

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	client *openai.Client
	model  string
}

// NewOllama creates a new Ollama provider.
// Ollama exposes an OpenAI-compatible API at /v1.
func NewOllama(endpoint, model string) *OllamaProvider {
	config := openai.DefaultConfig("")
	config.BaseURL = endpoint + "/v1"

	return &OllamaProvider{
		client: openai.NewClientWithConfig(config),
		model:  model,
	}
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Chat sends messages and returns the complete response.
func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
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
func (p *OllamaProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
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

func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}
