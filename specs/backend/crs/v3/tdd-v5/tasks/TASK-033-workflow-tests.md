# TASK-033: Workflow Tests

**Phase:** 5 — Workflow Orchestration  
**Prerequisite:** TASK-028, TASK-029, TASK-030  
**Status:** ✅ DONE — 2026-07-29

---

## Files cần tạo

### `src/main/workflow/__tests__/DAGBuilder.test.ts` (≥ 15 tests)

1. Linear A→B→C → 3 waves [[A],[B],[C]]
2. Parallel A,B,C → 1 wave [[A,B,C]]
3. Diamond A→(B,C)→D → 3 waves
4. Mixed: A→B, C→D, B→D, C→D → 2 waves
5. Single step → 1 wave
6. Cycle A→B→A → throws WorkflowCycleError
7. Self-cycle A→A → throws WorkflowCycleError
8. Missing dep ref → throws error
9. Empty steps → empty waves
10. Cycle in chain of 3 → throws with cycle array
11. Multiple independent subgraphs → correct wave assignment
12. Complex diamond → all in correct wave
13. Steps with no deps → first wave
14. Step with all deps in first wave → second wave
15. buildWaves: does not mutate input steps

### `src/main/workflow/__tests__/WorkflowOrchestrator.test.ts` (≥ 18 tests)

Use mocks for StepExecutors, pool.

1. execute: persists with status=pending → running
2. execute: buildWaves called
3. execute: steps in wave run in parallel (allSettled)
4. execute: marks completed on success
5. execute: marks failed on step error
6. execute: input interpolation ${inputs.x}
7. cancel: sets abort signal
8. cancel: running execution stops after current step
9. getExecution: returns persisted execution
10. resumeRunningExecutions: queries running executions
11. resumeRunningExecutions: resumes from currentWave
12. wave execution: updates currentWave in DB
13. continueOnError: step failure doesn't stop wave
14. !continueOnError: step failure stops execution
15. stepExecutors.execute called with signal
16. Multiple waves execute sequentially
17. Empty definition executes successfully
18. DB persistence: each step status persisted

### `src/main/workflow/__tests__/TemplateResolver.test.ts` (≥ 8 tests)

1. resolve: single template no parent
2. resolve: parent template merged
3. resolve: grandparent chain (depth 3)
4. resolve: MAX_INHERIT_DEPTH=5 enforced
5. resolve: leaf overrides parent
6. create: stores template in DB
7. list: returns user templates
8. list: returns company-scope templates

## Acceptance Criteria

- [x] ≥ 15 DAGBuilder tests pass (15 tests ✅)
- [x] ≥ 18 Orchestrator tests pass (18 tests ✅)
- [x] ≥ 8 TemplateResolver tests pass (10 tests ✅)
