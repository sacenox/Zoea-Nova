---
description: Investigate the first item in the known issues document.
agent: analyser
model: opencode/gpt-5.2-codex
---

Instructions:

Read `documentation/KNOWN_ISSUES.md` and:

- Remove completed items first. If no items remain stop, and report back. Update KNOWN_ISSUES.md to reflect that no isses exist.
- Then investigate the first item. Create a plan and a todo list for the fixes and tests required to verify the changes.

**IMPORTANT**:

- Do not create files.
- Read the documentation before investigating.
- Gather all required context first.
- Use explorer subagents to gather information.
