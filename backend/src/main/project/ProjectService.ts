/**
 * ProjectService — CRUD service for projects and project members (TDD-15)
 *
 * Manages orca_v5_projects and orca_v5_project_members tables (migration 0007).
 * Validates devServerId exists in DevServerManager before creating a project.
 *
 * Pattern follows sql-repository.ts: pool.withConnection((db) => db.query(...))
 *
 * @module main/project/ProjectService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import { Tracers } from '../../shared/trace/tracers'
import type {
  OrcaProject,
  ProjectMember,
  ProjectRole,
  CreateProjectParams,
  UpdateProjectParams,
} from '../../shared/project-types'

/** Raw DB row for orca_v5_projects */
type ProjectRow = {
  id: string
  name: string
  description: string | null
  devServerId: string
  repoPath: string
  defaultBranch: string
  visibility: string
  createdBy: string
  createdAt: number
  updatedAt: number
}

/** Raw DB row for orca_v5_project_members */
type MemberRow = {
  projectId: string
  userId: string
  role: string
  addedAt: number
}

function rowToProject(r: ProjectRow): OrcaProject {
  return {
    id: r.id,
    name: r.name,
    description: r.description ?? undefined,
    devServerId: r.devServerId,
    repoPath: r.repoPath,
    defaultBranch: r.defaultBranch,
    visibility: r.visibility as OrcaProject['visibility'],
    createdBy: r.createdBy,
    createdAt: new Date(r.createdAt),
    updatedAt: new Date(r.updatedAt),
  }
}

function rowToMember(r: MemberRow): ProjectMember {
  return {
    projectId: r.projectId,
    userId: r.userId,
    role: r.role as ProjectRole,
    addedAt: new Date(r.addedAt),
  }
}

export class ProjectService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly devServerManager: DevServerManager
  ) {}

  /**
   * Create a new project.
   * - Validates that devServerId exists in DevServerManager
   * - Automatically adds createdBy as 'owner' member
   * @throws 'DEV_SERVER_NOT_FOUND' if devServerId is unknown
   */
  async create(params: CreateProjectParams): Promise<OrcaProject> {
    const span = Tracers.profileProjectRouteFlow.start({
      op: 'create', devServerId: params.devServerId, memberCount: 1 // creator luôn là owner đầu tiên
    })

    // Validate devServerId — list() instead of get(): GatewayDevServerManagerProxy
    // (User Process / multi-user web mode) doesn't support synchronous get() over
    // IPC and always throws (BUG-BE-RPC-002). list() is awaitable in both modes —
    // the real DevServerManager returns a plain array, awaiting a non-Promise
    // value just resolves to it immediately.
    const servers = await this.devServerManager.list()
    const server = servers.find((s) => s.id === params.devServerId) ?? null
    span.step('validateDevServer', { devServerId: params.devServerId })
    if (!server) {
      span.fail('DEV_SERVER_NOT_FOUND', { devServerId: params.devServerId })
      throw new Error(`DEV_SERVER_NOT_FOUND: devServerId '${params.devServerId}' does not exist`)
    }

    const id = randomUUID()
    const now = Date.now()
    const defaultBranch = params.defaultBranch ?? 'main'
    const visibility = params.visibility ?? 'team'

    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_v5_projects
           (id, name, description, dev_server_id, repo_path, default_branch, visibility, created_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id,
          params.name,
          params.description ?? null,
          params.devServerId,
          params.repoPath,
          defaultBranch,
          visibility,
          params.createdBy,
          now,
          now,
        ]
      )
    )

    // Auto-add creator as owner
    await this.addMember(id, params.createdBy, 'owner')

    span.ok({ op: 'create', projectId: id, devServerId: params.devServerId })
    return {
      id,
      name: params.name,
      description: params.description,
      devServerId: params.devServerId,
      repoPath: params.repoPath,
      defaultBranch,
      visibility,
      createdBy: params.createdBy,
      createdAt: new Date(now),
      updatedAt: new Date(now),
    }
  }

  /** Get a project by ID. Returns null if not found. */
  async get(projectId: string): Promise<OrcaProject | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<ProjectRow>(
        `SELECT
           id, name, description,
           dev_server_id as "devServerId",
           repo_path as "repoPath",
           default_branch as "defaultBranch",
           visibility,
           created_by as "createdBy",
           created_at as "createdAt",
           updated_at as "updatedAt"
         FROM orca_v5_projects WHERE id = ?`,
        [projectId]
      )
    )
    if (!rows[0]) {return null}
    return rowToProject(rows[0])
  }

  /**
   * List all projects where userId is a member.
   * JOIN orca_v5_project_members WHERE user_id = userId.
   */
  async list(userId: string): Promise<OrcaProject[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<ProjectRow>(
        `SELECT
           p.id, p.name, p.description,
           p.dev_server_id as "devServerId",
           p.repo_path as "repoPath",
           p.default_branch as "defaultBranch",
           p.visibility,
           p.created_by as "createdBy",
           p.created_at as "createdAt",
           p.updated_at as "updatedAt"
         FROM orca_v5_projects p
         JOIN orca_v5_project_members m ON m.project_id = p.id
         WHERE m.user_id = ?
         ORDER BY p.created_at DESC`,
        [userId]
      )
    )
    return rows.map(rowToProject)
  }

  /** Update project fields (partial patch). */
  /**
   * Update project fields (partial patch).
   * - Rebinding devServerId validates the target exists in DevServerManager,
   *   mirroring create()'s validation (ProjectService.ts:87-93).
   * @throws 'DEV_SERVER_NOT_FOUND' if patch.devServerId does not exist
   */
  async update(projectId: string, patch: UpdateProjectParams, _updatedBy: string): Promise<void> {
    const span = Tracers.profileProjectRouteFlow.start({
      op: 'update', projectId, devServerId: patch.devServerId
    })

    if (patch.devServerId !== undefined) {
      // See create()'s comment above — list() works in both real and User Process
      // (GatewayDevServerManagerProxy) mode, sync get() does not (BUG-BE-RPC-002).
      const servers = await this.devServerManager.list()
      const server = servers.find((s) => s.id === patch.devServerId) ?? null
      span.step('validateDevServer', { devServerId: patch.devServerId })
      if (!server) {
        span.fail('DEV_SERVER_NOT_FOUND', { devServerId: patch.devServerId })
        throw new Error(`DEV_SERVER_NOT_FOUND: devServerId '${patch.devServerId}' does not exist`)
      }

      // TODO(BUG-BE-HLD-020, business decision cần xác nhận): chặn rebind nếu
      // project đang có workflow execution / task chạy dở gắn với dev server
      // CŨ (patch.devServerId khác project.devServerId hiện tại) — tránh orphan
      // execution khi worker mất kết nối tới host cũ giữa chừng. TDD-15 §7 đã
      // định nghĩa sẵn error code cho tình huống tương tự ở delete()
      // (`PROJECT_HAS_ACTIVE_WORKFLOWS` — xem specs/backend/tdd/v5/15-project-binding.md
      // dòng ~276) nhưng CHƯA được implement ở đâu trong backend/src (đã grep xác
      // nhận) — nghĩa là cả delete() lẫn rebind đều thiếu check này, không riêng
      // rebind. Cần quyết định business trước khi code:
      //   1. Có cho rebind khi có execution đang chạy không, hay luôn chặn?
      //   2. Nếu chặn, dùng WorkflowOrchestrator (backend/src/main/workflow/) để
      //      query execution theo projectId + status='running' — bảng nào, cột nào?
      //   3. Nếu cho phép, execution cũ có cần được hủy/marked orphaned tự động
      //      không, hay để nguyên và chỉ warning ở UI?
      // Không tự ý implement logic chặn ở đây cho tới khi có quyết định.
    }

    const now = Date.now()
    const sets: string[] = ['updated_at = ?']
    const values: unknown[] = [now]

    if (patch.name !== undefined) { sets.push('name = ?'); values.push(patch.name) }
    if (patch.description !== undefined) { sets.push('description = ?'); values.push(patch.description) }
    if (patch.defaultBranch !== undefined) { sets.push('default_branch = ?'); values.push(patch.defaultBranch) }
    if (patch.visibility !== undefined) { sets.push('visibility = ?'); values.push(patch.visibility) }
    if (patch.devServerId !== undefined) { sets.push('dev_server_id = ?'); values.push(patch.devServerId) }

    values.push(projectId)
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_v5_projects SET ${sets.join(', ')} WHERE id = ?`, values)
    )
    span.ok({ op: 'update', projectId })
  }

  /** Delete a project (cascades to members via FK). */
  async delete(projectId: string, _deletedBy: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query('DELETE FROM orca_v5_projects WHERE id = ?', [projectId])
    )
  }

  // ── Member operations ───────────────────────────────────────────────────────

  /**
   * Add or update a member's role (upsert).
   * ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role
   */
  async addMember(projectId: string, userId: string, role: ProjectRole): Promise<void> {
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_v5_project_members (project_id, user_id, role, added_at)
         VALUES (?, ?, ?, ?)
         ON CONFLICT(project_id, user_id) DO UPDATE SET
           role     = excluded.role,
           added_at = excluded.added_at`,
        [projectId, userId, role, now]
      )
    )
  }

  /** Remove a member from a project. */
  async removeMember(projectId: string, userId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        'DELETE FROM orca_v5_project_members WHERE project_id = ? AND user_id = ?',
        [projectId, userId]
      )
    )
  }

  /** Update a member's role. */
  async updateMemberRole(projectId: string, userId: string, role: ProjectRole): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        'UPDATE orca_v5_project_members SET role = ? WHERE project_id = ? AND user_id = ?',
        [role, projectId, userId]
      )
    )
  }

  /** Get all members of a project. */
  async getMembers(projectId: string): Promise<ProjectMember[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<MemberRow>(
        `SELECT
           project_id as "projectId",
           user_id as "userId",
           role,
           added_at as "addedAt"
         FROM orca_v5_project_members WHERE project_id = ?`,
        [projectId]
      )
    )
    return rows.map(rowToMember)
  }

  /** Get a single member record. Returns null if not a member. */
  async getMember(projectId: string, userId: string): Promise<ProjectMember | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<MemberRow>(
        `SELECT
           project_id as "projectId",
           user_id as "userId",
           role,
           added_at as "addedAt"
         FROM orca_v5_project_members WHERE project_id = ? AND user_id = ?`,
        [projectId, userId]
      )
    )
    if (!rows[0]) {return null}
    return rowToMember(rows[0])
  }

  /**
   * Assert that userId is a member of projectId.
   * @returns The member record
   * @throws 'PROJECT_ACCESS_DENIED' if userId is not a member
   */
  async assertAccess(projectId: string, userId: string): Promise<ProjectMember> {
    const member = await this.getMember(projectId, userId)
    if (!member) {
      throw new Error(`PROJECT_ACCESS_DENIED: user '${userId}' is not a member of project '${projectId}'`)
    }
    return member
  }
}
