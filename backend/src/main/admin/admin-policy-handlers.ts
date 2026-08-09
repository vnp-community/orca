/**
 * Admin Policy Handlers — CRUD cho RBAC Access Policies (orca_access_policies)
 *
 * Handles: list, create, update, delete access policies.
 * Dùng bởi PoliciesPage trong Admin SPA để quản lý OrcaAccessPolicy (rbac-types.ts).
 * Tất cả route yêu cầu requireAdmin middleware (áp dụng qua router.use() trong admin-router.ts).
 *
 * FIX BUG-BE-HLD-007: file này trước đây không tồn tại — DB schema (orca_access_policies,
 * migration 0005) và type PolicyInput đã sẵn sàng nhưng chưa có API CRUD nào dùng chúng.
 *
 * @module main/admin/admin-policy-handlers
 */

import { randomUUID } from 'node:crypto'
import type { Request, Response } from 'express'
import type { ISyncDatabase } from '../db/types'
import type { AuditLogger } from './audit-logger'
import type { PolicyInput } from './admin-types'
import { AUDIT_ACTIONS } from './admin-types'

const VALID_AGENT_TRUST = ['minimal', 'standard', 'full'] as const

/** Shape trả về cho client — tương thích OrcaAccessPolicy (backend/src/shared/rbac-types.ts) + audit timestamps. */
type PolicyResponse = {
  id:                   string
  name:                 string
  teams:                string[]
  roles:                string[]
  users:                string[]
  allowedServers:       '*' | string[]
  allowedProjects:      '*' | string[]
  agentTrust:           'minimal' | 'standard' | 'full'
  canCreateWorktrees:   boolean
  canDeleteWorktrees:   boolean
  canAccessProduction:  boolean
  createdAt:            number
  updatedAt:            number
}

export class AdminPolicyHandlers {
  constructor(private readonly deps: {
    db:          ISyncDatabase
    auditLogger: AuditLogger
  }) {}

  /** GET /admin/api/policies */
  listPolicies = (_req: Request, res: Response): void => {
    const rows = this.deps.db.prepare(`
      SELECT * FROM orca_access_policies
      ORDER BY created_at DESC
    `).all() as Record<string, unknown>[]

    const policies = rows.map((r) => this.rowToPolicy(r))
    res.json({ policies, total: policies.length })
  }

  /** POST /admin/api/policies — Body: PolicyInput */
  createPolicy = (req: Request, res: Response): void => {
    const input = (req.body ?? {}) as PolicyInput

    if (!input.name || typeof input.name !== 'string') {
      res.status(400).json({ error: 'missing_fields', required: ['name'] })
      return
    }
    if (input.agentTrust && !(VALID_AGENT_TRUST as readonly string[]).includes(input.agentTrust)) {
      res.status(400).json({ error: 'invalid_agent_trust', allowed: VALID_AGENT_TRUST })
      return
    }

    const id  = randomUUID()
    const now = Date.now()

    try {
      this.deps.db.prepare(`
        INSERT INTO orca_access_policies
          (id, name, teams, roles, users, allowed_servers, allowed_projects,
           agent_trust, can_create_worktrees, can_delete_worktrees, can_access_production,
           created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      `).run(
        id,
        input.name,
        JSON.stringify(input.teams ?? []),
        JSON.stringify(input.roles ?? []),
        JSON.stringify(input.users ?? []),
        JSON.stringify(input.allowedServers ?? '*'),
        JSON.stringify(input.allowedProjects ?? '*'),
        input.agentTrust ?? 'standard',
        input.canCreateWorktrees === false ? 0 : 1,
        input.canDeleteWorktrees === false ? 0 : 1,
        input.canAccessProduction === true ? 1 : 0,
        now,
        now
      )
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Unknown error'
      res.status(500).json({ error: 'internal_error', message })
      return
    }

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_CREATE,
      ipAddress: req.ip,
      detail:    { policyId: id, name: input.name }
    })

    const row = this.deps.db.prepare(`SELECT * FROM orca_access_policies WHERE id = ?`).get(String(id)) as Record<string, unknown>
    res.status(201).json(this.rowToPolicy(row))
  }

  /** PUT /admin/api/policies/:id — Body: Partial<PolicyInput> */
  updatePolicy = (req: Request, res: Response): void => {
    const { id } = req.params
    const input = (req.body ?? {}) as Partial<PolicyInput>

    const existing = this.deps.db.prepare(`SELECT * FROM orca_access_policies WHERE id = ?`).get(String(id)) as Record<string, unknown> | undefined
    if (!existing) {
      res.status(404).json({ error: 'not_found' })
      return
    }
    if (input.agentTrust && !(VALID_AGENT_TRUST as readonly string[]).includes(input.agentTrust)) {
      res.status(400).json({ error: 'invalid_agent_trust', allowed: VALID_AGENT_TRUST })
      return
    }

    // Partial update — chỉ merge field nào có trong body, giữ nguyên phần còn lại
    const current = this.rowToPolicy(existing)
    const merged: PolicyResponse = {
      ...current,
      name:                input.name                ?? current.name,
      teams:               input.teams                ?? current.teams,
      roles:               input.roles                ?? current.roles,
      users:               input.users                ?? current.users,
      // PolicyInput allows generic `string`; PolicyResponse/OrcaAccessPolicy narrow it to
      // the literal '*' — every non-'*' string here is already the JSON-encoded '*' or a
      // real server/project id, so the cast is a widen-back, not a lossy conversion.
      allowedServers:      (input.allowedServers        ?? current.allowedServers) as '*' | string[],
      allowedProjects:     (input.allowedProjects       ?? current.allowedProjects) as '*' | string[],
      agentTrust:          input.agentTrust            ?? current.agentTrust,
      canCreateWorktrees:  input.canCreateWorktrees   ?? current.canCreateWorktrees,
      canDeleteWorktrees:  input.canDeleteWorktrees   ?? current.canDeleteWorktrees,
      canAccessProduction: input.canAccessProduction  ?? current.canAccessProduction,
      updatedAt:           Date.now()
    }

    this.deps.db.prepare(`
      UPDATE orca_access_policies
      SET name = ?, teams = ?, roles = ?, users = ?, allowed_servers = ?, allowed_projects = ?,
          agent_trust = ?, can_create_worktrees = ?, can_delete_worktrees = ?, can_access_production = ?,
          updated_at = ?
      WHERE id = ?
    `).run(
      merged.name,
      JSON.stringify(merged.teams),
      JSON.stringify(merged.roles),
      JSON.stringify(merged.users),
      JSON.stringify(merged.allowedServers),
      JSON.stringify(merged.allowedProjects),
      merged.agentTrust,
      merged.canCreateWorktrees ? 1 : 0,
      merged.canDeleteWorktrees ? 1 : 0,
      merged.canAccessProduction ? 1 : 0,
      merged.updatedAt,
      String(id)
    )

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_UPDATE,
      ipAddress: req.ip,
      detail:    { policyId: id, changes: Object.keys(input) }
    })

    res.json(merged)
  }

  /** DELETE /admin/api/policies/:id */
  deletePolicy = (req: Request, res: Response): void => {
    const { id } = req.params

    const existing = this.deps.db.prepare(`SELECT id, name FROM orca_access_policies WHERE id = ?`).get(String(id)) as Record<string, unknown> | undefined
    if (!existing) {
      res.status(404).json({ error: 'not_found' })
      return
    }

    this.deps.db.prepare(`DELETE FROM orca_access_policies WHERE id = ?`).run(String(id))

    this.deps.auditLogger.log({
      userId:    req.orcaSession!.userId,
      userEmail: req.orcaSession!.userEmail,
      action:    AUDIT_ACTIONS.POLICY_DELETE,
      ipAddress: req.ip,
      detail:    { policyId: id, name: existing['name'] }
    })

    res.json({ ok: true })
  }

  /** Map raw DB row → PolicyResponse (decode JSON columns, coerce INTEGER → boolean). */
  private rowToPolicy(row: Record<string, unknown>): PolicyResponse {
    return {
      id:                  row['id']   as string,
      name:                row['name'] as string,
      teams:               JSON.parse(row['teams']            as string) as string[],
      roles:               JSON.parse(row['roles']             as string) as string[],
      users:               JSON.parse(row['users']             as string) as string[],
      allowedServers:      JSON.parse(row['allowed_servers']   as string) as '*' | string[],
      allowedProjects:     JSON.parse(row['allowed_projects']  as string) as '*' | string[],
      agentTrust:          row['agent_trust'] as PolicyResponse['agentTrust'],
      canCreateWorktrees:  Boolean(row['can_create_worktrees']),
      canDeleteWorktrees:  Boolean(row['can_delete_worktrees']),
      canAccessProduction: Boolean(row['can_access_production']),
      createdAt:           row['created_at'] as number,
      updatedAt:           row['updated_at'] as number
    }
  }
}
