# Loop Context Composition - Parallel Workflow Diagram

## Overall Workflow

```mermaid
flowchart TD
    Start([Start: Plan Execution]) --> Phase1[Phase 1: Test Infrastructure]
    Phase1 --> Review1{Review Checkpoint 1}
    Review1 -->|Pass| Phase2[Phase 2: Helper Implementation]
    Review1 -->|Fail| Rollback1[Rollback & Fix Phase 1]
    Rollback1 --> Phase1
    
    Phase2 --> Review2{Review Checkpoint 2}
    Review2 -->|Pass| Phase3[Phase 3: Context Integration]
    Review2 -->|Fail| Rollback2[Rollback & Fix Phase 2]
    Rollback2 --> Phase2
    
    Phase3 --> Review3{Review Checkpoint 3}
    Review3 -->|Pass| Phase4[Phase 4: Documentation]
    Review3 -->|Fail| Rollback3[Rollback & Fix Phase 3]
    Rollback3 --> Phase3
    
    Phase4 --> Review4{Review Checkpoint 4}
    Review4 -->|Pass| Phase5[Phase 5: Full Validation]
    Review4 -->|Fail| Rollback4[Rollback & Fix Phase 4]
    Rollback4 --> Phase4
    
    Phase5 --> FinalReview{Final Review}
    FinalReview -->|Pass| Success([Complete: Merge to Main])
    FinalReview -->|Fail| RollbackAll[Rollback Feature Branch]
    RollbackAll --> Phase1
```

## Phase 1: Test Infrastructure (3 Parallel Agents)

```mermaid
flowchart LR
    subgraph Phase1["Phase 1: Test Infrastructure"]
        direction TB
        A1[Agent 1<br/>Prompt Source<br/>Priority Tests] --> R1A
        A2[Agent 2<br/>Loop Slice Tests] --> R1A
        A3[Agent 3<br/>Tool Pairing Tests] --> R1A
        
        R1A[Review Agent A<br/>Test Coverage] --> Gate1
        R1B[Review Agent B<br/>Test Quality] --> Gate1
        
        Gate1{Gate 1:<br/>All Tests Fail<br/>as Expected?}
        Gate1 -->|Yes| Commit1[Commit:<br/>test: add loop context<br/>composition tests]
        Gate1 -->|No| Fix1[Fix Test Issues]
        Fix1 --> A1
        Fix1 --> A2
        Fix1 --> A3
    end
    
    Commit1 --> Phase2[To Phase 2]
```

## Phase 2: Helper Implementation (2 Parallel Agents)

```mermaid
flowchart LR
    subgraph Phase2["Phase 2: Helper Implementation"]
        direction TB
        A4[Agent 4<br/>selectPromptSource<br/>Implementation] --> R2A
        A5[Agent 5<br/>extractLatestToolLoop<br/>Implementation] --> R2A
        
        R2A[Review Agent C<br/>Implementation<br/>Correctness] --> Gate2
        R2B[Review Agent D<br/>Performance &<br/>Safety] --> Gate2
        
        Gate2{Gate 2:<br/>Helpers Pass<br/>Unit Tests?}
        Gate2 -->|Yes| Commit2[Commit:<br/>feat: add context<br/>composition helpers]
        Gate2 -->|No| Fix2[Fix Implementation]
        Fix2 --> A4
        Fix2 --> A5
    end
    
    Commit2 --> Phase3[To Phase 3]
```

## Phase 3: Context Integration (1 Serial Agent)

```mermaid
flowchart TD
    subgraph Phase3["Phase 3: Context Composition Integration"]
        direction TB
        A6[Agent 6<br/>Rewrite getContextMemories<br/>using new helpers] --> R3A
        
        R3A[Review Agent E<br/>Integration Review] --> R3B
        R3B[Review Agent F<br/>Regression Review] --> Gate3
        
        Gate3{Gate 3:<br/>All Tests Pass<br/>No Regressions?}
        Gate3 -->|Yes| Commit3[Commit:<br/>fix: compose context<br/>from prompt source<br/>and last tool loop]
        Gate3 -->|No| Fix3[Fix Integration Issues]
        Fix3 --> A6
    end
    
    Commit3 --> Phase4[To Phase 4]
```

## Phase 4: Documentation (2 Parallel Agents)

```mermaid
flowchart LR
    subgraph Phase4["Phase 4: Documentation & Validation"]
        direction TB
        A7[Agent 7<br/>Architecture<br/>Documentation] --> R4
        A8[Agent 8<br/>Code Comment<br/>Documentation] --> R4
        
        R4[Review Agent G<br/>Documentation<br/>Review] --> Gate4
        
        Gate4{Gate 4:<br/>Docs Match<br/>Implementation?}
        Gate4 -->|Yes| Commit4[Commit:<br/>docs: document loop<br/>slice context composition]
        Gate4 -->|No| Fix4[Fix Documentation]
        Fix4 --> A7
        Fix4 --> A8
    end
    
    Commit4 --> Phase5[To Phase 5]
```

## Phase 5: Full Validation (3 Parallel Agents)

```mermaid
flowchart LR
    subgraph Phase5["Phase 5: Full Validation"]
        direction TB
        A9[Agent 9<br/>Unit Test Validation<br/>Race Detection] --> R5
        A10[Agent 10<br/>Integration Test<br/>Validation] --> R5
        A11[Agent 11<br/>Build & Smoke<br/>Test Validation] --> R5
        
        R5[Review Agent H<br/>Release Readiness] --> Gate5
        
        Gate5{Gate 5:<br/>All Validations<br/>Pass?}
        Gate5 -->|Yes| Merge[Merge to Main<br/>& Tag Release]
        Gate5 -->|No| Fix5[Fix Issues]
        Fix5 --> A9
        Fix5 --> A10
        Fix5 --> A11
    end
    
    Merge --> Complete([Implementation Complete])
```

## Agent Dependency Graph

```mermaid
graph TD
    subgraph Legend
        direction LR
        Impl[Implementation Agent]
        Rev[Review Agent]
        Val[Validation Agent]
    end
    
    subgraph "Phase 1 (Parallel)"
        A1[Agent 1: Prompt Tests]
        A2[Agent 2: Loop Tests]
        A3[Agent 3: Pairing Tests]
    end
    
    subgraph "Review 1"
        RA[Review A: Coverage]
        RB[Review B: Quality]
    end
    
    A1 --> RA
    A2 --> RA
    A3 --> RA
    A1 --> RB
    A2 --> RB
    A3 --> RB
    
    subgraph "Phase 2 (Parallel)"
        A4[Agent 4: selectPromptSource]
        A5[Agent 5: extractLatestToolLoop]
    end
    
    subgraph "Review 2"
        RC[Review C: Correctness]
        RD[Review D: Performance]
    end
    
    RA --> A4
    RB --> A4
    RA --> A5
    RB --> A5
    
    A4 --> RC
    A5 --> RC
    A4 --> RD
    A5 --> RD
    
    subgraph "Phase 3 (Serial)"
        A6[Agent 6: Integration]
    end
    
    subgraph "Review 3"
        RE[Review E: Integration]
        RF[Review F: Regression]
    end
    
    RC --> A6
    RD --> A6
    A6 --> RE
    A6 --> RF
    
    subgraph "Phase 4 (Parallel)"
        A7[Agent 7: Arch Docs]
        A8[Agent 8: Code Comments]
    end
    
    subgraph "Review 4"
        RG[Review G: Documentation]
    end
    
    RE --> A7
    RF --> A7
    RE --> A8
    RF --> A8
    A7 --> RG
    A8 --> RG
    
    subgraph "Phase 5 (Parallel)"
        A9[Agent 9: Unit Tests]
        A10[Agent 10: Integration]
        A11[Agent 11: Build]
    end
    
    subgraph "Final Review"
        RH[Review H: Release]
    end
    
    RG --> A9
    RG --> A10
    RG --> A11
    A9 --> RH
    A10 --> RH
    A11 --> RH
    
    RH --> Done[Complete]
    
    style A1 fill:#a8dadc
    style A2 fill:#a8dadc
    style A3 fill:#a8dadc
    style A4 fill:#a8dadc
    style A5 fill:#a8dadc
    style A6 fill:#a8dadc
    style A7 fill:#a8dadc
    style A8 fill:#a8dadc
    style A9 fill:#457b9d
    style A10 fill:#457b9d
    style A11 fill:#457b9d
    style RA fill:#f1faee
    style RB fill:#f1faee
    style RC fill:#f1faee
    style RD fill:#f1faee
    style RE fill:#f1faee
    style RF fill:#f1faee
    style RG fill:#f1faee
    style RH fill:#f1faee
```

## Timing Analysis

```mermaid
gantt
    title Loop Context Composition - Execution Timeline
    dateFormat X
    axisFormat %S
    
    section Phase 1
    Agent 1: Prompt Tests           :a1, 0, 10
    Agent 2: Loop Tests             :a2, 0, 10
    Agent 3: Pairing Tests          :a3, 0, 10
    Review 1: Coverage & Quality    :r1, 10, 12
    Gate 1: Decision                :milestone, g1, 12, 0
    
    section Phase 2
    Agent 4: selectPromptSource     :a4, 12, 22
    Agent 5: extractLatestToolLoop  :a5, 12, 22
    Review 2: Correctness & Perf    :r2, 22, 24
    Gate 2: Decision                :milestone, g2, 24, 0
    
    section Phase 3
    Agent 6: Integration            :a6, 24, 44
    Review 3: Integration & Regression :r3, 44, 46
    Gate 3: Decision                :milestone, g3, 46, 0
    
    section Phase 4
    Agent 7: Arch Docs              :a7, 46, 56
    Agent 8: Code Comments          :a8, 46, 56
    Review 4: Documentation         :r4, 56, 58
    Gate 4: Decision                :milestone, g4, 58, 0
    
    section Phase 5
    Agent 9: Unit Validation        :a9, 58, 68
    Agent 10: Integration Validation :a10, 58, 68
    Agent 11: Build Validation      :a11, 58, 68
    Final Review: Release Readiness :r5, 68, 70
    Gate 5: Decision                :milestone, g5, 70, 0
```

**Total Time:** 70 time units (6T execution + 5 reviews)
**Sequential Equivalent:** 90 time units (8T + 5 reviews)
**Speedup:** 22% faster with parallelization

## Critical Path

```mermaid
flowchart LR
    Start --> P1[Phase 1: Tests<br/>10 units]
    P1 --> R1[Review 1<br/>2 units]
    R1 --> P2[Phase 2: Helpers<br/>10 units]
    P2 --> R2[Review 2<br/>2 units]
    R2 --> P3[Phase 3: Integration<br/>20 units]
    P3 --> R3[Review 3<br/>2 units]
    R3 --> P4[Phase 4: Docs<br/>10 units]
    P4 --> R4[Review 4<br/>2 units]
    R4 --> P5[Phase 5: Validation<br/>10 units]
    P5 --> R5[Final Review<br/>2 units]
    R5 --> End[Complete]
    
    style P3 fill:#e63946
    style R3 fill:#e63946
```

**Critical Path:** Phase 3 (Integration) - longest serial task at 20 units

## Success Metrics

```mermaid
pie title Success Criteria Checklist
    "Tests Pass" : 20
    "No Regressions" : 15
    "No Race Conditions" : 15
    "Docs Updated" : 15
    "Reviews Passed" : 20
    "Build Clean" : 15
```

---

## Usage

This workflow document should be used alongside:
- **Plan:** [2026-02-07-loop-context-composition.md](2026-02-07-loop-context-composition.md)
- **Workflow:** [2026-02-07-loop-context-parallel-workflow.md](2026-02-07-loop-context-parallel-workflow.md)

Agents should reference the appropriate phase section and follow the gate criteria before proceeding to the next phase.
