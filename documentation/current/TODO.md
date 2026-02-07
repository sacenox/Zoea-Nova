# TODO

**RULES**

- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.

---

- Example item: example description [KEEP FOR REFERENCE ONLY]

---

- when messaging directly to mysis to restart the llm loop, the TUI shows an error:

```
Error: mysis not running
```

- myses become idle even when a broadcast from commander exists.

- claim_account() marks account as is use, this is wrong, only login should do it.
- Server added session_id recently, ensure it's in context

- Auto start mysis on app start: make it into an option `--start-swarm`, default off.

- When the app is exiting, as the user presses `q` or `ESC` or `CTRL+C`, show a splash screen with branding, a text saying the app is closing connections and animated infinite loading bar, once connections are cleaned up, exit.. So the users knows what he is waiting for.

- tool messages need JSON rendering properly

- when nudging, encourage broadcast to restart other idle mysis

> RELEASE CUTOFF ----~ 8< ~----

- Refactor Help/controls pannel and error message displays in TUI.
  - Compact all controls in one help panel, show only [HELP h/H] on the right edge of the line.
  - Use the rest of the line to say:
    - A thematic sentence that all systems are ok when there are no errors.
    - If there are errored mysis, or any error non-game related (connection to game server mcp, app errors like timeouts etc. Dont confuse this with errors in game.) Display a truncated messahge, property formatted with timestamp, error source, type, truncated message.

- when run with -debug, reset the log file for a clean run.

- call\_-7908546343142072217:Error calling zoea_broadcast: broadcast failed: broadcast failed for 2
  Broadcast should never fail. Commander counts as online in broadcast.

Follow-up (optional):

1. Clean up skipped tests (8 tests with documented rationale)
2. Investigate TestStateTransition_Running_To_Idle goroutine hang
3. Fix TUI test environment issues (unrelated to this release)
