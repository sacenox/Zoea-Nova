---
description: Runs a database snapshot analysis and reports on agent behavior and tool usage.
agent: analyser
model: opencode/gemini-3-flash
---

Instructions:

Analyze the last 5 minutes of activity from the database, application logs, and Ollama server logs, using 30-second intervals for the snapshot analysis.

1.  **Baseline**: Establish the system state as it was 5 minutes ago (active agents, memory counts) to use as a reference point.
2.  **Investigation**: For each 30-second interval within the 5-minute window:
    - Query the `agents` and `memories` tables for state changes and new messages.
    - Examine `zoea.log` for application events, tool calls, or errors.
    - Review `journalctl -u ollama` for provider performance or issues.
3.  **Analyze**: Aggregate the findings from the entire 5-minute window.
4.  **Report**: Provide a detailed report on:
    - **Agent Activity**: Active agents and their state transitions during the period.
    - **Tool Efficacy**: Tools called, their frequency, and success/failure rates.
    - **Observed Patterns**: Recurring issues, performance bottlenecks, repetitive or anomalous behaviors.

**IMPORTANT**:

- Do not create any files.
- Do not modify any data in the database.
- Ensure your report is data-driven, referencing specific log entries or query results.
