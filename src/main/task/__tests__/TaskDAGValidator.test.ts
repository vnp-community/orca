/**
 * Tests for TaskDAGValidator (TDD-18) — TASK-036
 *
 * Uses in-memory SQLite via SqliteSingleConnectionPool + ALL_MIGRATIONS.
 * Tests: detectCycle (BFS), wouldCreateCycle (DFS, edge-type aware), getReachable.
 *
 * @module main/task/__tests__/TaskDAGValidator.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TaskDAGValidator } from '../TaskDAGValidator'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'

// ── helpers ────────────────────────────────────────────────────────────────────

async function makeValidator(): Promise<{
  pool: SqliteSingleConnectionPool
  validator: TaskDAGValidator
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const validator = new TaskDAGValidator(pool)
  return { pool, validator }
}

/** Insert a task into orca_tasks (minimal required fields) */
async function insertTask(pool: SqliteSingleConnectionPool, id: string): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_tasks (id, title, type, status, priority, labels, visibility, progress_percent, created_at, updated_at)
       VALUES (?, ?, 'task', 'backlog', 'medium', '[]', 'team', 0, ?, ?)`,
      [id, `Task ${id}`, now, now]
    )
  )
}

/** Insert an edge between two tasks */
async function insertEdge(
  pool: SqliteSingleConnectionPool,
  from: string,
  to: string,
  edgeType = 'depends_on'
): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_task_edges (from_task_id, to_task_id, edge_type, created_at) VALUES (?, ?, ?, ?)`,
      [from, to, edgeType, Date.now()]
    )
  )
}

// ── tests ──────────────────────────────────────────────────────────────────────

describe('TaskDAGValidator', () => {
  let pool: SqliteSingleConnectionPool
  let validator: TaskDAGValidator

  beforeEach(async () => {
    const setup = await makeValidator()
    pool = setup.pool
    validator = setup.validator

    // Seed tasks: A → B → C (linear chain)
    await insertTask(pool, 'A')
    await insertTask(pool, 'B')
    await insertTask(pool, 'C')
    await insertTask(pool, 'D')
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  // ── detectCycle ─────────────────────────────────────────────────────────────

  it('detectCycle: self-loop always returns true', async () => {
    expect(await validator.detectCycle('A', 'A')).toBe(true)
  })

  it('detectCycle: empty graph returns false (no cycle)', async () => {
    expect(await validator.detectCycle('A', 'B')).toBe(false)
  })

  it('detectCycle: returns false for valid forward edge A→B in linear chain', async () => {
    await insertEdge(pool, 'A', 'B')
    await insertEdge(pool, 'B', 'C')
    // Adding C→D is safe
    expect(await validator.detectCycle('C', 'D')).toBe(false)
  })

  it('detectCycle: returns true for back-edge creating 2-cycle (A→B, add B→A)', async () => {
    await insertEdge(pool, 'A', 'B')
    expect(await validator.detectCycle('B', 'A')).toBe(true)
  })

  it('detectCycle: returns true for 3-cycle (A→B, B→C, add C→A)', async () => {
    await insertEdge(pool, 'A', 'B')
    await insertEdge(pool, 'B', 'C')
    expect(await validator.detectCycle('C', 'A')).toBe(true)
  })

  it('detectCycle: disconnected subgraph not affected — returns false', async () => {
    await insertEdge(pool, 'A', 'B')
    // C and D are isolated
    expect(await validator.detectCycle('C', 'D')).toBe(false)
    expect(await validator.detectCycle('D', 'C')).toBe(false)
  })

  // ── wouldCreateCycle ────────────────────────────────────────────────────────

  it('wouldCreateCycle: self-loop returns true', async () => {
    expect(await validator.wouldCreateCycle('A', 'A', 'depends_on')).toBe(true)
  })

  it('wouldCreateCycle: safe new edge in empty graph returns false', async () => {
    expect(await validator.wouldCreateCycle('A', 'B', 'depends_on')).toBe(false)
  })

  it('wouldCreateCycle: detects 2-node cycle within same edge type', async () => {
    await insertEdge(pool, 'A', 'B', 'depends_on')
    expect(await validator.wouldCreateCycle('B', 'A', 'depends_on')).toBe(true)
  })

  it('wouldCreateCycle: edges of different type do NOT cause cycle detection', async () => {
    // A→B exists as 'depends_on'; B→A as 'blocks' should NOT be caught by 'blocks' cycle check
    // because there is no existing 'blocks' edge
    await insertEdge(pool, 'A', 'B', 'depends_on')
    expect(await validator.wouldCreateCycle('B', 'A', 'blocks')).toBe(false)
  })

  it('wouldCreateCycle: transitive 3-node cycle detected correctly', async () => {
    await insertEdge(pool, 'A', 'B', 'depends_on')
    await insertEdge(pool, 'B', 'C', 'depends_on')
    expect(await validator.wouldCreateCycle('C', 'A', 'depends_on')).toBe(true)
  })

  // ── getReachable ────────────────────────────────────────────────────────────

  it('getReachable: returns empty array when no outbound edges', async () => {
    const result = await validator.getReachable('A')
    expect(result).toEqual([])
  })

  it('getReachable: returns direct successor', async () => {
    await insertEdge(pool, 'A', 'B')
    const result = await validator.getReachable('A')
    expect(result).toContain('B')
    expect(result).not.toContain('A')
  })

  it('getReachable: returns all nodes in transitive closure (A→B→C)', async () => {
    await insertEdge(pool, 'A', 'B')
    await insertEdge(pool, 'B', 'C')
    const result = await validator.getReachable('A')
    expect(result.sort()).toEqual(['B', 'C'])
  })

  it('getReachable: handles diamond topology without duplicates (A→B,A→C,B→D,C→D)', async () => {
    await insertTask(pool, 'E') // ensure D exists
    await insertEdge(pool, 'A', 'B')
    await insertEdge(pool, 'A', 'C')
    await insertEdge(pool, 'B', 'D')
    await insertEdge(pool, 'C', 'D')
    const result = await validator.getReachable('A')
    expect(new Set(result).size).toBe(result.length) // no duplicates
    expect(result.sort()).toEqual(['B', 'C', 'D'])
  })

  it('getReachable: isolated node D not in reachable set from A', async () => {
    await insertEdge(pool, 'A', 'B')
    const result = await validator.getReachable('A')
    expect(result).not.toContain('D')
  })
})
