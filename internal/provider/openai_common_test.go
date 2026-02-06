package provider

import (
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// TestMergeSystemMessagesOpenAI_MultipleSystemMessages tests that multiple
// system messages are merged into a single message at the start.
func TestMergeSystemMessagesOpenAI_MultipleSystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "system", Content: "Be concise."},
		{Role: "assistant", Content: "Hi!"},
		{Role: "system", Content: "Use simple words."},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 merged system + 2 non-system + 1 continuation = 4 messages
	// (Continuation added because conversation ends with assistant message)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// First message should be merged system message
	if result[0].Role != "system" {
		t.Errorf("expected first role=system, got %s", result[0].Role)
	}
	expected := "You are helpful.\n\nBe concise.\n\nUse simple words."
	if result[0].Content != expected {
		t.Errorf("expected content=%q, got %q", expected, result[0].Content)
	}

	// Non-system messages should follow in order
	if result[1].Role != "user" || result[1].Content != "Hello" {
		t.Errorf("expected user message 'Hello', got role=%s content=%q", result[1].Role, result[1].Content)
	}
	if result[2].Role != "assistant" || result[2].Content != "Hi!" {
		t.Errorf("expected assistant message 'Hi!', got role=%s content=%q", result[2].Role, result[2].Content)
	}

	// Last should be continuation prompt added by workaround
	if result[3].Role != "user" || result[3].Content != "Continue." {
		t.Errorf("expected continuation prompt, got role=%s content=%q", result[3].Role, result[3].Content)
	}
}

// TestMergeSystemMessagesOpenAI_OnlySystemMessages tests that when only
// system messages exist, a fallback "Begin." user message is added.
func TestMergeSystemMessagesOpenAI_OnlySystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "system", Content: "Be concise."},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 merged system + 1 fallback user = 2 messages
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// First message should be merged system
	if result[0].Role != "system" {
		t.Errorf("expected first role=system, got %s", result[0].Role)
	}
	expected := "You are helpful.\n\nBe concise."
	if result[0].Content != expected {
		t.Errorf("expected content=%q, got %q", expected, result[0].Content)
	}

	// Second message should be fallback user message
	if result[1].Role != "user" {
		t.Errorf("expected second role=user, got %s", result[1].Role)
	}
	if result[1].Content != "Begin." {
		t.Errorf("expected content='Begin.', got %q", result[1].Content)
	}
}

// TestMergeSystemMessagesOpenAI_NoSystemMessages tests that when no system
// messages exist, the messages are returned unchanged.
func TestMergeSystemMessagesOpenAI_NoSystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 2 messages + 1 continuation = 3 messages
	// (Continuation added because conversation ends with assistant message)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Errorf("expected user message unchanged, got role=%s content=%q", result[0].Role, result[0].Content)
	}
	if result[1].Role != "assistant" || result[1].Content != "Hi!" {
		t.Errorf("expected assistant message unchanged, got role=%s content=%q", result[1].Role, result[1].Content)
	}

	// Last should be continuation prompt added by workaround
	if result[2].Role != "user" || result[2].Content != "Continue." {
		t.Errorf("expected continuation prompt, got role=%s content=%q", result[2].Role, result[2].Content)
	}
}

// TestMergeSystemMessagesOpenAI_PreservesConversation tests that conversation
// messages are preserved when system messages are merged.
func TestMergeSystemMessagesOpenAI_PreservesConversation(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System 1"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "system", Content: "System 2"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm good"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 system (merged) + 4 conversation messages + 1 continuation = 6 total
	// (Continuation added because conversation ends with assistant message)
	if len(result) != 6 {
		t.Errorf("expected 6 messages, got %d", len(result))
	}

	// First should be merged system
	if result[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", result[0].Role)
	}
	expected := "System 1\n\nSystem 2"
	if result[0].Content != expected {
		t.Errorf("expected content=%q, got %q", expected, result[0].Content)
	}

	// Rest should be conversation in order
	if result[1].Role != "user" || result[1].Content != "Hello" {
		t.Errorf("user message 1 not preserved, got role=%s content=%q", result[1].Role, result[1].Content)
	}
	if result[2].Role != "assistant" || result[2].Content != "Hi there" {
		t.Errorf("assistant message 1 not preserved, got role=%s content=%q", result[2].Role, result[2].Content)
	}
	if result[3].Role != "user" || result[3].Content != "How are you?" {
		t.Errorf("user message 2 not preserved, got role=%s content=%q", result[3].Role, result[3].Content)
	}
	if result[4].Role != "assistant" || result[4].Content != "I'm good" {
		t.Errorf("assistant message 2 not preserved, got role=%s content=%q", result[4].Role, result[4].Content)
	}

	// Last should be continuation prompt added by workaround
	if result[5].Role != "user" || result[5].Content != "Continue." {
		t.Errorf("expected continuation prompt, got role=%s content=%q", result[5].Role, result[5].Content)
	}
}

// TestMergeSystemMessagesOpenAI_EmptyInput tests empty input.
func TestMergeSystemMessagesOpenAI_EmptyInput(t *testing.T) {
	messages := []openai.ChatCompletionMessage{}

	result := mergeSystemMessagesOpenAI(messages)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d messages", len(result))
	}
}

// TestMergeSystemMessagesOpenAI_SystemFirst tests that system messages are
// moved to the start when mixed with other messages.
func TestMergeSystemMessagesOpenAI_SystemFirst(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hello"},
		{Role: "system", Content: "You are helpful."},
		{Role: "assistant", Content: "Hi!"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 system + 2 non-system + 1 continuation = 4 messages
	// (Continuation added because conversation ends with assistant message)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// System should be first
	if result[0].Role != "system" {
		t.Errorf("expected first role=system, got %s", result[0].Role)
	}
	if result[0].Content != "You are helpful." {
		t.Errorf("expected content='You are helpful.', got %q", result[0].Content)
	}

	// Non-system messages should follow
	if result[1].Role != "user" {
		t.Errorf("expected second role=user, got %s", result[1].Role)
	}
	if result[2].Role != "assistant" {
		t.Errorf("expected third role=assistant, got %s", result[2].Role)
	}

	// Last should be continuation prompt added by workaround
	if result[3].Role != "user" || result[3].Content != "Continue." {
		t.Errorf("expected continuation prompt, got role=%s content=%q", result[3].Role, result[3].Content)
	}
}

// TestToOpenAIMessages_ToolCalls tests conversion of assistant messages
// with tool calls.
func TestToOpenAIMessages_ToolCalls(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "What's the weather?"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{
					ID:        "call_123",
					Name:      "get_weather",
					Arguments: json.RawMessage(`{"location":"NYC"}`),
				},
				{
					ID:        "call_456",
					Name:      "get_forecast",
					Arguments: json.RawMessage(`{"days":3}`),
				},
			},
		},
	}

	result := toOpenAIMessages(messages)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}

	// Check user message
	if result[0].Role != "user" || result[0].Content != "What's the weather?" {
		t.Errorf("expected user message, got role=%s content=%q", result[0].Role, result[0].Content)
	}

	// Check assistant message with tool calls
	assistantMsg := result[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected role=assistant, got %s", assistantMsg.Role)
	}
	if len(assistantMsg.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(assistantMsg.ToolCalls))
	}

	// Check first tool call
	tc1 := assistantMsg.ToolCalls[0]
	if tc1.ID != "call_123" {
		t.Errorf("expected ID=call_123, got %s", tc1.ID)
	}
	if tc1.Type != openai.ToolTypeFunction {
		t.Errorf("expected Type=function, got %s", tc1.Type)
	}
	if tc1.Function.Name != "get_weather" {
		t.Errorf("expected Name=get_weather, got %s", tc1.Function.Name)
	}
	if tc1.Function.Arguments != `{"location":"NYC"}` {
		t.Errorf("expected Arguments={\"location\":\"NYC\"}, got %s", tc1.Function.Arguments)
	}

	// Check second tool call
	tc2 := assistantMsg.ToolCalls[1]
	if tc2.ID != "call_456" {
		t.Errorf("expected ID=call_456, got %s", tc2.ID)
	}
	if tc2.Function.Name != "get_forecast" {
		t.Errorf("expected Name=get_forecast, got %s", tc2.Function.Name)
	}
}

// TestToOpenAIMessages_ToolResults tests conversion of tool result messages
// with tool_call_id.
func TestToOpenAIMessages_ToolResults(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "What's the weather?"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call_123", Name: "get_weather", Arguments: json.RawMessage(`{}`)},
			},
		},
		{
			Role:       "tool",
			Content:    `{"temp":72,"condition":"sunny"}`,
			ToolCallID: "call_123",
		},
	}

	result := toOpenAIMessages(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Check tool result message
	toolMsg := result[2]
	if toolMsg.Role != "tool" {
		t.Errorf("expected role=tool, got %s", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "call_123" {
		t.Errorf("expected ToolCallID=call_123, got %s", toolMsg.ToolCallID)
	}
	if toolMsg.Content != `{"temp":72,"condition":"sunny"}` {
		t.Errorf("unexpected content: %s", toolMsg.Content)
	}
}

// TestToOpenAIMessages_EmptyMessages tests empty input.
func TestToOpenAIMessages_EmptyMessages(t *testing.T) {
	messages := []Message{}

	result := toOpenAIMessages(messages)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d messages", len(result))
	}
}

// TestToOpenAIMessages_EmptyContent tests messages with empty content.
func TestToOpenAIMessages_EmptyContent(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: ""},
		{Role: "assistant", Content: ""},
	}

	result := toOpenAIMessages(messages)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Content != "" {
		t.Errorf("expected empty content, got %q", result[0].Content)
	}
	if result[1].Content != "" {
		t.Errorf("expected empty content, got %q", result[1].Content)
	}
}

// TestToOpenAITools_ValidJSON tests conversion of tools with valid JSON schema.
func TestToOpenAITools_ValidJSON(t *testing.T) {
	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}`),
		},
		{
			Name:        "get_time",
			Description: "Get current time",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"timezone":{"type":"string"}}}`),
		},
	}

	result, err := toOpenAITools(tools)
	if err != nil {
		t.Fatalf("toOpenAITools() error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}

	// Check first tool
	tool1 := result[0]
	if tool1.Type != openai.ToolTypeFunction {
		t.Errorf("expected Type=function, got %s", tool1.Type)
	}
	if tool1.Function.Name != "get_weather" {
		t.Errorf("expected Name=get_weather, got %s", tool1.Function.Name)
	}
	if tool1.Function.Description != "Get current weather" {
		t.Errorf("expected Description='Get current weather', got %s", tool1.Function.Description)
	}

	// Check parameters are valid map
	params1, ok := tool1.Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Parameters to be map[string]interface{}, got %T", tool1.Function.Parameters)
	}
	if params1["type"] != "object" {
		t.Errorf("expected type=object, got %v", params1["type"])
	}
	props1, ok := params1["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties to be map, got %T", params1["properties"])
	}
	if _, ok := props1["location"]; !ok {
		t.Error("expected location property")
	}

	// Check second tool
	tool2 := result[1]
	if tool2.Function.Name != "get_time" {
		t.Errorf("expected Name=get_time, got %s", tool2.Function.Name)
	}
}

// TestToOpenAITools_InvalidJSON tests that invalid JSON schema returns error.
func TestToOpenAITools_InvalidJSON(t *testing.T) {
	tools := []Tool{
		{
			Name:        "bad_tool",
			Description: "Tool with invalid JSON",
			Parameters:  json.RawMessage(`{invalid json}`),
		},
	}

	result, err := toOpenAITools(tools)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}

// TestToOpenAITools_EmptyParameters tests tools with empty parameters.
func TestToOpenAITools_EmptyParameters(t *testing.T) {
	tools := []Tool{
		{
			Name:        "no_params",
			Description: "Tool with no parameters",
			Parameters:  json.RawMessage(``),
		},
	}

	result, err := toOpenAITools(tools)
	if err != nil {
		t.Fatalf("toOpenAITools() error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	// Should have default object schema with empty properties
	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Parameters to be map[string]interface{}, got %T", result[0].Function.Parameters)
	}
	if params["type"] != "object" {
		t.Errorf("expected type=object, got %v", params["type"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties to be map, got %T", params["properties"])
	}
	if len(props) != 0 {
		t.Errorf("expected empty properties, got %v", props)
	}
}

// TestToOpenAITools_NilParameters tests tools with nil parameters.
func TestToOpenAITools_NilParameters(t *testing.T) {
	tools := []Tool{
		{
			Name:        "nil_params",
			Description: "Tool with nil parameters",
			Parameters:  nil,
		},
	}

	result, err := toOpenAITools(tools)
	if err != nil {
		t.Fatalf("toOpenAITools() error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	// Should have default object schema
	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Parameters to be map[string]interface{}, got %T", result[0].Function.Parameters)
	}
	if params["type"] != "object" {
		t.Errorf("expected type=object, got %v", params["type"])
	}
}

// TestToOpenAITools_Empty tests empty tool list.
func TestToOpenAITools_Empty(t *testing.T) {
	tools := []Tool{}

	result, err := toOpenAITools(tools)
	if err != nil {
		t.Fatalf("toOpenAITools() error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d tools", len(result))
	}
}

// TestToOpenAITools_ComplexSchema tests tools with complex nested schemas.
func TestToOpenAITools_ComplexSchema(t *testing.T) {
	complexSchema := `{
		"type": "object",
		"properties": {
			"user": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number"}
				},
				"required": ["name"]
			},
			"tags": {
				"type": "array",
				"items": {"type": "string"}
			}
		},
		"required": ["user"]
	}`

	tools := []Tool{
		{
			Name:        "create_user",
			Description: "Create a new user",
			Parameters:  json.RawMessage(complexSchema),
		},
	}

	result, err := toOpenAITools(tools)
	if err != nil {
		t.Fatalf("toOpenAITools() error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	params, ok := result[0].Function.Parameters.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Parameters to be map[string]interface{}, got %T", result[0].Function.Parameters)
	}

	// Verify nested structure
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties to be map, got %T", params["properties"])
	}

	userProp, ok := props["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user property to be map, got %T", props["user"])
	}

	if userProp["type"] != "object" {
		t.Errorf("expected user type=object, got %v", userProp["type"])
	}

	tagsProp, ok := props["tags"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tags property to be map, got %T", props["tags"])
	}

	if tagsProp["type"] != "array" {
		t.Errorf("expected tags type=array, got %v", tagsProp["type"])
	}
}

// TestValidateOpenAIMessages_Valid tests validation passes for valid conversation.
func TestValidateOpenAIMessages_Valid(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	err := validateOpenAIMessages(messages)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestValidateOpenAIMessages_EmptyArray tests validation fails for empty messages array.
func TestValidateOpenAIMessages_EmptyArray(t *testing.T) {
	messages := []openai.ChatCompletionMessage{}

	err := validateOpenAIMessages(messages)
	if err == nil {
		t.Error("Expected error for empty messages array")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' error, got: %v", err)
	}
}

// TestValidateOpenAIMessages_AssistantWithoutUser tests validation fails for assistant without user message.
func TestValidateOpenAIMessages_AssistantWithoutUser(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System"},
		{Role: "assistant", Content: "Hi"},
	}

	err := validateOpenAIMessages(messages)
	if err == nil {
		t.Error("Expected error for assistant without user message")
	}
	if !strings.Contains(err.Error(), "assistant") || !strings.Contains(err.Error(), "user") {
		t.Errorf("Expected assistant/user error, got: %v", err)
	}
}

// TestValidateOpenAIMessages_OnlySystemMessages tests validation passes for only system messages.
func TestValidateOpenAIMessages_OnlySystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System 1"},
		{Role: "system", Content: "System 2"},
	}

	// Only system messages is valid (but will trigger fallback user message in merging)
	err := validateOpenAIMessages(messages)
	if err != nil {
		t.Errorf("Expected no error for only system messages, got: %v", err)
	}
}
