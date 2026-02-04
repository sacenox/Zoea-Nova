# KNOWN ISSUES:

These are the currently known issues to investigate:

## Context & Memory Management

- [ ] Myses should know to prefer more recent information in their context. We should improve our context messages list to compact repeated state updates into the most recent state snapshot. We should also re-enforce the use of search tools to access older memory. Maybe increase the tool loop for longer autonomy.
- [ ] Evidence: Sliding-window context uses MaxContextMessages=20 with no compaction of repeated state snapshots.
- [ ] zoea exclusive fields in tool payload is unecessary bloat (the ollama model here.):
  > ````│ │ TOOL: call_42829tff:[                                                                                                  │  │
  > │  │       {                                                                                                                │  │
  > │  │       "id": "22c38707-f6a9-4643-8bc4-27b47fe31cbd",                                                                    │  │
  > │  │       "name": "test",                                                                                                  │  │
  > │  │       "provider": "ollama",                                                                                            │  │
  > │  │       "state": "running"                                                                                               │  │
  > │  │       }                                                                                                                │  │
  > │  │       ]```
  > ````

## Prompt & Behavior

- [ ] Mysis prompt review: Encourage myses to collaborate together via zoea tooling to create a goal and collaborate towards it. Encourage to pick a Crustacean Cosmos + Zoea/Mysis themed username. Critical rules must be in every prompt, not only the first one.
- [ ] Evidence: SystemPrompt includes collaboration + themed usernames; ContinuePrompt now includes limited CRITICAL REMINDERS but not the full CRITICAL RULES, so enforcement still depends on the system prompt being present in the 20-message window.
- [ ] mysis dont recognize their own messages in broadcast, so they reply to them

- [ ] Cognitive Looping and Prompt Inefficiency: Myses can get stuck in "cooldown" loops or redundant "waiting" states, consuming tokens every 30 seconds without operational progress. The system prompts Myses every tick regardless of their state. Myses may hallucinate non-existent cooldowns or fail to use non-traveling actions during long journeys.
- [ ] Evidence: ContinuePrompt is sent on a fixed 30-second ticker with no state-aware suppression.

## Gameplay Issues

- [ ] Travel Duration: The travel to the asteroid belt is a long-duration task (30k+ ticks). The Myses correctly entered a "waiting" mode, showing they can manage long-term state without wasting tokens on redundant actions. However, Mysis should continue to act on their tick during travel. They can still use other actions when traveling, like chatting or other non-traveling actions in the game. Currently the Mysis loses their ticks waiting when they could be using the turns to explore the game API and interacting with the swarm.
- [ ] Evidence: ContinuePrompt is always issued while running, even during travel/cooldown states.
- [ ] Mysis are aware of real time, they should only know tick time.

## TUI Issues

- [ ] Broadcast messages are labelled YOU in focus view. We don't differentiate between swarm broadcast senders. We need to start tracking it and reflecting it in the UI, in a consistent formatting and style according to our thematic and design rules (see the documentation before doing changes).
- [ ] Reasoning messages are not displayed in the UI. They should appear in the focus mysis view, and be rendered with the purple color we use for text elsewhere. Note: Reasoning capture and storage was implemented in v1.5.2, but UI display is still pending.
- [ ] Username status for each mysis focus view and in commander view.
- [ ] Evidence: Focus view labels are based on Role only (broadcast source ignored); reasoning is stored but not rendered; TUI models do not include account username fields.
- [ ] Json needs to be human formatted in TUI and preserve a "Tree" view of it use unicode for the "tree" visuals. TUI focus view needs a `verbose` toggle to show truncated or not. Use inteligent truncation: first 3 items, [x more], last 3 item.

## Ollama error:

- [ ] Investigate:

```
│  │ TOOL: call_a66zlzes:{                                                                                                  │  │
│  │       "id": "34657164-845d-423e-8cfd-9994199dc10f",                                                                    │  │
│  │       "last_error": "Post \"http://localhost:11434/v1/chat/completions\": context deadline exceeded",                  │  │
│  │       "name": "test2",                                                                                                 │  │
│  │       "provider": "ollama",                                                                                            │  │
│  │       "state": "running"                                                                                               │  │
│  │       }
```

## Opencode Zen auth

- [ ] Zen requires an api key, where are configuring this?

## Notes

- [ ] Create an OpenCode workflow command to enforce: "Save the plan, make a todo list, then follow the plan. Stop if anything deviates from the plan."

= [ ] another opencode command: "Audit our agents, readme, and documentation/ against the code. show me the differences or inconsistencies between the two. Use @explore to help."

- [x] Add a Make command that:
  - extracts username/passwords from the current DB to a root-level file
  - wipes the current DB
  - imports username/passwords from file
  - updates docs to use this when wiping the DB

- [ ] Validate MCP and game server updates for inconsistencies.
