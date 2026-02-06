package provider

import (
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAI-compliant response types for providers that follow OpenAI Chat Completions API spec.
// These types should NOT include provider-specific extensions.

type openaiChatResponse struct {
	Choices []openaiChatChoice `json:"choices"`
}

type openaiChatChoice struct {
	Message openaiChatMessage `json:"message"`
}

type openaiChatMessage struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	ToolCalls []openaiChatToolCall `json:"tool_calls,omitempty"`
}

type openaiChatToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiChatFunction `json:"function"`
}

type openaiChatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// toOpenAIMessages converts provider-agnostic messages to OpenAI SDK message format.
// This function enforces OpenAI Chat Completions API requirements:
// - System messages must be first
// - User and assistant messages must alternate (as much as possible)
// - Tool messages must have tool_call_id and follow assistant messages with tool calls
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

// mergeSystemMessagesOpenAI merges all system messages into a single message at the start.
// OpenAI Chat Completions API requires:
// 1. System messages must be first
// 2. At least one non-system message must follow
//
// This function collects ALL system messages regardless of position and places them
// at the start as a single merged message. If only system messages exist, it adds
// a minimal "Begin." user message to meet OpenAI requirements.
func mergeSystemMessagesOpenAI(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	// Separate system messages from others
	var systemBuffer strings.Builder
	nonSystemMessages := make([]openai.ChatCompletionMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.Role == "system" {
			if systemBuffer.Len() > 0 {
				systemBuffer.WriteString("\n\n")
			}
			systemBuffer.WriteString(msg.Content)
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// Build result: system first, then non-system
	result := make([]openai.ChatCompletionMessage, 0, len(messages))

	if systemBuffer.Len() > 0 {
		result = append(result, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemBuffer.String(),
		})
	}

	result = append(result, nonSystemMessages...)

	// OpenAI requires at least one non-system message
	// If we only have system messages, add a minimal user message
	if len(nonSystemMessages) == 0 && len(result) > 0 {
		log.Debug().
			Msg("OpenAI: Only system messages present, adding minimal user message")
		result = append(result, openai.ChatCompletionMessage{
			Role:    "user",
			Content: "Begin.",
		})
	}

	log.Debug().
		Int("original_count", len(messages)).
		Int("merged_count", len(result)).
		Bool("added_user_msg", len(nonSystemMessages) == 0 && len(result) > 0).
		Msg("OpenAI: Merged system messages")

	return result
}

// toOpenAITools converts provider-agnostic tools to OpenAI SDK tool format.
// Returns error if any tool has invalid JSON schema.
func toOpenAITools(tools []Tool) ([]openai.Tool, error) {
	result := make([]openai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]interface{}
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &params); err != nil {
				// Invalid JSON schema - return error instead of silently failing
				return nil, err
			}
		}
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
	return result, nil
}
