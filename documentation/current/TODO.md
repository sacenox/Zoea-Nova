# TODO

**RULES**

- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.
- Managed by the user, ask before edit

---

- Example item: example description [KEEP FOR REFERENCE ONLY]

---

- Myses appear running, but network indicator shows idle, this should be impossible. Specially with 4 myses.

- sending direct message to mysis doesn't reset the message input box. Needs test coverage and fix.

- remove zoea_list_mysis()

- Investigate no content from AI messages in coversation log. only happens ocasionally.

```
 T0 ⬡ [19:05] AI: ──────────────────────────────────────────────────────────────────────────────────────────────────  │
 Sold 10 Iron Ore for 🟢40 credits (Skill Level 0, +1 XP). Current credits: 764.                                      │
                                                                                                                      │
 Cargo now contains 12 Iron Ore (was 22). Your mining lasers are still active. Would you like to:                     │
 1. Mine more ore to fill cargo                                                                                       │
 2. Sell remaining ore at NPC market                                                                                  │
 3. Check current market prices for Iron Ore                                                                          │
 4. Deploy mining drones to increase efficiency                                                                       │
                                                                                                                      │
 Your goal of 10,000 credits for a hauler is still far off (currently at 3,500). Would you like to                    │
 adjust your strategy?                                                                                                │
                                                                                                                      │
 T0 ⬡ [19:05] REASONING: ───────────────────────────────────────────────────────────────────────────────────────────  │
 Okay, let me see what's going on here. The user is playing as Nova Zoea, a voidborn empire                           │
 [17 more]                                                                                                            │
 credit goal.                                                                                                         │
                                                                                                                      │
                                                                                                                      │
 T0 ⬡ [19:07] AI: ──────────────────────────────────────────────────────────────────────────────────────────────────  │
 (no response)                                                                                                        │
                                                                                                                      │
 T0 ⬡ [19:08] YOU: ─────────────────────────────────────────────────────────────────────────────────────────────────  │
 What was your last reply?                                                                                            │
                                                                                                                      │
 T0 ⬡ [19:08] AI: ──────────────────────────────────────────────────────────────────────────────────────────────────  │
 The last reply was selling 10 units of Iron Ore, earning 40 credits and gaining 1 XP.                                █
                                                                                                                      █
 T0 ⬡ [19:08] REASONING: ───────────────────────────────────────────────────────────────────────────────────────────  █
 Okay, the user is asking, "What was your last reply?" Let me think about how to approach                             █
 [58 more]                                                                                                            █
 last reply was the sale of 10 Iron Ore, earning 40 credits and 1 XP.                                                 █
```

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
