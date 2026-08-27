/**
 * Tests for ANNOTATION_METHODS (annotation.list / annotation.create).
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following
 * OrcaProjectSourceProjectService.test.ts / orca-project-sharing-rpc-handler.test.ts
 * pattern — real orca_annotations table (migration 0018), no mocking of the store.
 *
 * @module main/runtime/rpc/methods/__tests__/annotation.test
 */
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteSingleConnectionPool } from '../../../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../../../db/migrations'
import { initAnnotationStore, resetAnnotationStore } from '../../../../code-review/annotation-store'
import { ANNOTATION_METHODS } from '../annotation'
import type { RpcContext, RpcMethod } from '../../core'

function findMethod(name: string): RpcMethod {
  const method = ANNOTATION_METHODS.find((m) => m.name === name)
  if (!method) {throw new Error(`method not found: ${name}`)}
  return method
}

function fakeCtx(userId?: string): RpcContext {
  return { userId } as RpcContext
}

async function insertProject(pool: SqliteSingleConnectionPool, projectId: string): Promise<void> {
  const now = Date.now()
  await pool.withConnection((db) =>
    db.query(
      `INSERT INTO orca_v5_projects
         (id, name, dev_server_id, repo_path, default_branch, visibility, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [projectId, projectId, 'dev-server-001', '/irrelevant', 'main', 'team', 'u-A', now, now]
    )
  )
}

describe('annotation.list / annotation.create', () => {
  let pool: SqliteSingleConnectionPool

  beforeEach(async () => {
    pool = new SqliteSingleConnectionPool(':memory:')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })
    await insertProject(pool, 'proj-1')
    initAnnotationStore(pool)
  })

  afterEach(async () => {
    resetAnnotationStore()
    await pool.destroy().catch(() => {})
  })

  it('annotation.list returns [] when no comments exist yet for a line', async () => {
    const method = findMethod('annotation.list')
    const result = await method.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 10 },
      fakeCtx('alice@example.com')
    )
    expect(result).toEqual([])
  })

  it('annotation.create persists a comment and annotation.list returns it back', async () => {
    const createMethod = findMethod('annotation.create')
    const created = (await createMethod.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 10, content: 'nice catch' },
      fakeCtx('alice@example.com')
    )) as { id: string; author: string; authorInitials: string; content: string }

    expect(created.content).toBe('nice catch')
    expect(created.author).toBe('alice')
    expect(created.authorInitials).toBe('AL')
    expect(created.id).toBeTruthy()

    const listMethod = findMethod('annotation.list')
    const list = (await listMethod.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 10 },
      fakeCtx('alice@example.com')
    )) as unknown[]

    expect(list).toHaveLength(1)
    expect(list[0]).toMatchObject({ id: created.id, content: 'nice catch', lineNumber: 10, filePath: 'src/foo.ts' })
  })

  it('annotation.list scopes strictly by project/file/line — a different line returns nothing', async () => {
    const createMethod = findMethod('annotation.create')
    await createMethod.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 10, content: 'comment A' },
      fakeCtx('alice@example.com')
    )

    const listMethod = findMethod('annotation.list')
    const otherLine = await listMethod.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 11 },
      fakeCtx('alice@example.com')
    )
    expect(otherLine).toEqual([])
  })

  it('annotation.create falls back to "anonymous" author when ctx.userId is absent', async () => {
    const createMethod = findMethod('annotation.create')
    const created = (await createMethod.handler(
      { projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 1, content: 'hi' },
      fakeCtx(undefined)
    )) as { author: string }
    expect(created.author).toBe('anonymous')
  })

  it('annotation.create rejects an empty comment body', () => {
    const method = findMethod('annotation.create')
    expect(() =>
      method.params?.parse({ projectId: 'proj-1', filePath: 'src/foo.ts', lineNumber: 1, content: '' })
    ).toThrow()
  })

  it('annotation.list rejects a missing filePath', () => {
    const method = findMethod('annotation.list')
    expect(() => method.params?.parse({ projectId: 'proj-1', lineNumber: 1 })).toThrow()
  })
})
