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
	"time"

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
	name        string
	client      *openai.Client
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	model       string
	temperature float64
	limiter     *rate.Limiter
}

var opencodeRetryDelays = []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

const (
	opencodeChatCompletionsEndpoint = "/chat/completions"
	opencodeMessagesEndpoint        = "/messages"
	opencodeResponsesEndpoint       = "/responses"
)

var opencodeModelEndpoints = map[string]string{
	"big-pickle":                 opencodeChatCompletionsEndpoint,
	"gemini-3-pro":               "/models/gemini-3-pro",
	"gemini-3-flash":             "/models/gemini-3-flash",
	"glm-4.7-free":               opencodeChatCompletionsEndpoint,
	"gpt-5-nano":                 opencodeChatCompletionsEndpoint, // Using chat/completions despite docs saying /responses (500 errors)
	"kimi-k2.5-free":             opencodeChatCompletionsEndpoint,
	"minimax-m2.1-free":          opencodeMessagesEndpoint,
	"trinity-large-preview-free": opencodeChatCompletionsEndpoint,
}

// NewOpenCode creates a new OpenCode Zen provider.
func NewOpenCode(endpoint, model, apiKey string) *OpenCodeProvider {
	return NewOpenCodeWithTemp("opencode_zen", endpoint, model, apiKey, 0.7, nil)
}

func NewOpenCodeWithTemp(name string, endpoint, model, apiKey string, temperature float64, limiter *rate.Limiter) *OpenCodeProvider {
	config := openai.DefaultConfig(apiKey)
	baseURL := strings.TrimRight(endpoint, "/")
	config.BaseURL = baseURL

	return &OpenCodeProvider{
		name:        name,
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
	return p.name
}

// Chat sends messages and returns the complete response.
func (p *OpenCodeProvider) Chat(ctx context.Context, messages []Message) (string, error) {
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
			Str("provider", p.name).
			Msg("OpenCode returned empty choices array")
		return nil, errors.New("no response choices")
	}

	choice := resp.Choices[0]
	result := &ChatResponse{
		Content:   choice.Message.Content,
		Reasoning: "", // OpenAI standard doesn't provide reasoning field
	}

	log.Debug().
		Str("provider", p.name).
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
				Str("provider", p.name).
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

	url := p.baseURL + opencodeEndpointForModel(p.model)

	// Retry logic for transient errors (rate limits, server outages)
	maxRetries := len(opencodeRetryDelays)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := opencodeRetryDelays[attempt-1]
			log.Warn().
				Str("provider", p.name).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying OpenCode request after transient error")

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if p.limiter != nil {
			if err := p.limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}

		log.Debug().
			Str("provider", p.name).
			Str("url", url).
			Str("model", req.Model).
			Bool("has_api_key", p.apiKey != "").
			Int("message_count", len(req.Messages)).
			Int("tool_count", len(req.Tools)).
			Int("attempt", attempt+1).
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
			// Do not retry on context cancellation or timeout
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			lastErr = err
			continue // Network error - retry
		}
		defer resp.Body.Close()

		log.Debug().
			Str("provider", p.name).
			Str("url", url).
			Int("status", resp.StatusCode).
			Int("attempt", attempt+1).
			Msg("OpenCode chat completion response")

		// Check for retryable status codes
		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 502 ||
			resp.StatusCode == 503 || resp.StatusCode == 504 {
			payload, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))

			log.Warn().
				Str("provider", p.name).
				Int("status", resp.StatusCode).
				Int("attempt", attempt+1).
				Str("body", string(payload)).
				Msg("OpenCode retryable error")

			resp.Body.Close()
			continue // Retry on transient server errors and rate limits
		}

		// Non-retryable client error (4xx except 429)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(resp.Body)
			log.Error().
				Str("provider", p.name).
				Int("status", resp.StatusCode).
				Str("body", string(payload)).
				Msg("OpenCode non-2xx response")
			return nil, fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
		}

		// Success - read and decode body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error().
				Str("provider", p.name).
				Err(err).
				Msg("OpenCode failed to read response body")
			return nil, fmt.Errorf("read response body: %w", err)
		}

		var decoded openaiChatResponse
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			log.Error().
				Str("provider", p.name).
				Err(err).
				Str("body", string(bodyBytes)).
				Msg("OpenCode JSON decode failed")
			return nil, fmt.Errorf("decode response: %w", err)
		}

		log.Debug().
			Str("provider", p.name).
			Int("choice_count", len(decoded.Choices)).
			Msg("OpenCode response decoded")

		return &decoded, nil
	}

	// All retries exhausted
	log.Error().
		Str("provider", p.name).
		Int("max_retries", maxRetries).
		Err(lastErr).
		Msg("OpenCode request failed after all retries")
	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

// Stream sends messages and returns a channel that streams response chunks.
func (p *OpenCodeProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	if opencodeEndpointForModel(p.model) != opencodeChatCompletionsEndpoint {
		return nil, fmt.Errorf("opencode model %q does not support streaming via chat completions endpoint", p.model)
	}
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

func opencodeEndpointForModel(model string) string {
	if endpoint, ok := opencodeModelEndpoints[model]; ok {
		return endpoint
	}

	switch {
	case strings.HasPrefix(model, "gpt-"):
		return opencodeResponsesEndpoint
	case strings.HasPrefix(model, "claude-"):
		return opencodeMessagesEndpoint
	default:
		return opencodeChatCompletionsEndpoint
	}
}

// Close closes idle HTTP connections
func (p *OpenCodeProvider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
