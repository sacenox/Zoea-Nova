# Offline Mode

Offline mode runs Zoea Nova without connecting to the SpaceMolt MCP server. It replaces the MCP client with a stub that returns mock data so the TUI and orchestration flow can be exercised safely.

## Run

```bash
./bin/zoea --offline
```

## Supported Tools

The stub MCP client implements these tools with mock payloads:

- `get_status`
- `get_system`
- `get_ship`
- `get_poi`

All other MCP tool calls return an error response indicating the tool is not implemented in the stub.

## What Works

- TUI navigation, focus mode, and command handling
- Mysis orchestration (create, start, stop, delete)
- Provider calls and LLM prompts
- Local orchestration tools (zoea_* tools)

## Limitations

- No live game actions (mine, travel, trade, etc.)
- Tool results are mock data and do not change based on game state
- Any MCP tool outside the supported list returns an error
