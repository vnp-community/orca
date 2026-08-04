/**
 * Tests for TemplateResolver (TDD-17) — TASK-033
 *
 * Uses in-memory SQLite via SqliteSingleConnectionPool + ALL_MIGRATIONS.
 * ≥ 8 tests.
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

  // 1. resolve: single template no parent
  it('resolve: single template with no parent returns its definition', async () => {
    const id = await resolver.create({
      name: 'Solo',
      definition: def(['A', 'B']),
      ownerId: 'u-1',
    })
    const resolved = await resolver.resolve(id)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['A', 'B'])
  })

  // 2. resolve: parent template merged
  it('resolve: child merges with parent (leaf adds steps)', async () => {
    const parentId = await resolver.create({
      name: 'Parent',
      definition: def(['Base1', 'Base2']),
      ownerId: 'u-1',
    })
    const childId = await resolver.create({
      name: 'Child',
      definition: def(['ChildStep']),
      ownerId: 'u-1',
      parentTemplateId: parentId,
    })
    const resolved = await resolver.resolve(childId)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['Base1', 'Base2', 'ChildStep'])
  })

  // 3. resolve: grandparent chain (depth 3)
  it('resolve: 3-level inheritance chain merges all steps', async () => {
    const grandId = await resolver.create({
      name: 'Grand',
      definition: def(['G1']),
      ownerId: 'u-1',
    })
    const parentId = await resolver.create({
      name: 'Parent',
      definition: def(['P1']),
      ownerId: 'u-1',
      parentTemplateId: grandId,
    })
    const childId = await resolver.create({
      name: 'Child',
      definition: def(['C1']),
      ownerId: 'u-1',
      parentTemplateId: parentId,
    })
    const resolved = await resolver.resolve(childId)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['C1', 'G1', 'P1'])
  })

  // 4. resolve: MAX_INHERIT_DEPTH=5 enforced
  it('resolve: throws when inheritance depth exceeds MAX_INHERIT_DEPTH (5)', async () => {
    // Create chain of 7 templates (depth 7 > MAX_INHERIT_DEPTH=5)
    let currentId: string | undefined
    for (let i = 0; i <= 6; i++) {
      currentId = await resolver.create({
        name: `Level${i}`,
        definition: def([`step${i}`]),
        ownerId: 'u-1',
        parentTemplateId: currentId,
      })
    }
    await expect(resolver.resolve(currentId!)).rejects.toThrow(/TEMPLATE_INHERIT_DEPTH_EXCEEDED/)
  })

  // 5. resolve: leaf overrides parent steps with same id
  it('resolve: leaf step overrides parent step with same id', async () => {
    const parentId = await resolver.create({
      name: 'Parent',
      definition: {
        steps: [
          {
            id: 'shared',
            name: 'Parent Version',
            serverSpec: 'server:srv-1',
            config: { type: 'shell', script: 'echo parent' },
          },
        ],
      },
      ownerId: 'u-1',
    })
    const childId = await resolver.create({
      name: 'Child',
      definition: {
        steps: [
          {
            id: 'shared',
            name: 'Child Version',
            serverSpec: 'server:srv-1',
            config: { type: 'shell', script: 'echo child' },
          },
        ],
      },
      ownerId: 'u-1',
      parentTemplateId: parentId,
    })
    const resolved = await resolver.resolve(childId)
    expect(resolved.steps).toHaveLength(1)
    expect(resolved.steps[0].name).toBe('Child Version')
  })

  // 6. create: stores template in DB
  it('create: returns a valid UUID template ID', async () => {
    const id = await resolver.create({
      name: 'My Template',
      definition: def(['A']),
      ownerId: 'u-2',
    })
    expect(id).toMatch(/^[0-9a-f-]{36}$/)
  })

  // 7. list: returns user templates filtered by scope
  it('list: returns templates for given scope', async () => {
    await resolver.create({ name: 'T1', definition: def(['A']), ownerId: 'u-1', scope: 'user' })
    await resolver.create({ name: 'T2', definition: def(['B']), ownerId: 'u-2', scope: 'user' })
    await resolver.create({ name: 'T3', definition: def(['C']), ownerId: 'u-1', scope: 'company' })

    const userTemplates = await resolver.list('user')
    expect(userTemplates.length).toBe(2)
    expect(userTemplates.every(t => t.scope === 'user')).toBe(true)
  })

  // 8. list: returns company-scope templates
  it('list: company-scope templates returned separately from user-scope', async () => {
    await resolver.create({ name: 'U1', definition: def(['A']), ownerId: 'u-1', scope: 'user' })
    await resolver.create({ name: 'C1', definition: def(['B']), ownerId: 'org-1', scope: 'company' })
    await resolver.create({ name: 'C2', definition: def(['C']), ownerId: 'org-1', scope: 'company' })

    const company = await resolver.list('company')
    expect(company.length).toBe(2)
    expect(company.every(t => t.scope === 'company')).toBe(true)
  })

  // 9. resolve: throws TEMPLATE_NOT_FOUND for unknown id
  it('resolve: throws TEMPLATE_NOT_FOUND for unknown templateId', async () => {
    await expect(resolver.resolve('no-such-id')).rejects.toThrow(/TEMPLATE_NOT_FOUND/)
  })

  // 10. list: filtered by ownerId
  it('list: filtering by ownerId returns only that owner\'s templates', async () => {
    await resolver.create({ name: 'A', definition: def(['x']), ownerId: 'u-1', scope: 'user' })
    await resolver.create({ name: 'B', definition: def(['y']), ownerId: 'u-2', scope: 'user' })

    const u1Templates = await resolver.list('user', 'u-1')
    expect(u1Templates.length).toBe(1)
    expect(u1Templates[0].name).toBe('A')
  })
})

// ── CR-TRACE-017 tracing (BL-WF-01) ─────────────────────────────────────────

describe('TemplateResolver — CR-TRACE-017 tracing (BL-WF-01)', () => {
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

  it('create() without parentTemplateId → span field hasParent: false', async () => {
    const { events, stop } = captureTraceEvents()
    const id = await resolver.create({ name: 'NoParent', definition: def(['A']), ownerId: 'u-1' })
    stop()

    const startEvent = events.find((e) => e.flow === 'workflow:templateCreate' && e.level === 'start')
    expect(startEvent?.fields).toMatchObject({ name: 'NoParent', hasParent: false })
    const okEvent = events.find((e) => e.flow === 'workflow:templateCreate' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ templateId: id })
  })

  it('create() with parentTemplateId → span field hasParent: true', async () => {
    const parentId = await resolver.create({ name: 'Parent', definition: def(['A']), ownerId: 'u-1' })

    const { events, stop } = captureTraceEvents()
    await resolver.create({
      name: 'Child',
      definition: def(['B']),
      ownerId: 'u-1',
      parentTemplateId: parentId,
    })
    stop()

    const startEvent = events.find((e) => e.flow === 'workflow:templateCreate' && e.level === 'start')
    expect(startEvent?.fields).toMatchObject({ name: 'Child', hasParent: true })
  })

  it('resolve() does not emit any workflow:* tracer event — read-path, no tracer per BL-WF-01 design', async () => {
    const id = await resolver.create({ name: 'X', definition: def(['A']), ownerId: 'u-1' })

    const { events, stop } = captureTraceEvents()
    await resolver.resolve(id)
    stop()

    expect(events.filter((e) => e.flow.startsWith('workflow:'))).toHaveLength(0)
  })
})
