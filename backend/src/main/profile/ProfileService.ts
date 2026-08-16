/**
 * ProfileService — CRUD service for company/department/user profiles (TDD-14)
 *
 * Stores profile JSON in orca_companies, orca_departments, orca_user_profiles
 * created by migration 0006.
 *
 * Pattern follows sql-repository.ts: pool.withConnection((db) => db.query(...))
 *
 * @module main/profile/ProfileService
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'
import type { OrcaProfile } from './OrcaProfile'

/** Row shape for `profile.listDepts` — admin dept picker (CompanyProfileAdmin/DeptProfileAdmin). */
export type Department = {
  id: string
  companyId: string
  name: string
  parentDeptId: string | null
}

export class ProfileService {
  constructor(private readonly pool: IConnectionPool) {}

  // ── Company ────────────────────────────────────────────────────────────────

  /** Create a new company. Returns the generated company ID. */
  async createCompany(name: string, adminUserId: string): Promise<string> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_companies (id, name, profile_json, admin_user_id, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
        [id, name, JSON.stringify({}), adminUserId, now, now]
      )
    )
    return id
  }

  /**
   * Get a company's profile JSON.
   * Returns empty object `{}` for newly created companies (no profile set).
   * Returns null if the company does not exist.
   */
  async getCompanyProfile(companyId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        'SELECT profile_json as "profileJson" FROM orca_companies WHERE id = ?',
        [companyId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /** Replace a company's profile JSON. */
  async setCompanyProfile(
    companyId: string,
    profile: OrcaProfile,
    updatedBy: string
  ): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_companies
         SET profile_json = ?, updated_at = ?, updated_by = ?
         WHERE id = ?`,
        [JSON.stringify(profile), Date.now(), updatedBy, companyId]
      )
    )
  }

  // ── Department ─────────────────────────────────────────────────────────────

  /** Create a department under a company. Returns the generated department ID. */
  async createDepartment(
    companyId: string,
    name: string,
    parentDeptId?: string
  ): Promise<string> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_departments
           (id, company_id, name, parent_dept_id, profile_json, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        [id, companyId, name, parentDeptId ?? null, JSON.stringify({}), now, now]
      )
    )
    return id
  }

  /** List all departments across all companies (admin dept picker). */
  async listDepartments(): Promise<Department[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ id: string; companyId: string; name: string; parentDeptId: string | null }>(
        `SELECT id, company_id as "companyId", name, parent_dept_id as "parentDeptId"
         FROM orca_departments
         ORDER BY name`
      )
    )
    return rows as unknown as Department[]
  }

  /** Get a department's profile JSON. Returns null if not found. */
  async getDeptProfile(deptId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        'SELECT profile_json as "profileJson" FROM orca_departments WHERE id = ?',
        [deptId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /** Replace a department's profile JSON. */
  async setDeptProfile(
    deptId: string,
    profile: OrcaProfile,
    updatedBy: string
  ): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_departments
         SET profile_json = ?, updated_at = ?, updated_by = ?
         WHERE id = ?`,
        [JSON.stringify(profile), Date.now(), updatedBy, deptId]
      )
    )
  }

  // ── User Profile ───────────────────────────────────────────────────────────

  /** Get a user's personal profile. Returns null if no profile has been set. */
  async getUserProfile(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        'SELECT profile_json as "profileJson" FROM orca_user_profiles WHERE user_id = ?',
        [userId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /**
   * Upsert a user's personal profile.
   * INSERT … ON CONFLICT(user_id) DO UPDATE — safe for repeated calls.
   */
  async setUserProfile(userId: string, profile: OrcaProfile): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_user_profiles (user_id, profile_json, updated_at)
         VALUES (?, ?, ?)
         ON CONFLICT(user_id) DO UPDATE SET
           profile_json = excluded.profile_json,
           updated_at   = excluded.updated_at`,
        [userId, JSON.stringify(profile), Date.now()]
      )
    )
  }

  // ── Cross-entity queries ───────────────────────────────────────────────────

  /**
   * Get the company profile that applies to a user.
   * JOIN: orca_users → orca_departments → orca_companies
   * Returns null if user has no department or department has no company.
   */
  async getCompanyProfileForUser(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        `SELECT c.profile_json as "profileJson"
         FROM orca_users u
         JOIN orca_departments d ON d.id = u.department_id
         JOIN orca_companies c ON c.id = d.company_id
         WHERE u.id = ?`,
        [userId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /**
   * Get the `companyId` (= tenant id, ADR-021 §2) that a user belongs to —
   * same JOIN as `getCompanyProfileForUser()` but returns the id itself
   * rather than the profile JSON. Used by `TenantResolver`
   * (main/tenancy/tenant-resolver.ts) to populate `RpcContext.tenantId`/
   * `TenantContext` once per user-process at bootstrap (server-bootstrap.ts
   * mirrors the existing once-per-process `ORCA_USER_ID` → `ctx.userId` wiring
   * — see runtime/rpc/core.ts's `RpcContext.userId` doc comment for why that's
   * safe: `ORCA_MULTI_USER=1` forks exactly one process per authenticated user).
   * Returns null if the user has no department or the department has no company.
   */
  async getCompanyIdForUser(userId: string): Promise<string | null> {
    // Why quoted alias: unquoted `as "companyId"` comes back as `companyid` on
    // Postgres (identifiers are folded to lowercase unless quoted) — silently
    // made this always return null (row.companyId undefined), so tenant
    // resolution never succeeded (2026-08-16 incident, found while
    // diagnosing a related crash in orca-data-state-persistence.ts).
    const rows = await this.pool.withConnection((db) =>
      db.query<{ companyId: string }>(
        `SELECT c.id as "companyId"
         FROM orca_users u
         JOIN orca_departments d ON d.id = u.department_id
         JOIN orca_companies c ON c.id = d.company_id
         WHERE u.id = ?`,
        [userId]
      )
    )
    return rows[0]?.companyId ?? null
  }

  /**
   * Get the department profile that applies to a user.
   * JOIN: orca_users → orca_departments
   * Returns null if user has no department.
   */
  async getDeptProfileForUser(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        `SELECT d.profile_json as "profileJson"
         FROM orca_users u
         JOIN orca_departments d ON d.id = u.department_id
         WHERE u.id = ?`,
        [userId]
      )
    )
    if (!rows[0]) {return null}
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /**
   * Assign a user to a department.
   * Updates department_id column (added by migration 0006).
   */
  async setUserDepartment(userId: string, deptId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        'UPDATE orca_users SET department_id = ? WHERE id = ?',
        [deptId, userId]
      )
    )
  }

  // ── Team (cross-entity) ────────────────────────────────────────────────────

  /**
   * Get every team profile that applies to a user, ordered ascending by
   * `orca_team_members.priority` (migration 0016) — lowest priority first, so
   * ProfileResolver can fold them in cascade order and let the highest
   * priority (last in the returned array) win on conflicting fields.
   *
   * Standalone query, independent of TeamService — a user may belong to zero,
   * one, or many teams; an empty array is a normal result, not an error.
   */
  async getTeamProfilesForUser(
    userId: string
  ): Promise<{ teamId: string; profile: OrcaProfile }[]> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ teamId: string; profileJson: string }>(
        `SELECT tm.team_id as "teamId", t.profile_json as "profileJson"
         FROM orca_team_members tm
         JOIN orca_teams t ON t.id = tm.team_id
         WHERE tm.user_id = ?
         ORDER BY tm.priority ASC`,
        [userId]
      )
    )
    return rows.map((r) => ({
      teamId: r.teamId,
      profile: JSON.parse(r.profileJson) as OrcaProfile
    }))
  }
}
