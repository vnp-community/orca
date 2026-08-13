/**
 * Tests for orca-project-sharing-rpc-handler
 *
 * SECURITY-CRITICAL: covers BOTH directions —
 * - positive: a legitimate member sees exactly the shared Project, nothing more
 * - negative: every cross-user leak / spoofing / privilege-escalation path is blocked
 *
 * Uses in-memory SQLite + ALL_MIGRATIONS, following ProjectService.test.ts /
 * TeamService.test.ts pattern. `getProjectData`'s filesystem read is replaced
 * with an in-memory fixture map via the injectable fileDeps param — no real
 * orca-data.json is touched.
 *
 * @module main/project/__tests__/orca-project-sharing-rpc-handler.test
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { SqliteSingleConnectionPool } from '../../db/sqlite/sqlite-pool'
import { MigrationRunner } from '../../db/migrations/runner'
import { ALL_MIGRATIONS } from '../../db/migrations'
import { ProjectService } from '../ProjectService'
import { OrcaProjectSourceProjectService } from '../OrcaProjectSourceProjectService'
import {
  createOrcaProjectSharingMethods,
  filterOwnerProjectData,
  type OrcaProjectSharedProjectData
} from '../orca-project-sharing-rpc-handler'
import type { DevServerManager } from '../../dev-server/dev-server-manager'
import type { RpcMethod, RpcContext } from '../../runtime/rpc/core'
import type { PersistedState, Project, Repo } from '../../../shared/types'

// ── helpers ────────────────────────────────────────────────────────────────

const FAKE_DEV_SERVER_ID = 'dev-server-001'

function makeMockDSM(): DevServerManager {
  return {
    get: vi.fn().mockReturnValue({
      id: FAKE_DEV_SERVER_ID,
      name: 'Test Server',
      connectionType: 'direct-websocket',
      status: 'connected'
    })
  } as unknown as DevServerManager
}

async function insertUser(pool: SqliteSingleConnectionPool, userId: string): Promise<void> {
  await pool.withConnection((db) =>
    db.query(
      'INSERT INTO orca_users (id, email, name, role, provider, created_at) VALUES (?, ?, ?, ?, ?, ?)',
      [userId, `${userId}@test.com`, userId, 'developer', 'none', Date.now()]
    )
  )
}

function findMethod(methods: RpcMethod[], name: string): RpcMethod {
  const method = methods.find((m) => m.name === name)
  if (!method) {throw new Error(`RPC method not found: ${name}`)}
  return method
}

/** Minimal fake RpcContext — handlers under test only touch ctx.userId. */
function fakeCtx(userId?: string): RpcContext {
  return { userId } as RpcContext
}

function makeRepo(id: string, displayName: string): Repo {
  return {
    id,
    path: `/repos/${id}`,
    displayName,
    badgeColor: '#123456',
    addedAt: Date.now()
  } as Repo
}

function makeProject(id: string, displayName: string, sourceRepoIds: string[]): Project {
  return {
    id,
    displayName,
    badgeColor: '#654321',
    sourceRepoIds,
    createdAt: Date.now(),
    updatedAt: Date.now()
  } as Project
}

/** Minimal PersistedState fixture — only the fields getProjectData actually reads. */
function makePersistedState(projects: Project[], repos: Repo[]): PersistedState {
  return {
    schemaVersion: 1,
    repos,
    projects,
    worktreeMeta: {
      // one worktree per repo, keyed `${repoId}::${path}` (WORKTREE_ID_SEPARATOR)
      ...Object.fromEntries(
        repos.map((r) => [
          `${r.id}::/wt/${r.id}`,
          { displayName: `wt-${r.id}`, comment: '', linkedIssue: null, linkedPR: null, linkedLinearIssue: null }
        ])
      )
    }
  } as unknown as PersistedState
}

describe('orca-project-sharing-rpc-handler', () => {
  let pool: SqliteSingleConnectionPool
  let projectService: ProjectService
  let sourceProjectService: OrcaProjectSourceProjectService
  let methods: RpcMethod[]

  // In-memory stand-in for per-user orca-data.json files, keyed by ownerUserId.
  let dataByOwner: Map<string, PersistedState>

  const getUserRole = async (userId: string) => (userId === 'admin-1' ? 'admin' : 'developer')

  beforeEach(async () => {
    pool = new SqliteSingleConnectionPool(':memory:')
    await pool.withConnection(async (db) => {
      const runner = new MigrationRunner(db, ALL_MIGRATIONS)
      await runner.migrate()
    })
    projectService = new ProjectService(pool, makeMockDSM())
    sourceProjectService = new OrcaProjectSourceProjectService(pool)
    dataByOwner = new Map()

    methods = createOrcaProjectSharingMethods(sourceProjectService, projectService, getUserRole, {
      // Tests never touch the real filesystem — "path" is just ownerUserId.
      resolveOwnerDataFile: (ownerUserId) => ownerUserId,
      readOwnerPersistedState: (path) => {
        const state = dataByOwner.get(path)
        if (!state) {throw new Error('OWNER_DATA_NOT_FOUND')}
        return state
      }
    })

    await insertUser(pool, 'u-A')
    await insertUser(pool, 'u-B')
    await insertUser(pool, 'u-C')
    await insertUser(pool, 'admin-1')
  })

  afterEach(async () => {
    await pool.destroy().catch(() => {})
  })

  /** A creates OrcaProject "Team Backend" (A becomes owner automatically). */
  async function makeOrcaProject(): Promise<string> {
    const orcaProject = await projectService.create({
      name: 'Team Backend',
      devServerId: FAKE_DEV_SERVER_ID,
      repoPath: '/irrelevant',
      createdBy: 'u-A'
    })
    return orcaProject.id
  }

  // ── filterOwnerProjectData (unit-level, exercised again through the RPC below) ──

  it('filterOwnerProjectData returns only the matching project + its repos', () => {
    const repoP = makeRepo('repo-P1', 'P1')
    const repoQ = makeRepo('repo-Q1', 'Q1')
    const projP = makeProject('proj-P', 'Project P', ['repo-P1'])
    const projQ = makeProject('proj-Q', 'Project Q (secret)', ['repo-Q1'])
    const state = makePersistedState([projP, projQ], [repoP, repoQ])

    const result = filterOwnerProjectData(state, 'proj-P')
    expect(result.project.id).toBe('proj-P')
    expect(result.repos.map((r) => r.id)).toEqual(['repo-P1'])
    expect(Object.keys(result.worktreeMeta)).toEqual(['repo-P1::/wt/repo-P1'])
  })

  // ── SECURITY POSITIVE CASE ───────────────────────────────────────────────────
  // B is a member; A shared Project P into the OrcaProject. B must see exactly
  // P — never Project Q, which A did NOT share.

  it('POSITIVE: a member sees exactly the shared Project, never another project owned by A', async () => {
    const orcaProjectId = await makeOrcaProject()
    await projectService.addMember(orcaProjectId, 'u-B', 'member')

    const repoP = makeRepo('repo-P1', 'P1')
    const repoQ = makeRepo('repo-Q1', 'Q1')
    const projP = makeProject('proj-P', 'Project P', ['repo-P1'])
    const projQ = makeProject('proj-Q', 'Project Q (secret, NOT shared)', ['repo-Q1'])
    dataByOwner.set('u-A', makePersistedState([projP, projQ], [repoP, repoQ]))

    // A links only P, not Q.
    const linkMethod = findMethod(methods, 'orcaProjects.linkSourceProject')
    await linkMethod.handler({ orcaProjectId, projectId: 'proj-P' }, fakeCtx('u-A'))

    const getDataMethod = findMethod(methods, 'orcaProjects.getProjectData')
    const result = (await getDataMethod.handler(
      { orcaProjectId, projectId: 'proj-P' },
      fakeCtx('u-B')
    )) as OrcaProjectSharedProjectData

    expect(result.project.id).toBe('proj-P')
    expect(result.repos.map((r) => r.id)).toEqual(['repo-P1'])
    // The full response must never mention Project Q or its repo anywhere.
    const serialized = JSON.stringify(result)
    expect(serialized).not.toContain('proj-Q')
    expect(serialized).not.toContain('repo-Q1')
  })

  // ── SECURITY NEGATIVE CASES ──────────────────────────────────────────────────

  it('SECURITY: a non-member is rejected with FORBIDDEN (does not leak project existence)', async () => {
    const orcaProjectId = await makeOrcaProject()
    dataByOwner.set('u-A', makePersistedState(
      [makeProject('proj-P', 'Project P', ['repo-P1'])],
      [makeRepo('repo-P1', 'P1')]
    ))
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const getDataMethod = findMethod(methods, 'orcaProjects.getProjectData')
    // u-C was never added as a member of orcaProjectId.
    await expect(
      getDataMethod.handler({ orcaProjectId, projectId: 'proj-P' }, fakeCtx('u-C'))
    ).rejects.toThrow(/FORBIDDEN/)
  })

  it('SECURITY: a member passing a projectId not linked to this OrcaProject is rejected with FORBIDDEN', async () => {
    const orcaProjectId = await makeOrcaProject()
    await projectService.addMember(orcaProjectId, 'u-B', 'member')

    // proj-Q exists in A's data and A even owns it, but never linked it to orcaProjectId.
    dataByOwner.set('u-A', makePersistedState(
      [
        makeProject('proj-P', 'Project P', ['repo-P1']),
        makeProject('proj-Q', 'Project Q (secret, NOT shared)', ['repo-Q1'])
      ],
      [makeRepo('repo-P1', 'P1'), makeRepo('repo-Q1', 'Q1')]
    ))
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const getDataMethod = findMethod(methods, 'orcaProjects.getProjectData')
    await expect(
      getDataMethod.handler({ orcaProjectId, projectId: 'proj-Q' }, fakeCtx('u-B'))
    ).rejects.toThrow(/FORBIDDEN/)
  })

  it('SECURITY: "no such OrcaProject" and "not a member" produce the identical error message', async () => {
    const orcaProjectId = await makeOrcaProject()
    await projectService.addMember(orcaProjectId, 'u-B', 'member')
    dataByOwner.set('u-A', makePersistedState(
      [makeProject('proj-P', 'Project P', ['repo-P1'])],
      [makeRepo('repo-P1', 'P1')]
    ))
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const getDataMethod = findMethod(methods, 'orcaProjects.getProjectData')

    let messageForNonMember = ''
    try {
      await getDataMethod.handler({ orcaProjectId, projectId: 'proj-P' }, fakeCtx('u-C'))
    } catch (err) {
      messageForNonMember = (err as Error).message
    }

    let messageForUnknownOrcaProject = ''
    try {
      await getDataMethod.handler(
        { orcaProjectId: 'orca-does-not-exist', projectId: 'proj-P' },
        fakeCtx('u-C')
      )
    } catch (err) {
      messageForUnknownOrcaProject = (err as Error).message
    }

    expect(messageForNonMember).toMatch(/FORBIDDEN/)
    expect(messageForNonMember).toBe(messageForUnknownOrcaProject)
  })

  it('SECURITY: linkSourceProject rejects a caller who is not a member of orcaProjectId', async () => {
    const orcaProjectId = await makeOrcaProject()
    const linkMethod = findMethod(methods, 'orcaProjects.linkSourceProject')
    await expect(
      linkMethod.handler({ orcaProjectId, projectId: 'proj-P' }, fakeCtx('u-C'))
    ).rejects.toThrow(/PROJECT_ACCESS_DENIED/)
    expect(await sourceProjectService.listSourceProjects(orcaProjectId)).toEqual([])
  })

  it('SECURITY: linkSourceProject always uses ctx.userId as ownerUserId — client-supplied ownerUserId is ignored', async () => {
    const orcaProjectId = await makeOrcaProject()
    const linkMethod = findMethod(methods, 'orcaProjects.linkSourceProject')

    // u-A (a real member/owner) attempts to smuggle an ownerUserId claiming u-B owns it.
    await linkMethod.handler(
      { orcaProjectId, projectId: 'proj-P', ownerUserId: 'u-B' } as unknown as {
        orcaProjectId: string
        projectId: string
      },
      fakeCtx('u-A')
    )

    const sources = await sourceProjectService.listSourceProjects(orcaProjectId)
    expect(sources).toEqual([{ ownerUserId: 'u-A', projectId: 'proj-P' }])
  })

  it('SECURITY: unlinkSourceProject is rejected for a non-owner member (FORBIDDEN)', async () => {
    const orcaProjectId = await makeOrcaProject()
    await projectService.addMember(orcaProjectId, 'u-B', 'member')
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const unlinkMethod = findMethod(methods, 'orcaProjects.unlinkSourceProject')
    await expect(
      unlinkMethod.handler({ orcaProjectId, projectId: 'proj-P' }, fakeCtx('u-B'))
    ).rejects.toThrow(/FORBIDDEN/)
    // Link must survive the rejected attempt.
    expect(await sourceProjectService.listSourceProjects(orcaProjectId)).toEqual([
      { ownerUserId: 'u-A', projectId: 'proj-P' }
    ])
  })

  it('unlinkSourceProject succeeds for the OrcaProject owner', async () => {
    const orcaProjectId = await makeOrcaProject() // u-A is owner
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const unlinkMethod = findMethod(methods, 'orcaProjects.unlinkSourceProject')
    const result = (await unlinkMethod.handler(
      { orcaProjectId, projectId: 'proj-P' },
      fakeCtx('u-A')
    )) as { success: boolean }

    expect(result.success).toBe(true)
    expect(await sourceProjectService.listSourceProjects(orcaProjectId)).toEqual([])
  })

  it('unlinkSourceProject succeeds for a global admin who is not the OrcaProject owner', async () => {
    const orcaProjectId = await makeOrcaProject()
    await projectService.addMember(orcaProjectId, 'admin-1', 'member')
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    const unlinkMethod = findMethod(methods, 'orcaProjects.unlinkSourceProject')
    const result = (await unlinkMethod.handler(
      { orcaProjectId, projectId: 'proj-P' },
      fakeCtx('admin-1')
    )) as { success: boolean }

    expect(result.success).toBe(true)
  })

  // ── UNAUTHENTICATED ───────────────────────────────────────────────────────────

  it('all methods reject an unauthenticated caller', async () => {
    const orcaProjectId = await makeOrcaProject()
    await expect(
      findMethod(methods, 'orcaProjects.linkSourceProject').handler(
        { orcaProjectId, projectId: 'proj-P' },
        fakeCtx()
      )
    ).rejects.toThrow(/UNAUTHENTICATED/)
    await expect(
      findMethod(methods, 'orcaProjects.unlinkSourceProject').handler(
        { orcaProjectId, projectId: 'proj-P' },
        fakeCtx()
      )
    ).rejects.toThrow(/UNAUTHENTICATED/)
    await expect(
      findMethod(methods, 'orcaProjects.getProjectData').handler(
        { orcaProjectId, projectId: 'proj-P' },
        fakeCtx()
      )
    ).rejects.toThrow(/UNAUTHENTICATED/)
    await expect(
      findMethod(methods, 'orcaProjects.list').handler(null, fakeCtx())
    ).rejects.toThrow(/UNAUTHENTICATED/)
  })

  // ── orcaProjects.list ──────────────────────────────────────────────────────────

  it('orcaProjects.list returns only OrcaProjects the caller is a member of, with sourceProjects', async () => {
    const orcaProjectId = await makeOrcaProject() // u-A owner
    await projectService.addMember(orcaProjectId, 'u-B', 'member')
    await sourceProjectService.linkProject(
      { orcaProjectId, ownerUserId: 'u-A', projectId: 'proj-P' },
      'u-A'
    )

    // A second OrcaProject that u-B is NOT a member of.
    const otherOrcaProject = await projectService.create({
      name: 'Other Team',
      devServerId: FAKE_DEV_SERVER_ID,
      repoPath: '/irrelevant2',
      createdBy: 'u-C'
    })

    const listMethod = findMethod(methods, 'orcaProjects.list')
    const result = (await listMethod.handler(null, fakeCtx('u-B'))) as Array<{
      orcaProject: { id: string }
      sourceProjects: { ownerUserId: string; projectId: string }[]
    }>

    expect(result).toHaveLength(1)
    expect(result[0].orcaProject.id).toBe(orcaProjectId)
    expect(result[0].sourceProjects).toEqual([{ ownerUserId: 'u-A', projectId: 'proj-P' }])
    expect(result.some((r) => r.orcaProject.id === otherOrcaProject.id)).toBe(false)
  })
})
