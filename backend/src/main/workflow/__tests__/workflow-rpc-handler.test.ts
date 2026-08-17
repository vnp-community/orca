/**
 * Tests for workflow-rpc-handler.ts — focused on 'workflow.template.update'
 * (the Category 3 gap from specs/frontend/tdd/api/gaps-and-mismatches.md:
 * frontend called workflow.template.update but the backend never registered
 * that method under workflow.template.*).
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS + real TemplateResolver, matching
 * the orca-project-sharing-rpc-handler.test.ts pattern (findMethod/fakeCtx).
 * WorkflowOrchestrator is stubbed — the method under test never calls it.
 *
 * @module main/workflow/__tests__/workflow-rpc-handler.test
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { createWorkflowMethods } from '../workflow-rpc-handler'
import { TemplateResolver } from '../TemplateResolver'
import type { WorkflowOrchestrator } from '../WorkflowOrchestrator'
import type { RpcMethod, RpcContext } from '../../runtime/rpc/core'
import type { WorkflowDefinition } from '../WorkflowTypes'

// ── helpers ────────────────────────────────────────────────────────────────

function findMethod(methods: RpcMethod[], name: string): RpcMethod {
  const method = methods.find((m) => m.name === name)
  if (!method) {throw new Error(`RPC method not found: ${name}`)}
  return method
}

/** Minimal fake RpcContext — handlers under test only touch ctx.userId. */
function fakeCtx(userId?: string): RpcContext {
  return { userId } as RpcContext
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

async function makeMethods(): Promise<{ pool: SqliteSingleConnectionPool; methods: RpcMethod[]; resolver: TemplateResolver }> {
  const pool = new SqliteSingleConnectionPool(':memory:')
  await pool.withConnection(async (db) => {
    const runner = new MigrationRunner(db, ALL_MIGRATIONS)
    await runner.migrate()
  })
  const resolver = new TemplateResolver(pool)
  // The update handler under test never touches the orchestrator — a stub is enough.
  const orchestrator = {} as WorkflowOrchestrator
  const methods = createWorkflowMethods(orchestrator, resolver, pool)
  return { pool, methods, resolver }
}

// ── tests ─────────────────────────────────────────────────────────────────────

describe('workflow.template.update RPC method', () => {
  let pool: SqliteSingleConnectionPool
  let methods: RpcMethod[]
  let resolver: TemplateResolver

  beforeEach(async () => {
    const setup = await makeMethods()
    pool = setup.pool
    methods = setup.methods
    resolver = setup.resolver
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  it('is registered under workflow.template.update', () => {
    expect(() => findMethod(methods, 'workflow.template.update')).not.toThrow()
  })

  it('happy path: owner updates name + definition, response echoes templateId', async () => {
    const id = await resolver.create({ name: 'Draft', definition: def(['A']), ownerId: 'u-1' })

    const method = findMethod(methods, 'workflow.template.update')
    const result = await method.handler(
      { templateId: id, name: 'Final', definition: def(['A', 'B']), traceId: 'trace-1' },
      fakeCtx('u-1')
    )

    expect(result).toEqual({ templateId: id, updated: true })
    const resolved = await resolver.resolve(id)
    expect(resolved.steps.map(s => s.id).sort()).toEqual(['A', 'B'])
  })

  it('rejects params missing templateId (Zod validation)', () => {
    const method = findMethod(methods, 'workflow.template.update')
    const parseResult = method.params?.safeParse({ name: 'No id here' })
    expect(parseResult?.success).toBe(false)
  })

  it('not-found: throws TEMPLATE_NOT_FOUND for an unknown templateId', async () => {
    const method = findMethod(methods, 'workflow.template.update')
    await expect(
      method.handler({ templateId: 'ghost-id', name: 'X' }, fakeCtx('u-1'))
    ).rejects.toThrow(/TEMPLATE_NOT_FOUND/)
  })

  it('authorization: a different user cannot update someone else\'s template', async () => {
    const id = await resolver.create({ name: 'Mine', definition: def(['A']), ownerId: 'u-1' })
    const method = findMethod(methods, 'workflow.template.update')

    await expect(
      method.handler({ templateId: id, name: 'Stolen' }, fakeCtx('u-2'))
    ).rejects.toThrow(/TEMPLATE_UPDATE_DENIED/)

    // Confirm no mutation happened
    const rows = await pool.withConnection((db) =>
      db.query<{ name: string }>(`SELECT name FROM orca_workflow_templates WHERE id = ?`, [id])
    )
    expect(rows[0]?.name).toBe('Mine')
  })

  it('missing ctx.userId falls back to "system" — a template owned by "system" is updatable without auth', async () => {
    const id = await resolver.create({ name: 'System Template', definition: def(['A']), ownerId: 'system' })
    const method = findMethod(methods, 'workflow.template.update')

    const result = await method.handler({ templateId: id, name: 'System Renamed' }, fakeCtx(undefined))
    expect(result).toEqual({ templateId: id, updated: true })
  })
})
