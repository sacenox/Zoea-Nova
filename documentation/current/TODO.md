# TODO

**RULES**

- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.
- Managed by the user, ask before edit

---

- Example item: example description [KEEP FOR REFERENCE ONLY]

---

- Myses become idle even when there are broadcasts from both commander and other mysis. Broadcasts should be sent as user messages to keep myses active.

- When nudging idle myses, encourage use of broadcasts to restart other idle myses in the swarm.

> RELEASE CUTOFF ----~ 8< ~---- STOP HERE **DON'T TOUCH** ----~ 8< ~----

- Refactor Help/controls pannel and error message displays in TUI.
  - Compact all controls in one help panel, show only [HELP h/H] on the right edge of the line.
  - Use the rest of the line to say:
    - A thematic sentence that all systems are ok when there are no errors.
    - If there are errored mysis, or any error non-game related (connection to game server mcp, app errors like timeouts etc. Dont confuse this with errors in game.) Display a truncated messahge, property formatted with timestamp, error source, type, truncated message.

- when run with -debug, reset the log file for a clean run.

Follow-up (optional):

1. Clean up skipped tests (8 tests with documented rationale)
2. Investigate TestStateTransition_Running_To_Idle goroutine hang
3. Fix TUI test environment issues (unrelated to this release)
