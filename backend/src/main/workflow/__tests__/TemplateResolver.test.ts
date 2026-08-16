/**
 * Tests for TemplateResolver (TDD-17) — covers create/list/resolve baseline
 * plus update() (Category 3 gap fix: workflow.template.update was missing
 * entirely — see specs/frontend/tdd/api/gaps-and-mismatches.md §Category 3).
 *
 * Uses in-memory SQLite via SqliteSingleConnectionPool + ALL_MIGRATIONS.
 *
 * @module main/workflow/__tests__/TemplateResolver.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TemplateResolver } from '../TemplateResolver'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'
import type { WorkflowDefinition } from '../WorkflowTypes'

// ── helpers ──────────────────────────────────────────────────────────────────

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

function def(stepIds: string[]): WorkflowDefinition {
  return {
    steps: stepIds.map(id => ({
      id,
      name: `Step ${id}`,
      serverSpec: 'server:srv-1',
      config: { type: 'shell' as const, script: `echo ${id}` },
    })),
  }
}

async function makeResolver(): Promise<{
  pool: SqliteSingleConnectionPool
  resolver: TemplateResolver
}> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const resolver = new TemplateResolver(pool)
  return { pool, resolver }
}

async function readRow(
  pool: SqliteSingleConnectionPool,
  id: string
): Promise<{ name: string; version: number; scope: string; ownerId: string; definitionJson: string } | undefined> {
  const rows = await pool.withConnection((db) =>
    db.query<{ name: string; version: number; scope: string; ownerId: string; definitionJson: string }>(
      `SELECT name, version, scope, owner_id as "ownerId", definition_json as "definitionJson"
       FROM orca_workflow_templates WHERE id = ?`,
      [id]
    )
  )
  return rows[0]
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe('TemplateResolver', () => {
  let pool: SqliteSingleConnectionPool
  let resolver: TemplateResolver

  beforeEach(async () => {
    const setup = await makeResolver()
    pool = setup.pool
    resolver = setup.resolver
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  it('create: returns a valid UUID template ID', async () => {
    const id = await resolver.create({ name: 'My Template', definition: def(['A']), ownerId: 'u-2' })
    expect(id).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('resolve: single template with no parent returns its definition', async () => {
    const id = await resolver.create({ name: 'Solo', definition: def(['A', 'B']), ownerId: 'u-1' })
    const resolved = await resolver.resolve(id)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['A', 'B'])
  })

  it('list: filtering by ownerId returns only that owner\'s templates', async () => {
    await resolver.create({ name: 'A', definition: def(['x']), ownerId: 'u-1', scope: 'user' })
    await resolver.create({ name: 'B', definition: def(['y']), ownerId: 'u-2', scope: 'user' })
    const u1Templates = await resolver.list('user', 'u-1')
    expect(u1Templates.length).toBe(1)
    expect(u1Templates[0].name).toBe('A')
  })

  // ── update() — BUG-FE-RPC-006 fix ─────────────────────────────────────────

  it('update: happy path — changes name, bumps version, refreshes updated_at', async () => {
    const id = await resolver.create({ name: 'Original', definition: def(['A']), ownerId: 'u-1' })
    const before = await readRow(pool, id)
    expect(before?.version).toBe(1)

    await resolver.update({ templateId: id, ownerId: 'u-1', name: 'Renamed' })

    const after = await readRow(pool, id)
    expect(after?.name).toBe('Renamed')
    expect(after?.version).toBe(2)
  })

  it('update: definition-only update replaces definition_json and preserves name', async () => {
    const id = await resolver.create({ name: 'Keep Name', definition: def(['A']), ownerId: 'u-1' })

    await resolver.update({ templateId: id, ownerId: 'u-1', definition: def(['A', 'B', 'C']) })

    const resolved = await resolver.resolve(id)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['A', 'B', 'C'])
    const row = await readRow(pool, id)
    expect(row?.name).toBe('Keep Name')
  })

  it('update: repeated calls keep incrementing version (optimistic-concurrency counter)', async () => {
    const id = await resolver.create({ name: 'V', definition: def(['A']), ownerId: 'u-1' })
    await resolver.update({ templateId: id, ownerId: 'u-1', name: 'V2' })
    await resolver.update({ templateId: id, ownerId: 'u-1', name: 'V3' })
    const row = await readRow(pool, id)
    expect(row?.version).toBe(3)
  })

  it('update: throws TEMPLATE_NOT_FOUND for an unknown templateId', async () => {
    await expect(
      resolver.update({ templateId: 'no-such-id', ownerId: 'u-1', name: 'X' })
    ).rejects.toThrow(/TEMPLATE_NOT_FOUND/)
  })

  it('update: throws TEMPLATE_UPDATE_DENIED when ownerId does not match the template owner', async () => {
    const id = await resolver.create({ name: 'Owned by u-1', definition: def(['A']), ownerId: 'u-1' })
    await expect(
      resolver.update({ templateId: id, ownerId: 'u-2', name: 'Hijacked' })
    ).rejects.toThrow(/TEMPLATE_UPDATE_DENIED/)

    // Row must be untouched by the denied attempt
    const row = await readRow(pool, id)
    expect(row?.name).toBe('Owned by u-1')
    expect(row?.version).toBe(1)
  })

  it('update: emits workflow:templateUpdate trace events (start + ok)', async () => {
    const id = await resolver.create({ name: 'Traced', definition: def(['A']), ownerId: 'u-1' })

    const { events, stop } = captureTraceEvents()
    await resolver.update({ templateId: id, ownerId: 'u-1', name: 'Traced2' })
    stop()

    const startEvent = events.find(e => e.flow === 'workflow:templateUpdate' && e.level === 'start')
    expect(startEvent?.fields).toMatchObject({ templateId: id })
    const okEvent = events.find(e => e.flow === 'workflow:templateUpdate' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ templateId: id })
  })

  it('update: emits a fail trace event when denied', async () => {
    const id = await resolver.create({ name: 'X', definition: def(['A']), ownerId: 'u-1' })

    const { events, stop } = captureTraceEvents()
    await resolver.update({ templateId: id, ownerId: 'u-2', name: 'Y' }).catch(() => {})
    stop()

    const failEvent = events.find(e => e.flow === 'workflow:templateUpdate' && e.level === 'fail')
    expect(failEvent).toBeDefined()
  })
})
