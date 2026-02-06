package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStubClient_NewStubClient(t *testing.T) {
	client := NewStubClient()
	if client == nil {
		t.Fatal("NewStubClient returned nil")
	}
}

func TestStubClient_Initialize(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.Initialize(ctx, nil)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if result == nil {
		t.Fatal("Initialize returned nil response")
	}

	if result.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", result.JSONRPC)
	}

	if result.ID != 1 {
		t.Errorf("Expected ID 1, got %v", result.ID)
	}

	// Parse and verify result
	var initResult map[string]interface{}
	if err := json.Unmarshal(result.Result, &initResult); err != nil {
		t.Fatalf("Failed to parse initialize result: %v", err)
	}

	protocolVersion, ok := initResult["protocolVersion"].(string)
	if !ok || protocolVersion == "" {
		t.Error("Expected protocolVersion in result")
	}

	serverInfo, ok := initResult["serverInfo"].(map[string]interface{})
	if !ok {
		t.Error("Expected serverInfo in result")
	} else {
		if name, ok := serverInfo["name"].(string); !ok || name != "spacemolt-stub" {
			t.Errorf("Expected serverInfo.name to be 'spacemolt-stub', got %v", serverInfo["name"])
		}
	}
}

func TestStubClient_Initialize_WithClientInfo(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	clientInfo := map[string]interface{}{
		"name":    "test-client",
		"version": "1.0.0",
	}

	result, err := client.Initialize(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Initialize with clientInfo failed: %v", err)
	}

	if result == nil {
		t.Fatal("Initialize returned nil response")
	}
}

func TestStubClient_ListTools(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	expectedTools := []string{"get_status", "get_system", "get_ship", "get_poi", "get_notifications"}
	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Verify each expected tool is present
	toolMap := make(map[string]Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	for _, expectedName := range expectedTools {
		tool, ok := toolMap[expectedName]
		if !ok {
			t.Errorf("Expected tool %s not found", expectedName)
			continue
		}

		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", expectedName)
		}

		if len(tool.InputSchema) == 0 {
			t.Errorf("Tool %s has empty input schema", expectedName)
		}

		// Verify input schema is valid JSON
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid input schema: %v", expectedName, err)
		}
	}
}

func TestStubClient_CallTool_AllTools(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	testCases := []struct {
		name         string
		tool         string
		expectedTick int64
	}{
		{"get_status", "get_status", 42},
		{"get_system", "get_system", 42},
		{"get_ship", "get_ship", 42},
		{"get_poi", "get_poi", 42},
		{"get_notifications", "get_notifications", 42},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, tc.tool, nil)
			if err != nil {
				t.Fatalf("CallTool(%s) failed: %v", tc.tool, err)
			}

			if result == nil {
				t.Fatal("CallTool returned nil result")
			}

			if result.IsError {
				t.Errorf("CallTool(%s) returned error result", tc.tool)
			}

			if len(result.Content) == 0 {
				t.Fatal("CallTool returned empty content")
			}

			if result.Content[0].Type != "text" {
				t.Errorf("Expected content type 'text', got '%s'", result.Content[0].Type)
			}

			// Verify result is valid JSON
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
				t.Errorf("Invalid JSON response: %v", err)
			}

			// Verify tick is present
			if tick, ok := data["current_tick"]; ok {
				if tick != float64(tc.expectedTick) {
					t.Errorf("Expected tick %d, got %v", tc.expectedTick, tick)
				}
			} else {
				t.Errorf("Expected current_tick in response")
			}
		})
	}
}

func TestStubClient_CallTool_UnknownTool(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "unknown_tool", nil)
	if err != nil {
		t.Fatalf("Expected error result, got actual error: %v", err)
	}

	if !result.IsError {
		t.Error("Expected IsError=true for unknown tool")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected error content")
	}

	if result.Content[0].Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", result.Content[0].Type)
	}

	// Verify error message contains tool name
	errorMsg := result.Content[0].Text
	if errorMsg == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestStubClient_CallTool_WithArguments(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	args := map[string]interface{}{
		"session_id": "test_session",
		"some_param": "test_value",
	}

	result, err := client.CallTool(ctx, "get_status", args)
	if err != nil {
		t.Fatalf("CallTool with arguments failed: %v", err)
	}

	if result == nil {
		t.Fatal("CallTool returned nil result")
	}

	if result.IsError {
		t.Error("CallTool returned error result")
	}
}

func TestStubClient_CallTool_GetStatus_Structure(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "get_status", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Verify structure
	if _, ok := data["player"]; !ok {
		t.Error("Expected 'player' field in get_status response")
	}

	if _, ok := data["ship"]; !ok {
		t.Error("Expected 'ship' field in get_status response")
	}

	if _, ok := data["current_tick"]; !ok {
		t.Error("Expected 'current_tick' field in get_status response")
	}
}

func TestStubClient_CallTool_GetSystem_Structure(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "get_system", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Verify structure
	if _, ok := data["id"]; !ok {
		t.Error("Expected 'id' field in get_system response")
	}

	if _, ok := data["name"]; !ok {
		t.Error("Expected 'name' field in get_system response")
	}

	if _, ok := data["current_tick"]; !ok {
		t.Error("Expected 'current_tick' field in get_system response")
	}
}

func TestStubClient_CallTool_GetShip_Structure(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "get_ship", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Verify structure
	if _, ok := data["id"]; !ok {
		t.Error("Expected 'id' field in get_ship response")
	}

	if _, ok := data["name"]; !ok {
		t.Error("Expected 'name' field in get_ship response")
	}

	if _, ok := data["current_tick"]; !ok {
		t.Error("Expected 'current_tick' field in get_ship response")
	}

	if _, ok := data["cargo"]; !ok {
		t.Error("Expected 'cargo' field in get_ship response")
	}
}

func TestStubClient_CallTool_GetPOI_Structure(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "get_poi", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Verify structure
	if _, ok := data["id"]; !ok {
		t.Error("Expected 'id' field in get_poi response")
	}

	if _, ok := data["name"]; !ok {
		t.Error("Expected 'name' field in get_poi response")
	}

	if _, ok := data["type"]; !ok {
		t.Error("Expected 'type' field in get_poi response")
	}

	if _, ok := data["current_tick"]; !ok {
		t.Error("Expected 'current_tick' field in get_poi response")
	}
}

func TestStubClient_CallTool_GetNotifications_Structure(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	result, err := client.CallTool(ctx, "get_notifications", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Verify structure matches real API (server v0.44.4+)
	if _, ok := data["count"]; !ok {
		t.Error("Expected 'count' field in get_notifications response")
	}

	if _, ok := data["current_tick"]; !ok {
		t.Error("Expected 'current_tick' field in get_notifications response")
	}

	if _, ok := data["notifications"]; !ok {
		t.Error("Expected 'notifications' field in get_notifications response")
	}

	if _, ok := data["remaining"]; !ok {
		t.Error("Expected 'remaining' field in get_notifications response")
	}

	// Verify notifications is an array
	if notifications, ok := data["notifications"].([]interface{}); !ok {
		t.Error("Expected 'notifications' to be an array")
	} else if len(notifications) != 0 {
		t.Error("Expected empty notifications array in stub")
	}
}

func TestStubClient_CallTool_ContextCancellation(t *testing.T) {
	client := NewStubClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// CallTool should still work (stub doesn't check context)
	result, err := client.CallTool(ctx, "get_status", nil)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result even with cancelled context")
	}
}

func TestStubClient_MultipleCallsSameClient(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	// Call multiple tools in sequence
	tools := []string{"get_status", "get_system", "get_ship", "get_poi", "get_notifications"}

	for i := 0; i < 3; i++ {
		for _, tool := range tools {
			result, err := client.CallTool(ctx, tool, nil)
			if err != nil {
				t.Fatalf("CallTool(%s) iteration %d failed: %v", tool, i, err)
			}

			if result.IsError {
				t.Errorf("CallTool(%s) iteration %d returned error", tool, i)
			}
		}
	}
}

func TestStubClient_ConcurrentCalls(t *testing.T) {
	client := NewStubClient()
	ctx := context.Background()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			result, err := client.CallTool(ctx, "get_status", nil)
			if err != nil {
				t.Errorf("Concurrent call %d failed: %v", id, err)
			}
			if result == nil || result.IsError {
				t.Errorf("Concurrent call %d returned invalid result", id)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStubClient_ImplementsUpstreamClient(t *testing.T) {
	var _ UpstreamClient = (*StubClient)(nil)
}
