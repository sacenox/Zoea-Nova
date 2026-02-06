# OpenAI Chat Completions API Compatibility

This document describes how Zoea Nova maintains compatibility with the OpenAI Chat Completions API specification for all OpenAI-compatible providers.

## Architecture Principles

1. **OpenAI Compliance First**: All OpenAI-compatible providers (OpenCode Zen, etc.) MUST follow strict OpenAI Chat Completions API spec
2. **Ollama Isolation**: Ollama-specific customizations stay in `ollama.go` and are NOT used by other providers
3. **Shared OpenAI Types**: Common OpenAI-compliant code lives in `openai_common.go`

## Provider Types

### OpenAI-Compatible Providers
- **OpenCode Zen**: Strict OpenAI compliance
- **Future providers**: Any provider following OpenAI Chat Completions API

These providers use:
- `openai_common.go` types and functions
- `mergeSystemMessagesOpenAI()` for system message handling
- OpenAI SDK types (`openai.ChatCompletionRequest`, etc.)

### Ollama Provider
- **Ollama**: Custom provider with flexible API

Uses Ollama-specific code:
- Custom types (`ollamaReqMessage`, `chatCompletionResponse`, etc.)
- `mergeConsecutiveSystemMessagesOllama()` for flexible system handling
- Custom `reasoning()` method for Ollama extensions

## OpenAI Chat Completions API Requirements

### Message Ordering
1. System messages MUST be first
2. User and assistant messages should alternate
3. Tool result messages MUST follow assistant messages with tool calls
4. At least one non-system message MUST be present

### Tool Calls
1. Tool calls must have unique IDs
2. Tool results must reference valid tool call IDs
3. Tool arguments must be valid JSON
4. Tool results must follow the corresponding tool call message

### Validation Rules
- Empty tool_call_id → skip message
- Orphaned tool results (no matching tool call) → remove message
- Invalid JSON in tool arguments → use empty object `{}`
- Only system messages → add fallback user message "Begin."

## Code Structure

```
internal/provider/
├── openai_common.go       # Shared OpenAI-compliant code
│   ├── toOpenAIMessages()
│   ├── mergeSystemMessagesOpenAI()
│   └── toOpenAITools()
├── opencode.go            # OpenCode Zen (uses openai_common.go)
└── ollama.go              # Ollama (isolated, custom types)
    ├── toOllamaMessages()
    ├── mergeConsecutiveSystemMessagesOllama()
    └── reasoning() method
```

## Adding New OpenAI-Compatible Providers

To add a new OpenAI-compatible provider:

1. Create `internal/provider/<provider>.go`
2. Import and use `openai_common.go` functions:
   - `toOpenAIMessages()`
   - `mergeSystemMessagesOpenAI()`
   - `toOpenAITools()`
3. Use OpenAI SDK types for requests
4. Use `openaiChatResponse` types for responses
5. DO NOT use Ollama-specific code
6. Test against actual OpenAI API if possible

## Testing OpenAI Compliance

All OpenAI-compatible providers must pass:
- Message ordering validation tests
- Tool call/result association tests
- System message merging tests
- Edge case handling (orphaned tools, malformed data, etc.)

See `internal/provider/provider_test.go` for test patterns.

## References

- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/create)
- `documentation/guides/OPENCODE_ZEN_MODELS.md`
