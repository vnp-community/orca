/**
 * Tests for DAGBuilder (TDD-17) — TASK-033
 *
 * ≥ 15 tests covering wave computation, parallel execution, cycles, missing deps.
 *
 * @module main/workflow/__tests__/DAGBuilder.test
 */

import { describe, it, expect } from 'vitest'
import { DAGBuilder } from '../DAGBuilder'
import { WorkflowCycleError } from '../WorkflowTypes'
import type { WorkflowStep } from '../WorkflowTypes'

// ── helpers ──────────────────────────────────────────────────────────────────

function step(id: string, dependsOn?: string[]): WorkflowStep {
  return {
    id,
    name: `Step ${id}`,
    serverSpec: 'server:srv-1',
    dependsOn,
    config: { type: 'shell', script: 'echo done' },
  }
}

function waveIds(waves: WorkflowStep[][]): string[][] {
  return waves.map(w => w.map(s => s.id).sort())
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe('DAGBuilder', () => {
  const dag = new DAGBuilder()

  // 1. Linear A→B→C → 3 waves [[A],[B],[C]]
  it('linear chain A→B→C produces 3 sequential waves', () => {
    const steps = [step('A'), step('B', ['A']), step('C', ['B'])]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(3)
    expect(waveIds(waves)).toEqual([['A'], ['B'], ['C']])
  })

  // 2. Parallel A,B,C (no deps) → 1 wave
  it('steps with no dependencies are all in wave 0', () => {
    const steps = [step('A'), step('B'), step('C')]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(1)
    expect(waveIds(waves)[0]).toEqual(['A', 'B', 'C'])
  })

  // 3. Diamond A→(B,C)→D → 3 waves
  it('diamond A→(B,C)→D produces 3 waves', () => {
    const steps = [step('A'), step('B', ['A']), step('C', ['A']), step('D', ['B', 'C'])]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(3)
    expect(waveIds(waves)[0]).toEqual(['A'])
    expect(waveIds(waves)[1]).toEqual(['B', 'C'])
    expect(waveIds(waves)[2]).toEqual(['D'])
  })

  // 4. Mixed: A→B, C (no dep), B→D, C→D → 2 waves [[A,C],[B],[D]]
  it('mixed deps produce correct wave assignment', () => {
    const steps = [step('A'), step('B', ['A']), step('C'), step('D', ['B', 'C'])]
    const waves = dag.buildWaves(steps)
    // A and C have no deps → wave 0; B depends on A → wave 1; D depends on B,C → wave 2
    expect(waves).toHaveLength(3)
    expect(waveIds(waves)[0]).toEqual(['A', 'C'])
    expect(waveIds(waves)[1]).toEqual(['B'])
    expect(waveIds(waves)[2]).toEqual(['D'])
  })

  // 5. Single step → 1 wave
  it('single step produces 1 wave', () => {
    const waves = dag.buildWaves([step('solo')])
    expect(waves).toHaveLength(1)
    expect(waves[0][0].id).toBe('solo')
  })

  // 6. Cycle A→B→A → throws WorkflowCycleError
  it('cycle A→B→A throws WorkflowCycleError', () => {
    const steps = [step('A', ['B']), step('B', ['A'])]
    expect(() => dag.buildWaves(steps)).toThrow(WorkflowCycleError)
  })

  // 7. Self-cycle A→A → throws WorkflowCycleError
  it('self-cycle A→A throws WorkflowCycleError', () => {
    const steps = [step('A', ['A'])]
    expect(() => dag.buildWaves(steps)).toThrow(WorkflowCycleError)
  })

  // 8. Missing dep ref → throws error
  it('step referencing non-existent dep throws STEP_NOT_FOUND', () => {
    const steps = [step('A', ['ghost'])]
    expect(() => dag.buildWaves(steps)).toThrow(/STEP_NOT_FOUND/)
  })

  // 9. Empty steps → empty waves
  it('empty steps array returns empty waves', () => {
    const waves = dag.buildWaves([])
    expect(waves).toHaveLength(0)
  })

  // 10. 3-step cycle → throws with cycle array
  it('3-step cycle A→B→C→A throws WorkflowCycleError with cycle nodes', () => {
    const steps = [step('A', ['C']), step('B', ['A']), step('C', ['B'])]
    let thrown: unknown
    try {
      dag.buildWaves(steps)
    } catch (e) {
      thrown = e
    }
    expect(thrown).toBeInstanceOf(WorkflowCycleError)
    const err = thrown as WorkflowCycleError
    // All 3 nodes are in the cycle
    expect(err.cycle).toHaveLength(3)
    expect(err.cycle.sort()).toEqual(['A', 'B', 'C'])
  })

  // 11. Multiple independent subgraphs → correct wave assignment
  it('two independent chains run in parallel across waves', () => {
    // Chain 1: P→Q, Chain 2: X→Y
    const steps = [step('P'), step('Q', ['P']), step('X'), step('Y', ['X'])]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(2)
    expect(waveIds(waves)[0]).toEqual(['P', 'X'])
    expect(waveIds(waves)[1]).toEqual(['Q', 'Y'])
  })

  // 12. Complex diamond — all in correct wave
  it('double diamond produces correct 4 waves', () => {
    // A → B,C → D → E,F → G
    const steps = [
      step('A'),
      step('B', ['A']),
      step('C', ['A']),
      step('D', ['B', 'C']),
      step('E', ['D']),
      step('F', ['D']),
      step('G', ['E', 'F']),
    ]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(5)
    expect(waveIds(waves)[0]).toEqual(['A'])
    expect(waveIds(waves)[1]).toEqual(['B', 'C'])
    expect(waveIds(waves)[2]).toEqual(['D'])
    expect(waveIds(waves)[3]).toEqual(['E', 'F'])
    expect(waveIds(waves)[4]).toEqual(['G'])
  })

  // 13. Steps with no deps → first wave
  it('steps with empty dependsOn array go in first wave', () => {
    const steps = [step('X'), { ...step('Y'), dependsOn: [] }]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(1)
    expect(waveIds(waves)[0]).toEqual(['X', 'Y'])
  })

  // 14. Step with all deps in first wave → second wave
  it('step depending on all wave-0 steps goes into wave 1', () => {
    const steps = [step('A'), step('B'), step('C', ['A', 'B'])]
    const waves = dag.buildWaves(steps)
    expect(waves).toHaveLength(2)
    expect(waveIds(waves)[0]).toEqual(['A', 'B'])
    expect(waveIds(waves)[1]).toEqual(['C'])
  })

  // 15. buildWaves: does not mutate input steps
  it('buildWaves does not mutate input step objects', () => {
    const input = [step('A'), step('B', ['A'])]
    const origA = { ...input[0] }
    const origB = { ...input[1] }
    dag.buildWaves(input)
    expect(input[0]).toEqual(origA)
    expect(input[1]).toEqual(origB)
  })
})
