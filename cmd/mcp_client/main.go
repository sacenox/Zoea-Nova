package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/mcp"
)

// parseFunctionCall parses function-style calls like: tool_name(arg1, arg2, key=value)
func parseFunctionCall(input string) (name string, args map[string]interface{}, ok bool) {
	// Match pattern: function_name(args...)
	re := regexp.MustCompile(`^(\w+)\((.*)\)$`)
	matches := re.FindStringSubmatch(input)
	if matches == nil {
		return "", nil, false
	}

	name = matches[1]
	argsStr := strings.TrimSpace(matches[2])
	args = make(map[string]interface{})

	if argsStr == "" {
		return name, args, true
	}

	// Split arguments by comma, respecting quotes
	argList := splitArgs(argsStr)

	// Track positional argument index
	positionalIndex := 0

	for _, arg := range argList {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		// Check if it's a key=value pair
		if idx := strings.Index(arg, "="); idx > 0 {
			key := strings.TrimSpace(arg[:idx])
			value := strings.TrimSpace(arg[idx+1:])
			args[key] = parseValue(value)
		} else {
			// Positional argument: use "arg0", "arg1", etc.
			args[fmt.Sprintf("arg%d", positionalIndex)] = parseValue(arg)
			positionalIndex++
		}
	}

	return name, args, true
}

// splitArgs splits comma-separated arguments, respecting quoted strings
func splitArgs(s string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch {
		case (ch == '"' || ch == '\'') && !inQuotes:
			inQuotes = true
			quoteChar = ch
			current.WriteRune(ch)
		case ch == quoteChar && inQuotes:
			inQuotes = false
			quoteChar = 0
			current.WriteRune(ch)
		case ch == ',' && !inQuotes:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// parseValue parses a string value into appropriate type (string, number, bool, JSON object/array)
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Try to parse as JSON first (for objects and arrays)
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		var result interface{}
		if err := json.Unmarshal([]byte(s), &result); err == nil {
			return result
		}
	}

	// Try boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try number
	if num, err := strconv.ParseFloat(s, 64); err == nil {
		// Return int if it's a whole number
		if num == float64(int(num)) {
			return int(num)
		}
		return num
	}

	// Try quoted string
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`)) {
		return s[1 : len(s)-1]
	}

	// Return as-is (unquoted string)
	return s
}

func main() {
	configPath := flag.String("config", "config.toml", "Path to config file")
	endpoint := flag.String("endpoint", "", "MCP server endpoint (overrides config)")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Use endpoint from config unless overridden by flag
	mcpEndpoint := cfg.MCP.Upstream
	if *endpoint != "" {
		mcpEndpoint = *endpoint
	}

	ctx := context.Background()
	client := mcp.NewClient(mcpEndpoint)

	// Initialize connection
	fmt.Printf("Connecting to %s...\n", mcpEndpoint)
	clientInfo := map[string]interface{}{
		"name":    "mcp_client",
		"version": "1.0.0",
	}

	initResp, err := client.Initialize(ctx, clientInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	if initResp.Error != nil {
		fmt.Fprintf(os.Stderr, "Initialization error: %s\n", initResp.Error.Message)
		os.Exit(1)
	}

	fmt.Println("Connected successfully!")
	fmt.Println()

	// List available tools
	tools, err := client.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list tools: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Available tools (%d):\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// Interactive loop
	fmt.Println("Enter tool calls:")
	fmt.Println("  Function style: tool_name(arg1, arg2, key=value)")
	fmt.Println("  JSON style: {\"name\":\"tool_name\",\"arguments\":{\"key\":\"value\"}}")
	fmt.Println("  Commands: 'list' to show tools, 'quit' to exit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			break
		}

		if input == "list" {
			fmt.Printf("Available tools (%d):\n", len(tools))
			for _, tool := range tools {
				fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
				if len(tool.InputSchema) > 0 {
					var schema map[string]interface{}
					if err := json.Unmarshal(tool.InputSchema, &schema); err == nil {
						schemaJSON, _ := json.MarshalIndent(schema, "    ", "  ")
						fmt.Printf("    Schema: %s\n", string(schemaJSON))
					}
				}
			}
			fmt.Println()
			continue
		}

		// Parse tool call - try function syntax first, then JSON
		var toolName string
		var toolArgs map[string]interface{}

		if name, args, ok := parseFunctionCall(input); ok {
			toolName = name
			toolArgs = args
		} else {
			// Try JSON format
			var toolCall struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}

			if err := json.Unmarshal([]byte(input), &toolCall); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid syntax: %v\n", err)
				fmt.Println("Expected: tool_name(args...) or {\"name\":\"tool_name\",\"arguments\":{...}}")
				continue
			}

			toolName = toolCall.Name
			toolArgs = toolCall.Arguments
		}

		if toolName == "" {
			fmt.Fprintf(os.Stderr, "Tool name is required\n")
			continue
		}

		// Call tool
		fmt.Printf("Calling tool: %s\n", toolName)
		if len(toolArgs) > 0 {
			argsJSON, _ := json.MarshalIndent(toolArgs, "  ", "  ")
			fmt.Printf("  Arguments: %s\n", string(argsJSON))
		}
		result, err := client.CallTool(ctx, toolName, toolArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Tool call failed: %v\n", err)
			continue
		}

		// Print result
		fmt.Println("Result:")
		if result.IsError {
			fmt.Println("  [ERROR]")
		}
		for _, content := range result.Content {
			if content.Type == "text" {
				fmt.Printf("  %s\n", content.Text)
			} else {
				fmt.Printf("  [%s content]\n", content.Type)
			}
		}
		fmt.Println()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Closing connection...")
	client.Close()
}
