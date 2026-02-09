# TODO

**RULES**

- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.
- Managed by the user, ask before edit

---

- Example item: example description [KEEP FOR REFERENCE ONLY]

---

Recommendations

1. Cargo management: Add logic to check cargo capacity before mining, trigger dock/sell when full
2. POI validation: Require get_poi before mine, use get_system for travel targets (stop hallucinating names)
3. Docking sequence: Ensure dock succeeds before calling station services (buy, sell, missions)
4. Increase iteration limit: From 10 to 15-20 to reduce turn overhead for complex tasks

7. Refactor OpenCode Zen provider factory: Make endpoint selection intelligent based on model type
   - Reference: https://opencode.ai/docs/zen/#models
   - Current: Hardcoded model→endpoint mapping in opencodeModelEndpoints map
   - Goal: Automatically route to correct endpoint (/chat/completions, /messages, /responses, /models/*) based on model prefix/type
   - Benefits: Support new models without code changes, align with official OpenCode Zen API structure
   - Note: /responses endpoint format is currently undocumented and incompatible with our OpenAI SDK usage

- network activity indicator revision

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
