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
