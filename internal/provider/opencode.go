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

	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)

// openCodeRequest is a custom request struct to ensure stream:false is serialized
// The openai.ChatCompletionRequest has omitempty on Stream, which omits false values
type openCodeRequest struct {
	Model       string                         `json:"model"`
	Messages    []openai.ChatCompletionMessage `json:"messages"`
	Tools       []openai.Tool                  `json:"tools,omitempty"`
	Temperature float32                        `json:"temperature,omitempty"`
	Stream      bool                           `json:"stream"` // NO omitempty - always serialize
}

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
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
		Temperature: float32(p.temperature),
		Stream:      false,
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
	openaiTools, err := toOpenAITools(tools)
	if err != nil {
		return nil, fmt.Errorf("invalid tool schema: %w", err)
	}

	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
		Tools:       openaiTools,
		Temperature: float32(p.temperature),
		Stream:      false,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		log.Error().
			Str("provider", "opencode_zen").
			Msg("OpenCode returned empty choices array")
		return nil, errors.New("no response choices")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Content:   choice.Message.Content,
		Reasoning: "", // OpenAI standard doesn't provide reasoning field
	}

	log.Debug().
		Str("provider", "opencode_zen").
		Str("content", choice.Message.Content).
		Int("tool_call_count", len(choice.Message.ToolCalls)).
		Msg("OpenCode ChatWithTools result")

	// Extract tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			}
			log.Debug().
				Str("provider", "opencode_zen").
				Str("tool_call_id", tc.ID).
				Str("tool_name", tc.Function.Name).
				Str("arguments", tc.Function.Arguments).
				Msg("OpenCode tool call extracted")
		}
	}

	return result, nil
}

func (p *OpenCodeProvider) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (*openaiChatResponse, error) {
	// Use custom struct to ensure stream:false is serialized
	customReq := openCodeRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}
	body, err := json.Marshal(customReq)
	if err != nil {
		return nil, err
	}

	url := p.baseURL + "/chat/completions"
	log.Debug().
		Str("provider", "opencode_zen").
		Str("url", url).
		Str("model", req.Model).
		Bool("has_api_key", p.apiKey != "").
		Int("message_count", len(req.Messages)).
		Int("tool_count", len(req.Tools)).
		Str("request_body", string(body)).
		Msg("OpenCode chat completion request")
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

	log.Debug().
		Str("provider", "opencode_zen").
		Str("url", url).
		Int("status", resp.StatusCode).
		Msg("OpenCode chat completion response")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		log.Error().
			Str("provider", "opencode_zen").
			Int("status", resp.StatusCode).
			Str("body", string(payload)).
			Msg("OpenCode non-2xx response")
		return nil, fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	// Read body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Msg("OpenCode failed to read response body")
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var decoded openaiChatResponse
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Str("body", string(bodyBytes)).
			Msg("OpenCode JSON decode failed")
		return nil, fmt.Errorf("decode response: %w", err)
	}

	log.Debug().
		Str("provider", "opencode_zen").
		Int("choice_count", len(decoded.Choices)).
		Msg("OpenCode response decoded")

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
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
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
