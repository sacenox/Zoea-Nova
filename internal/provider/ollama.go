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

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	name        string
	client      *openai.Client
	baseURL     string
	httpClient  *http.Client
	model       string
	temperature float64
	limiter     *rate.Limiter
}

var ollamaRetryDelays = []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}

// NewOllama creates a new Ollama provider.
// Ollama exposes an OpenAI-compatible API at /v1.
func NewOllama(endpoint, model string) *OllamaProvider {
	return NewOllamaWithTemp("ollama", endpoint, model, 0.7, nil)
}

func NewOllamaWithTemp(name string, endpoint, model string, temperature float64, limiter *rate.Limiter) *OllamaProvider {
	config := openai.DefaultConfig("")
	baseURL := strings.TrimRight(endpoint, "/") + "/v1"
	config.BaseURL = baseURL

	return &OllamaProvider{
		name:        name,
		client:      openai.NewClientWithConfig(config),
		baseURL:     baseURL,
		httpClient:  &http.Client{},
		model:       model,
		temperature: temperature,
		limiter:     limiter,
	}
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return p.name
}

// Chat sends messages and returns the complete response.
func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.createChatCompletion(ctx, ollamaChatRequest{
		Model:       p.model,
		Messages:    mergeConsecutiveSystemMessagesOllama(toOllamaMessages(messages)),
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
func (p *OllamaProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	resp, err := p.createChatCompletion(ctx, ollamaChatRequest{
		Model:       p.model,
		Messages:    mergeConsecutiveSystemMessagesOllama(toOllamaMessages(messages)),
		Tools:       toOllamaTools(tools),
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

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message chatCompletionMessage `json:"message"`
}

type chatCompletionMessage struct {
	Role             string                   `json:"role"`
	Content          string                   `json:"content"`
	Reasoning        string                   `json:"reasoning,omitempty"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
	ToolCalls        []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function chatCompletionFunction `json:"function"`
}

type chatCompletionFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OLLAMA-SPECIFIC TYPES
// These types are custom to Ollama and should NOT be used by other providers.
// They differ from OpenAI standard in structure and field names.

type ollamaChatRequest struct {
	Model       string             `json:"model"`
	Messages    []ollamaReqMessage `json:"messages"`
	Tools       []ollamaReqTool    `json:"tools,omitempty"`
	Temperature float32            `json:"temperature,omitempty"`
}

type ollamaReqMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolCalls  []ollamaReqToolCall `json:"tool_calls,omitempty"`
}

type ollamaReqTool struct {
	Type     string            `json:"type"`
	Function ollamaReqFunction `json:"function"`
}

type ollamaReqFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaReqToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function ollamaReqFuncCall `json:"function"`
}

type ollamaReqFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// reasoning extracts reasoning content from Ollama response.
//
// OLLAMA-SPECIFIC: Ollama may return reasoning in either "reasoning" or "reasoning_content" fields.
// This is an Ollama extension not present in standard OpenAI Chat Completions API.
func (m chatCompletionMessage) reasoning() string {
	if m.Reasoning != "" {
		return m.Reasoning
	}
	return m.ReasoningContent
}

func (p *OllamaProvider) createChatCompletion(ctx context.Context, req ollamaChatRequest) (*chatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := p.baseURL + "/chat/completions"

	maxRetries := len(ollamaRetryDelays)
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := ollamaRetryDelays[attempt-1]
			log.Warn().
				Str("provider", "ollama").
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying Ollama request after transient error")

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

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 502 ||
			resp.StatusCode == 503 || resp.StatusCode == 504 {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))

			log.Warn().
				Str("provider", "ollama").
				Int("status", resp.StatusCode).
				Int("attempt", attempt+1).
				Str("body", string(payload)).
				Msg("Ollama retryable error")
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("chat completion status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		var decoded chatCompletionResponse
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		return &decoded, nil
	}

	log.Error().
		Str("provider", "ollama").
		Int("max_retries", maxRetries).
		Err(lastErr).
		Msg("Ollama request failed after all retries")
	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

// Stream sends messages and returns a channel that streams response chunks.
func (p *OllamaProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
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

// toOllamaMessages converts provider messages to Ollama's custom request format.
//
// OLLAMA-SPECIFIC: Uses custom ollamaReqMessage type instead of OpenAI SDK types.
// Ollama has its own message format and tool call structure.
func toOllamaMessages(messages []Message) []ollamaReqMessage {
	result := make([]ollamaReqMessage, len(messages))
	for i, m := range messages {
		msg := ollamaReqMessage{
			Role:    m.Role,
			Content: m.Content,
		}

		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]ollamaReqToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msg.ToolCalls[j] = ollamaReqToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: ollamaReqFuncCall{
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

func toOllamaTools(tools []Tool) []ollamaReqTool {
	result := make([]ollamaReqTool, len(tools))
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

		result[i] = ollamaReqTool{
			Type: "function",
			Function: ollamaReqFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
	return result
}

// mergeConsecutiveSystemMessagesOllama merges consecutive system messages into a single message.
//
// OLLAMA-SPECIFIC BEHAVIOR:
// Unlike OpenAI, Ollama allows system messages anywhere in the conversation.
// This function merges consecutive system messages IN PLACE without moving them to the start.
// This preserves Ollama's flexible system message handling.
//
// DO NOT use this function for OpenAI-compatible providers - use mergeSystemMessagesOpenAI instead.
func mergeConsecutiveSystemMessagesOllama(messages []ollamaReqMessage) []ollamaReqMessage {
	if len(messages) == 0 {
		return messages
	}

	result := make([]ollamaReqMessage, 0, len(messages))
	var systemBuffer strings.Builder
	inSystemRun := false

	for i, msg := range messages {
		if msg.Role == "system" {
			// Start or continue system message accumulation
			if inSystemRun {
				systemBuffer.WriteString("\n\n")
			} else {
				inSystemRun = true
			}
			systemBuffer.WriteString(msg.Content)
		} else {
			// Flush accumulated system messages if any
			if inSystemRun {
				result = append(result, ollamaReqMessage{
					Role:    "system",
					Content: systemBuffer.String(),
				})
				systemBuffer.Reset()
				inSystemRun = false
			}
			// Add non-system message
			result = append(result, msg)
		}

		// Flush at end of array
		if i == len(messages)-1 && inSystemRun {
			result = append(result, ollamaReqMessage{
				Role:    "system",
				Content: systemBuffer.String(),
			})
		}
	}

	log.Debug().
		Int("original_count", len(messages)).
		Int("merged_count", len(result)).
		Msg("Ollama: Merged consecutive system messages")

	return result
}

// Close closes idle HTTP connections
func (p *OllamaProvider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
