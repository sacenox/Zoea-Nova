# KNOWN ISSUES:

These are the currently known issues to investigate:

## Authentication & Account Management

- [ ] Captain's Log Bug: There is a consistent pattern of Myses failing to use captains_log_add. They appear to be trying to "remember" their registration password but are encountering an empty_entry error from the MCP server. This suggests the LLM might be misformatting the arguments for this specific tool. Related: Myses find each other's passwords and fail to login. Myses do not have a reliable way to re-use usernames they created in the past.
- [ ] Evidence: System prompt mentions captains_log_add but does not define argument shape; MCP proxy forwards raw arguments upstream. Myses can send empty or malformed payloads and hit empty_entry.

## Context & Memory Management

- [ ] Myses should know to prefer more recent information in their context. We should improve our context messages list to compact repeated state updates into the most recent state snapshot. We should also re-enforce the use of search tools to access older memory. Maybe increase the tool loop for longer autonomy.
- [ ] Evidence: Sliding-window context uses MaxContextMessages=20 with no compaction of repeated state snapshots.

## Prompt & Behavior

- [ ] Mysis prompt review: Encourage myses to collaborate together via zoea tooling to create a goal and collaborate towards it. Encourage to pick a Crustacean Cosmos + Zoea/Mysis themed username. Critical rules must be in every prompt, not only the first one.
- [ ] Evidence: SystemPrompt includes collaboration + themed usernames; ContinuePrompt does not include CRITICAL RULES, so enforcement depends on system prompt being present in the 20-message window.

- [ ] Cognitive Looping and Prompt Inefficiency: Myses can get stuck in "cooldown" loops or redundant "waiting" states, consuming tokens every 30 seconds without operational progress. The system prompts Myses every tick regardless of their state. Myses may hallucinate non-existent cooldowns or fail to use non-traveling actions during long journeys.
- [ ] Evidence: ContinuePrompt is sent on a fixed 30-second ticker with no state-aware suppression.

## Gameplay Issues

- [ ] Travel Duration: The travel to the asteroid belt is a long-duration task (30k+ ticks). The Myses correctly entered a "waiting" mode, showing they can manage long-term state without wasting tokens on redundant actions. However, Mysis should continue to act on their tick during travel. They can still use other actions when traveling, like chatting or other non-traveling actions in the game. Currently the Mysis loses their ticks waiting when they could be using the turns to explore the game API and interacting with the swarm.
- [ ] Evidence: ContinuePrompt is always issued while running, even during travel/cooldown states.

## TUI Issues

- [ ] Broadcast messages are labelled YOU in focus view. We don't differentiate between swarm broadcast senders. We need to start tracking it and reflecting it in the UI, in a consistent formatting and style according to our thematic and design rules (see the documentation before doing changes).
- [ ] Reasoning messages are not displayed in the UI. They should appear in the focus mysis view, and be rendered with the purple color we use for text elsewhere. Note: Reasoning capture and storage was implemented in v1.5.2, but UI display is still pending.
- [ ] Username status for each mysis focus view and in commander view.
- [ ] Evidence: Focus view labels are based on Role only (broadcast source ignored); reasoning is stored but not rendered; TUI models do not include account username fields.

## Notes

- [ ] Create an OpenCode workflow command to enforce: "Follow the plan, create todo list first. Stop if anything deviates from the plan."

- [ ] Add a Make command that:
  - extracts username/passwords from the current DB to a root-level file
  - wipes the current DB
  - imports username/passwords from file
  - updates docs to use this when wiping the DB

- [ ] Validate MCP and game server updates for inconsistencies.
