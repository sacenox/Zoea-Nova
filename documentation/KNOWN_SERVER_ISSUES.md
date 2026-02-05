# Known SpaceMolt Server Issues

This document tracks known issues with the upstream SpaceMolt MCP server API. We do not modify or validate the game server's API behavior. Issues are documented here, and we improve prompts and error handling instead.

## captains_log_add: empty_entry Error

**Issue:** The `captains_log_add` tool returns an `empty_entry` error when:
- The `entry` field is an empty string
- The `entry` field is missing
- The `entry` field contains only whitespace

**Server Response:**
```json
{"error":{"code":0,"message":"empty_entry"}}
```

**Workaround:**
- System prompt includes explicit examples of correct usage
- Error messages provide actionable guidance when this error occurs

## MCP Initialization Handshake Failure

**Issue:** The SpaceMolt MCP server fails to complete the standard MCP initialization handshake. After issuing a session ID in response to the `initialize` request, the server immediately rejects the required `notifications/initialized` notification with a `404: session not found` error.

**MCP Protocol Flow:**
1. Client sends `initialize` request with protocol version and capabilities
2. Server responds with session ID in `Mcp-Session-Id` header
3. Client sends `notifications/initialized` notification with session ID
4. Server acknowledges and handshake is complete

**Observed Behavior:**
- Step 1-2: Server correctly returns session ID (e.g., `Mcp-Session-Id: 52ab67ce836578fa3ea340b321b94d4b`)
- Step 3: Server rejects notification with `HTTP 404: session not found`
- Step 4: Never reached

**Server Response:**
```
HTTP/1.1 404 Not Found
session not found
```

**Error Log Entry:**
```json
{
  "level": "error",
  "error": "initialize upstream: send initialized notification: http error 404: session not found\n",
  "upstream": "https://game.spacemolt.com/mcp",
  "message": "Failed to initialize MCP upstream - game tools will be unavailable"
}
```

**Testing Evidence:**

Manual reproduction confirms the issue persists regardless of:
- Timing (immediate vs delayed notification)
- Connection reuse (same TCP connection vs new connection)
- SSE stream handling (fully read vs partially read)

```bash
# Step 1: Initialize - SUCCESS
curl -X POST https://game.spacemolt.com/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test", "version": "1.0"}
    }
  }'
# Returns: Mcp-Session-Id: <valid-session-id> (200 OK)

# Step 2: Send notification - FAILS
curl -X POST https://game.spacemolt.com/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <valid-session-id>" \
  -d '{
    "jsonrpc": "2.0",
    "method": "notifications/initialized"
  }'
# Returns: 404 session not found
```

**Impact:**
- Game tools (mine, travel, trade, dock, etc.) are unavailable to Myses
- Only local orchestrator tools work (zoea_swarm_status, zoea_list_myses, etc.)
- Application continues to run but Myses cannot interact with SpaceMolt game
- Error is logged at startup but does not crash the TUI

**Code Location:**
- Initialization triggered: `cmd/zoea/main.go:110`
- Proxy initialization: `internal/mcp/proxy.go:193`
- Client handshake: `internal/mcp/client.go:213-234`
- Failing notification: `internal/mcp/client.go:230`

**Root Cause Analysis:**

This appears to be a server-side bug in the SpaceMolt MCP implementation. Possible causes:
1. Session IDs expire immediately after issuance
2. Session storage is not persisting between requests
3. Session validation logic is checking wrong identifier
4. Server expects notification on same HTTP/2 stream (not tested)
5. Server doesn't properly implement MCP specification

**Attempted Workarounds:**

None successful. The following were tested without resolving the issue:
- Sending notification immediately after receiving session ID
- Reusing HTTP connection with keep-alive
- Fully reading SSE stream before sending notification
- Using different HTTP clients and timeout configurations

**Status:**

**RESOLVED** - The server now correctly returns `202 Accepted` to the `notifications/initialized` request. Handshake completes successfully.

**Previous Issue (Fixed):**
The server previously returned `404 Not Found` to the initialization notification. This was verified fixed on 2026-02-05.

**Last Verified:** 2026-02-05
**Server Endpoint:** https://game.spacemolt.com/mcp
**Zoea Nova Version:** v1.5.7
