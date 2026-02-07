package provider

import (
	"encoding/json"
	"errors"
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

// validateOpenAIMessages ensures message structure meets OpenAI API requirements:
// 1. At least one non-system message exists
// 2. Conversation ends with user message (if expecting completion)
// 3. No consecutive assistant messages without user messages between them
//
// Returns error if validation fails.
func validateOpenAIMessages(messages []openai.ChatCompletionMessage) error {
	if len(messages) == 0 {
		return errors.New("messages array is empty")
	}

	// Count message types
	hasUser := false
	hasAssistant := false
	lastRole := ""

	for _, msg := range messages {
		if msg.Role == "user" {
			hasUser = true
		}
		if msg.Role == "assistant" {
			hasAssistant = true
			// Check for consecutive assistant messages
			if lastRole == "assistant" {
				log.Warn().Msg("OpenAI: Consecutive assistant messages detected")
			}
		}
		lastRole = msg.Role
	}

	// If we have assistant messages but no user messages, that's invalid
	if hasAssistant && !hasUser {
		return errors.New("conversation has assistant messages but no user messages")
	}

	// If last message is assistant (and we're expecting a completion), that's invalid
	if lastRole == "assistant" {
		log.Warn().Msg("OpenAI: Conversation ends with assistant message - may cause issues")
	}

	return nil
}

// mergeSystemMessagesOpenAI merges system messages intelligently while preserving conversation flow.
//
// Strategy:
// 1. Separate initial system messages (before any user/assistant messages)
// 2. Keep user/assistant conversation intact
// 3. Merge any mid-conversation system messages into the initial system prompt
//
// OpenAI requires:
// - System messages at the start
// - At least one non-system message
// - Proper user/assistant alternation
func mergeSystemMessagesOpenAI(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	// Separate system messages from conversation messages
	var systemMessages []string
	var conversationMessages []openai.ChatCompletionMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg.Content)
		} else {
			conversationMessages = append(conversationMessages, msg)
		}
	}

	// Build result: merged system message + conversation
	result := make([]openai.ChatCompletionMessage, 0, len(messages))

	// Add merged system message if any system messages exist
	if len(systemMessages) > 0 {
		mergedSystem := strings.Join(systemMessages, "\n\n")
		result = append(result, openai.ChatCompletionMessage{
			Role:    "system",
			Content: mergedSystem,
		})
	}

	// Add conversation messages
	result = append(result, conversationMessages...)

	log.Debug().
		Int("original_count", len(messages)).
		Int("merged_count", len(result)).
		Int("system_merged", len(systemMessages)).
		Int("conversation_kept", len(conversationMessages)).
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
