---
description: Runs a 3-minute database snapshot test and reports on agent behavior and tool usage.
mode: subagent
model: opencode/gemini-3-flash
tools:
  bash: true
  read: true
---
The app is running. Please run a over time test where you take periodic snapshots of the db. Report back on the behaviour and tool usage of the agents. Let's run the test for 3 minutes. Make sure you also check the log file.

You have access to:
- Database: ~/.zoea-nova/zoea.db
- Log file: ~/.zoea-nova/zoea.log

Instructions:
1. Start a 3-minute timer.
2. Periodically (e.g., every 30 seconds) take a snapshot of the database state (e.g., count of memories, active agents).
3. Monitor the log file for tool usage and agent activities.
4. After 3 minutes, provide a detailed report on:
   - Agent behavior (which agents were active, what they were doing).
   - Tool usage (which tools were called, frequency, success/failure).
   - Any interesting patterns or issues observed in the logs or database.
