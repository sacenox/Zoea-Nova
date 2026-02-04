package provider

import (
	"context"
	"encoding/json"
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

// ChatWithTools sends messages with available tools and returns response with potential tool calls.
func (p *OpenCodeProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	// Convert tools to OpenAI format
	openaiTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]interface{}
		if len(t.Parameters) > 0 {
			json.Unmarshal(t.Parameters, &params)
		}
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: toOpenAIMessages(messages),
		Tools:    openaiTools,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response choices")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Content: choice.Message.Content,
	}

	// Extract tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			}
		}
	}

	return result, nil
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
