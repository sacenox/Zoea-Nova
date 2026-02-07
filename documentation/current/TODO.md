# TODO

**RULES**

- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.
- Managed by the user, ask before edit

---

- Example item: example description [KEEP FOR REFERENCE ONLY]

---

- REGRESSION: broadcast doesn't start idle myses. test coverage gap!!

- include most recent comander broadcast if exits in nudges.

- myses become idle even when a broadcasts.

- Auto start mysis on app start: make it into an option `--start-swarm`, default off.
- default provider/model selection:
  - update new mysis flow to prompt for config values, provider and model
  - name is required, provider can be empty. If it is empty then use the default provider and model.
    - if a provider was chosen, model becomes required (to ensure provider->model relation)
  - add default provider and model to the config file. (config.toml)

- Investigate

```
 T0 ⬡ [05:49] TOOL: ─────────────────────────────────────────────────────────────────  │
 call_nhgyw9u8:Error calling zoea_broadcast: broadcast failed:                         │
 broadcast failed for 1 mysis: mysis                                                   │
 378f805e-be8e-48ee-97f4-d3594d512286: mysis stopped - press 'r'                       │
 to relaunch
 [...]

 and raw json in tui

  T10462 ⬡ [08:00] AI: ────────────────────────────────────────────────────────────────────────────────────────────────  │
 ⚡ Calling tools:                                                                                                      │
   • get_status(session_id: "d5d2f6c5bc8d0c113e45fb0c6c3fe2cd")                                                         │
                                                                                                                        │
 T10462 ⬡ [08:00] TOOL: ──────────────────────────────────────────────────────────────────────────────────────────────  │
 chatcmpl-tool-8b4ad55fe5e842ef8fb65e63e221ff52:{                                                                       │
 "player": {                                                                                                            │
 "id": "756c1c972b8699e25275318aea0a8c45",                                                                              │
 "username": "CrabZoea",                                                                                                │
 "empire": "voidborn",                                                                                                  │
 "credits": 814,                                                                                                        │
 "created_at": "2026-02-06T02:30:30.808969221Z",                                                                        │
 "last_login_at": "2026-02-07T07:57:45.771921803Z",                                                                     │
 "last_active_at": "2026-02-07T07:58:47.315025653Z",                                                                    │
 "status_message": "",                                                                                                  │
 "clan_tag": "",                                                                                                        │
 "primary_color": "",                                                                                                   │
 "secondary_color": "",                                                                                                 │
 "anonymous": false,                                                                                                    │
 "is_cloaked": false,                                                                                                   │
 "current_ship_id": "e52832c032e5dc043e8e127f8d301150",                                                                 │
 "current_system": "nexus",
```

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
