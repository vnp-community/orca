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
        'SELECT profile_json as profileJson FROM orca_companies WHERE id = ?',
        [companyId]
      )
    )
    if (!rows[0]) return null
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

  /** Get a department's profile JSON. Returns null if not found. */
  async getDeptProfile(deptId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        'SELECT profile_json as profileJson FROM orca_departments WHERE id = ?',
        [deptId]
      )
    )
    if (!rows[0]) return null
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
        'SELECT profile_json as profileJson FROM orca_user_profiles WHERE user_id = ?',
        [userId]
      )
    )
    if (!rows[0]) return null
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
        `SELECT c.profile_json as profileJson
         FROM orca_users u
         JOIN orca_departments d ON d.id = u.department_id
         JOIN orca_companies c ON c.id = d.company_id
         WHERE u.id = ?`,
        [userId]
      )
    )
    if (!rows[0]) return null
    return JSON.parse(rows[0].profileJson) as OrcaProfile
  }

  /**
   * Get the department profile that applies to a user.
   * JOIN: orca_users → orca_departments
   * Returns null if user has no department.
   */
  async getDeptProfileForUser(userId: string): Promise<OrcaProfile | null> {
    const rows = await this.pool.withConnection((db) =>
      db.query<{ profileJson: string }>(
        `SELECT d.profile_json as profileJson
         FROM orca_users u
         JOIN orca_departments d ON d.id = u.department_id
         WHERE u.id = ?`,
        [userId]
      )
    )
    if (!rows[0]) return null
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
}
