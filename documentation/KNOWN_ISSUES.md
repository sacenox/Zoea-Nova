# KNOWN ISSUES:

These are the currently know issues to investigate:

- [x] Mysis terminology: Agent is too generic of a name for the swarm members.
      **FIXED**: Swarm members are now referred to as "Mysis" (singular) and "Myses" (plural) in documentation.

- [ ] Opencode Zen as a provider: This is completly untested. Compile a list of the free models, and create a report in `documentation` named `OPENCODE_ZEN_MODELS.md`. Only use free models in your report.
      **BUG**: each individual Mysis should be able to be configured with different provider/model choice. Currently all use the same model and provider. Keep a sensible default in `config.toml`. We need to also add per model temperature values.

- [ ] Captain's Log Bug: There is a consistent pattern of Myses failing to use captains_log_add. They appear to be trying to "remember" their registration password but are encountering an empty_entry error from the MCP server. This suggests the LLM might be misformatting the arguments for this specific tool.
      **BUG**: Myses find each other's passwords and fail to login. Myses do not have a reliable way to re-use usernames they created in the past.

- [ ] Travel Duration: The travel to the asteroid belt is a long-duration task (30k+ ticks). The Myses correctly entered a "waiting" mode, showing they can manage long-term state without wasting tokens on redundant actions.
      **BUG**: Mysis should continue to act on his tick during travel. They can still use other actions when traveling. Like chatting, or other non traveling actions in the game. Currently the Mysis looses his ticks waiting when he could be using the turns to explore the game api and interacting with the swarm. Add a `zoea_get_passwords()` tool call that lists password<>username pairs for the swarm. (searches swarm messags for the passwords), Myses should not see passwords for accounts actively being used by other swarm Myses.

- [ ] TUI usability issues: Swarm messages list isnot kept up to date. Sending a broadcast doesn't update the list of swarm messages.
      **BUG**: Breaking change from the spec, the TUI should be high performance and in sync with the data at all times.

- [ ] Cognitive Looping and Prompt Inefficiency: Myses can get stuck in "cooldown" loops or redundant "waiting" states, consuming tokens every 30 seconds without operational progress.
      **BUG**: The system prompts Myses every tick regardless of their state. Myses may hallucinate non-existent cooldowns or fail to use non-traveling actions during long journeys.
