# KNOWN ISSUES:

These are the currently know issues to investigate:

- [ ] Myses have trouble login in, and mostly resort to new accoutns. We need to create a new table for username/password pairs available to the swarm, and which ones are in use or not. the introduce a new zoea tool to getch this data. Revise the prompt to ensure we encorage re-using swarm acounts first, and only create new ones as a fallback. If a mysis attempts to login to an active account, provide an directive response "use these available details" or "register a new one"

- [ ] Myses should know to prefer more recent information in their context. We should improve our context messages list to compact repeated state updates into the most recent state snapshot. We should also re-enforce the use of search tools to access older memory. Maybe increase the tool loop for longer autonomy.

- [ ] Mysis prompt review: Encorage myses to collaborate together via zoea tooling and create a goal and colaborate towards it. Encourage to pick a Crustacian Cosmos + Zoea/Mysis themed username. Critical rules must be in every prompt, not only the first one.

- [ ] Captain's Log Bug: There is a consistent pattern of Myses failing to use captains_log_add. They appear to be trying to "remember" their registration password but are encountering an empty_entry error from the MCP server. This suggests the LLM might be misformatting the arguments for this specific tool.
      **BUG**: Myses find each other's passwords and fail to login. Myses do not have a reliable way to re-use usernames they created in the past.

- [ ] Travel Duration: The travel to the asteroid belt is a long-duration task (30k+ ticks). The Myses correctly entered a "waiting" mode, showing they can manage long-term state without wasting tokens on redundant actions.
      **BUG**: Mysis should continue to act on his tick during travel. They can still use other actions when traveling. Like chatting, or other non traveling actions in the game. Currently the Mysis looses his ticks waiting when he could be using the turns to explore the game api and interacting with the swarm.

- [ ] TUI usability issues: Swarm messages list isnot kept up to date. Sending a broadcast doesn't update the list of swarm messages.
      **BUG**: Breaking change from the spec, the TUI should be high performance and in sync with the data at all times. When a message is sent by the commander, we need a "sending message state" with animation until the commander can send a new message. Stay consistent with zoea theme. check documentation for design rules.

- [ ] Broadcast messages are labelled YOU in focus view. We don't differentiate between swam broadcast senders. We need to start tracking it and reflecting it in the UI, in a consistent formatting and style acording to our thematic and design rules (see the documentation before doing changes)

- [ ] Reasoning messages are not displayed in the UI. They should appear in the focus mysis view, and be rendered with the purple color we use for text else where.

- [ ] The project shouldn't use emojis at all. Replace emoji usage with unicode accross all files in the project.

- [ ] Cognitive Looping and Prompt Inefficiency: Myses can get stuck in "cooldown" loops or redundant "waiting" states, consuming tokens every 30 seconds without operational progress.
      **BUG**: The system prompts Myses every tick regardless of their state. Myses may hallucinate non-existent cooldowns or fail to use non-traveling actions during long journeys.

## Notes:

I need to remmeber this to make a workflow command later:

```
Follow the plan, create todo list first. Stop if the anything deviates from the plan.
```
