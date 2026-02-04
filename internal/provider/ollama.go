package provider

import (
	"context"
	"encoding/json"
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

// ChatWithTools sends messages with available tools and returns response with potential tool calls.
func (p *OllamaProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
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
		msg := openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}

		// Handle tool call results
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		// Handle assistant messages with tool calls
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]openai.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msg.ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				}
			}
		}

		result[i] = msg
	}
	return result
}
