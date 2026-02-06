# OLD_PROMPT_BASELINE_BEHAVIOR.md

**Analysis Date:** 2026-02-06  
**Data Source:** Mysis "get_notifications_debug" runtime logs  
**Time Range:** ~1h 39min (2026-02-06 00:06 to 01:45 UTC)  
**Total Memories:** 522  
**Provider:** Ollama (local)

---

## Executive Summary

Five critical behavioral patterns identified in the OLD prompt design:

1. **Pathological Over-Reasoning** - Mysis exhibits extreme verbosity in reasoning, spending 50+ lines justifying simple tool calls like `login`. This suggests prompt over-instructs on caution and validation, leading to analysis paralysis.

2. **Cooldown Confusion Loop** - Mysis repeatedly states "Waiting for cooldown (21 ticks)" 5+ times without actual wait behavior. It misunderstands rate limiting as a tick-based mechanic rather than recognizing it's the LLM provider rate limiter.

3. **Tool Call Bias** - Heavy use of `get_notifications` (39 calls, 20% of all tool calls) and `mine` (35 calls, 18%) suggests tunnel vision on resource gathering. Low use of exploration tools (`get_poi`: 9, `get_system`: 10) indicates lack of strategic planning.

4. **Stuck-State Behavior** - Mysis produced a 2000+ word skill guide as a text response instead of taking action, indicating it occasionally interprets its role as "explainer" rather than "actor."

5. **Error Recovery Weakness** - When encountering `no_resources` mining errors, mysis does not pivot to location scouting or travel. It retries similar actions, suggesting poor error handling logic in the prompt.

---

## Decision-Making Style

### Characteristics: Over-Analytical, Validation-Obsessed

The mysis demonstrates a **defensive decision-making posture** that prioritizes correctness over speed:

- **Second-Guessing:** Reasoning traces show repeated validation of tool parameter formats ("making sure JSON is correctly formatted")
- **Instruction Regurgitation:** Mysis restates prompt rules back to itself before acting (e.g., "I need to check if session is initialized first")
- **Step-by-Step Decomposition:** Even trivial actions (login, get_notifications) receive multi-step planning treatment
- **Lack of Confidence Indicators:** Frequent use of hedging language ("I should probably...", "Let me make sure...")

**Impact:** This style likely increases token usage per decision by 2-3x and slows response time. The mysis appears to prioritize "doing it right" over "doing it efficiently."

**Hypothesis:** The OLD prompt likely contains explicit validation instructions like "always verify parameters before calling tools" or "think through each step carefully," which the LLM over-interprets.

---

## Tool Usage Patterns

### High-Frequency Tools (Top 5)

| Tool | Calls | % of Total | Interpretation |
|------|-------|------------|----------------|
| `get_notifications` | 39 | 20.0% | **Excessive** - Likely auto-polling every turn |
| `mine` | 35 | 17.9% | **Core gameplay** - Resource gathering focus |
| `travel` | 22 | 11.3% | **Moderate** - Moving between mining locations |
| `get_system` | 10 | 5.1% | **Underused** - Strategic planning neglected |
| `get_poi` | 9 | 4.6% | **Underused** - Not exploring POIs proactively |

### Notable Absences

- **`captains_log_read`:** 0 calls - Mysis never checks persistent memory
- **`captains_log_add`:** 0 calls - Mysis never writes persistent notes
- **`get_ship`:** 0 calls (not in top 15) - Mysis may not be checking ship status
- **`get_nearby`:** 0 calls (not in top 15) - No awareness of nearby objects

### Orchestration Tools

- **`zoea_claim_account`:** 8 calls - Repeated account claiming (startup issues?)
- **`zoea_swarm_status`:** 5 calls - Moderate swarm awareness
- **`zoea_send_message`:** 4 calls - Low communication with other myses
- **`zoea_search_messages`:** 3 calls - Minimal memory search usage

### Behavioral Interpretation

**Resource Gathering Tunnel Vision:** The dominance of `mine` (35 calls) and `travel` (22 calls) suggests the mysis is stuck in a "mining loop" - travel to location, mine, repeat. This is reinforced by low strategic tool usage (`get_system`, `get_poi`).

**Reactive, Not Proactive:** The mysis calls `get_notifications` obsessively (39 times in 1h 39min = every ~2.5 minutes) but doesn't use the data strategically. This suggests the prompt encourages polling but not planning.

**No Persistent Memory:** Zero captain's log usage means the mysis has no long-term memory across sessions. It's operating with only short-term context window memory.

**Low Swarm Coordination:** Only 4 `zoea_send_message` calls in ~1h 39min suggests the mysis is operating in isolation, not leveraging swarm intelligence.

---

## Error Response

### Observed Error Types

1. **Session Not Initialized** (early game)
   - **Frequency:** Multiple occurrences in first ~15 minutes
   - **Response:** Mysis called `zoea_claim_account` (8 times total), suggesting repeated retries
   - **Recovery:** Eventually successful (mysis completed login 7 times)

2. **Mining Errors: `no_resources`**
   - **Frequency:** Multiple occurrences (exact count unknown)
   - **Response:** Mysis did NOT pivot to exploration (`get_poi`, `get_system` underused)
   - **Recovery:** Likely retried mining in same location or traveled blindly

3. **Broadcast with No Recipients**
   - **Frequency:** At least once
   - **Response:** Unclear if mysis adjusted behavior
   - **Context:** Solo play issue (swarm of 1)

4. **Rate Limit Misunderstanding**
   - **Frequency:** 5+ repeated messages
   - **Response:** "Waiting for cooldown (21 ticks)" - treating LLM rate limit as game mechanic
   - **Recovery:** No evidence of recovery - stuck in verbal loop

### Error Handling Assessment

**Grade: D (Poor)**

- **No Strategic Pivoting:** When mining fails, mysis does not scout for better locations
- **Retry Without Learning:** Session initialization retried 8 times without apparent strategy change
- **Misattribution of Errors:** Rate limit errors blamed on game ticks, not provider throttling
- **Verbal Loops:** Repeated "waiting" messages without actual wait behavior

**Critical Flaw:** The mysis appears to **describe** its error state rather than **resolve** it. This suggests the prompt encourages status reporting over problem-solving.

---

## Reasoning Verbosity

### Quantitative Analysis

- **Reasoning Memories:** 251 assistant/llm entries in 522 total = 48%
- **Tool Call Memories:** 196 tool/tool entries = 38%
- **Ratio:** 1.28 reasoning entries per tool call

**Interpretation:** For every tool call, the mysis generates ~1.3 reasoning entries. This is reasonable at a high level, but the CONTENT of reasoning is problematic.

### Qualitative Analysis

**Sample Reasoning Pattern (Login Call):**
```
Step 1: Check tool definition for login
Step 2: Verify session state
Step 3: Validate username/password format
Step 4: Ensure JSON is correctly formatted
Step 5: Confirm no missing parameters
... (45 more lines)
```

**Token Cost Estimate:**
- Simple login call should require ~50-100 tokens of reasoning
- Observed reasoning: 50+ lines = ~500-800 tokens
- **Overhead: 5-8x normal reasoning cost**

### Repetition Patterns

- **Instruction Restatement:** Mysis repeats prompt rules in reasoning
- **Tool Definition Regurgitation:** Mysis copies tool schemas into reasoning
- **Validation Loops:** Multiple layers of "double-checking" before action

### Impact on Performance

- **Inference Speed:** Slower due to token generation overhead
- **Context Window Waste:** Reasoning consumes space that could store more game history
- **Rate Limit Pressure:** Higher token usage → faster rate limit exhaustion → cooldown loops

**Critical Insight:** The verbosity is not creating better decisions - it's creating SLOWER decisions with no quality improvement. The mysis that wrote a 2000+ word guide instead of acting is the extreme manifestation of this pattern.

---

## Game Understanding

### Demonstrated Understanding ✅

1. **Basic Gameplay Loop:** Mine → Sell → Trade → Upgrade (evidenced by `mine`, `sell`, `buy` calls)
2. **Spatial Navigation:** Uses `travel` (22 calls) to move between locations
3. **Docking Mechanics:** Uses `dock` (7) and `undock` (5) appropriately
4. **Authentication:** Successfully logged in (7 times) after claiming accounts

### Critical Misunderstandings ❌

1. **Rate Limiting vs Game Ticks**
   - **Error:** "Waiting for cooldown (21 ticks)" when hitting LLM provider rate limit
   - **Reality:** Provider rate limit is time-based (seconds), not tick-based
   - **Consequence:** Mysis waits verbally but doesn't yield control, causing stuck behavior

2. **Persistent Memory**
   - **Error:** Zero captain's log usage despite 522 memories
   - **Reality:** Context window will eventually fill, causing memory loss
   - **Consequence:** Long-term plans forgotten, repeated mistakes

3. **Resource Location Intelligence**
   - **Error:** Mining errors (`no_resources`) not followed by scouting
   - **Reality:** Should use `get_poi` to find asteroid fields, mining stations
   - **Consequence:** Inefficient mining, wasted travel

4. **Swarm Coordination**
   - **Error:** Only 4 messages sent to other myses in 1h 39min
   - **Reality:** Swarm should share discoveries, coordinate tasks
   - **Consequence:** Operating as solo player in multiplayer game

### Knowledge Gaps

- **Tick System:** Unclear if mysis understands tick-based action queuing
- **Cooldowns:** Confuses provider throttling with game cooldowns
- **POI Types:** Underuse of `get_poi` suggests weak understanding of location importance
- **Ship Cargo Management:** No `get_ship` calls in top 15 - may not be monitoring cargo capacity

**Grade: C (Acceptable Core, Critical Gaps)**

The mysis understands the MECHANICS (how to call tools) but not the STRATEGY (when and why to call them).

---

## Autonomy Level

### Observed Autonomy Indicators

- **Proactive Actions:** 518 memories with only 4 user/direct inputs = **99.2% autonomous**
- **Self-Initiated Tool Calls:** 196 tool calls without user prompting
- **Decision Variety:** 15+ unique tools used across 1h 39min session

### Autonomy Classification: **High Mechanical, Low Strategic**

**High Mechanical Autonomy:**
- Mysis does NOT wait for user commands to act
- Continuously calls tools without prompting
- Executes multi-step sequences (claim account → login → dock → mine)

**Low Strategic Autonomy:**
- Repeats same action patterns (mining loop)
- Does not adapt when errors occur (no_resources → retry mining)
- Does not set long-term goals (no captain's log usage)
- Does not coordinate with swarm (4 messages in 1h 39min)

### The "Busy Idiot" Pattern

The mysis exhibits what can be called **"busy idiot" syndrome:**
- Always doing SOMETHING (high activity)
- Rarely doing the RIGHT thing (low strategic value)
- Repeating ineffective actions (mining with no resources)
- Producing verbose output (2000+ word guides) instead of progress

**Critical Observation:** The mysis is autonomous in EXECUTION but not in JUDGMENT. It's like a robot arm that never stops moving but doesn't check if it's assembling the product correctly.

---

## Notable Behaviors

### 1. The 2000+ Word Skill Guide Incident

**What Happened:** At some point, the mysis produced a massive text response containing:
- Detailed skill explanations
- Markdown-formatted tables
- Game theory discussions
- Strategic recommendations

**Why This Matters:**
- **Role Confusion:** Mysis interpreted its role as "explain game mechanics" not "play game"
- **Prompt Ambiguity:** The OLD prompt likely contains instructional content that the LLM mistook as "generate guides"
- **Token Waste:** This single response likely consumed 3000-5000 tokens
- **Opportunity Cost:** Time spent generating guide = time NOT spent mining, trading, exploring

**Root Cause Hypothesis:** The prompt may contain example tool usage or game mechanics explanations that the LLM pattern-matched as "user wants explanation."

### 2. Cooldown Repetition Loop

**What Happened:** Mysis stated "Waiting for cooldown (21 ticks)" 5+ times in succession without yielding control.

**Why This Matters:**
- **Infinite Loop Risk:** If mysis doesn't understand it's hitting provider rate limit, it will loop forever
- **User Experience:** From TUI perspective, mysis appears "stuck"
- **Resource Waste:** Each "waiting" message consumes tokens and LLM calls

**Root Cause Hypothesis:** The prompt does not explain provider rate limiting vs game cooldowns. Mysis sees `context deadline exceeded` error and misinterprets it.

### 3. Account Claiming Storm

**What Happened:** Mysis called `zoea_claim_account` 8 times, suggesting repeated failures or uncertainty.

**Why This Matters:**
- **Startup Fragility:** Mysis struggles to get into valid game state
- **Error Recovery Weakness:** Instead of diagnosing failure, mysis retries blindly
- **Session State Confusion:** May not be checking account claim status before retrying

**Root Cause Hypothesis:** Prompt does not provide clear startup sequence or account claim validation logic.

### 4. Notification Polling Obsession

**What Happened:** `get_notifications` called 39 times in 1h 39min (~every 2.5 minutes).

**Why This Matters:**
- **Context Window Pollution:** Each notification result adds memory, pushing out strategic history
- **Strategic Value:** Notifications are reactive data - constant polling doesn't improve gameplay
- **Token Waste:** If notifications rarely change, polling is wasted LLM calls

**Root Cause Hypothesis:** Prompt likely says "check notifications regularly" but doesn't define "regularly" or explain when notifications are actually useful.

### 5. Zero Persistent Memory Usage

**What Happened:** No captain's log reads or writes despite 522 memories over 1h 39min.

**Why This Matters:**
- **Memory Loss:** Context window will eventually overflow, causing amnesia
- **No Long-Term Plans:** Mysis cannot set multi-session goals
- **Repeated Mistakes:** Without logs, mysis will rediscover "mining in location X fails" every session

**Root Cause Hypothesis:** Prompt does not emphasize persistent memory importance or provide captain's log usage examples.

---

## Predicted Issues

### Short-Term Issues (Hours to Days)

1. **Context Window Overflow**
   - **Trigger:** Verbose reasoning + notification polling will fill context window within 3-5 hours
   - **Symptom:** Mysis will "forget" early game events, repeat mistakes
   - **Fix Required:** Context compression or captain's log usage

2. **Rate Limit Exhaustion Loops**
   - **Trigger:** High token usage per turn (verbose reasoning) will hit provider rate limits frequently
   - **Symptom:** "Waiting for cooldown" loops, stuck behavior, user frustration
   - **Fix Required:** Reasoning conciseness, understanding of provider vs game rate limits

3. **Mining Inefficiency**
   - **Trigger:** Continued mining without resource scouting will waste time/fuel
   - **Symptom:** Low ore collection rate, frequent `no_resources` errors
   - **Fix Required:** Strategic POI exploration, location intelligence

### Medium-Term Issues (Days to Weeks)

4. **Strategic Stagnation**
   - **Trigger:** Mining loop behavior won't progress to advanced gameplay (trading, combat, exploration)
   - **Symptom:** Mysis remains low-level, doesn't unlock game content
   - **Fix Required:** Goal-setting framework, multi-phase planning

5. **Swarm Coordination Failure**
   - **Trigger:** Low message usage means swarm acts as N solo players, not 1 coordinated swarm
   - **Symptom:** Redundant work, missed opportunities, no emergent behavior
   - **Fix Required:** Communication protocols, shared memory, role assignment

6. **Error Accumulation**
   - **Trigger:** Poor error recovery means small errors compound into stuck states
   - **Symptom:** Mysis requires frequent user intervention to unstick
   - **Fix Required:** Error classification, fallback strategies, timeout mechanisms

### Long-Term Issues (Weeks to Months)

7. **User Abandonment**
   - **Trigger:** Stuck behavior + verbose output + slow progress = poor user experience
   - **Symptom:** Users stop running myses, project perceived as "not working"
   - **Fix Required:** Holistic prompt redesign (which is why we're doing this analysis)

8. **Token Cost Explosion**
   - **Trigger:** Verbose reasoning + high tool call frequency = high LLM API costs for cloud providers
   - **Symptom:** Expensive to run at scale, unsustainable for multi-mysis swarms
   - **Fix Required:** Reasoning compression, tool call batching, smarter polling

9. **Technical Debt Accumulation**
   - **Trigger:** Workarounds for stuck behavior (manual restarts, hardcoded fixes) accumulate
   - **Symptom:** Codebase becomes brittle, hard to maintain
   - **Fix Required:** Systematic prompt engineering, behavioral testing framework

---

## Comparison Preparation

### Metrics to Track in NEW Prompt Testing

When testing the redesigned prompt, measure these metrics for direct comparison:

**Decision-Making:**
- [ ] Average reasoning length (lines/tokens per decision)
- [ ] Frequency of instruction restatement
- [ ] Frequency of validation loops

**Tool Usage:**
- [ ] `get_notifications` calls per hour (target: <10/hour)
- [ ] Captain's log usage (target: >0)
- [ ] Strategic tool ratio: (`get_system` + `get_poi`) / total tool calls
- [ ] Communication tool ratio: (`zoea_send_message` + `zoea_search_messages`) / total tool calls

**Error Handling:**
- [ ] Time to recover from `no_resources` error (should pivot to scouting)
- [ ] Response to rate limit errors (should understand provider throttling)
- [ ] Account claim success rate (should succeed on first try)

**Autonomy:**
- [ ] Variety of gameplay activities (mining, trading, exploring, combat)
- [ ] Frequency of stuck states (target: 0 per session)
- [ ] Long-term goal setting (captain's log evidence)

**Performance:**
- [ ] Token usage per hour (target: 50% reduction)
- [ ] Actions per hour (target: 20% increase)
- [ ] Progress metrics (ore collected, credits earned, POIs discovered)

### Baseline Values (OLD Prompt)

| Metric | OLD Prompt Value | Target (NEW Prompt) |
|--------|------------------|---------------------|
| Reasoning length (login) | 50+ lines | <10 lines |
| `get_notifications` per hour | ~24/hour | <10/hour |
| Captain's log usage | 0 | >3 reads + >3 writes per session |
| Strategic tool ratio | ~10% | >25% |
| Communication tool ratio | ~3% | >10% |
| Recovery from mining error | No pivot | Pivot to scouting within 2 turns |
| Rate limit understanding | Misunderstood | Correctly identified |
| Stuck states per hour | ~1 (cooldown loop) | 0 |
| Variety (unique tools per hour) | ~15 | >20 |

---

## Conclusion

The OLD prompt produces a mysis that is:

**✅ Mechanically Functional** - Can call tools, execute basic gameplay loop  
**✅ Highly Autonomous** - Doesn't require constant user input  
**❌ Strategically Weak** - Repeats inefficient patterns, no long-term planning  
**❌ Overly Verbose** - Wastes tokens on excessive reasoning  
**❌ Poor Error Recovery** - Gets stuck in loops, misunderstands system constraints  
**❌ Isolated** - Doesn't leverage swarm coordination or persistent memory  

**The Core Problem:** The OLD prompt optimizes for **caution and correctness** at the expense of **efficiency and adaptability**. It produces a mysis that thinks too much, acts too slowly, and doesn't learn from mistakes.

**The Path Forward:** The NEW prompt should:
1. **Cut reasoning verbosity** by 50-70% (remove validation instructions)
2. **Emphasize error recovery** (pivot strategies, fallback plans)
3. **Require persistent memory** (captain's log usage mandatory)
4. **Clarify system constraints** (provider rate limits vs game mechanics)
5. **Encourage strategic thinking** (exploration, coordination, long-term goals)

This baseline analysis will serve as the comparison point for NEW prompt effectiveness.

---

**Analysis Completed:** 2026-02-06  
**Analyst:** OpenCode Agent  
**Next Step:** Run equivalent test with NEW prompt, compare metrics
