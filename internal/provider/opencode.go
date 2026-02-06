package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

// OpenCodeProvider implements the Provider interface for OpenCode Zen.
type OpenCodeProvider struct {
	client      *openai.Client
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	model       string
	temperature float64
	limiter     *rate.Limiter
}

// NewOpenCode creates a new OpenCode Zen provider.
func NewOpenCode(endpoint, model, apiKey string) *OpenCodeProvider {
	return NewOpenCodeWithTemp(endpoint, model, apiKey, 0.7, nil)
}

func NewOpenCodeWithTemp(endpoint, model, apiKey string, temperature float64, limiter *rate.Limiter) *OpenCodeProvider {
	config := openai.DefaultConfig(apiKey)
	baseURL := strings.TrimRight(endpoint, "/")
	config.BaseURL = baseURL

	return &OpenCodeProvider{
		client:      openai.NewClientWithConfig(config),
		baseURL:     baseURL,
		apiKey:      apiKey,
		httpClient:  &http.Client{},
		model:       model,
		temperature: temperature,
		limiter:     limiter,
	}
}

// Name returns the provider identifier.
func (p *OpenCodeProvider) Name() string {
	return "opencode_zen"
}

// Chat sends messages and returns the complete response.
func (p *OpenCodeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return "", err
		}
	}

	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: float32(p.temperature),
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
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	// Convert tools to OpenAI format
	openaiTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]interface{}
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &params); err != nil {
				params = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}
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

	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    toOpenAIMessages(messages),
		Tools:       openaiTools,
		Temperature: float32(p.temperature),
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response choices")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Content:   choice.Message.Content,
		Reasoning: choice.Message.reasoning(),
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

func (p *OpenCodeProvider) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (*chatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}

	return &decoded, nil
}

// Stream sends messages and returns a channel that streams response chunks.
func (p *OpenCodeProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    toOpenAIMessages(messages),
		Temperature: float32(p.temperature),
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

// Close closes idle HTTP connections
func (p *OpenCodeProvider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
