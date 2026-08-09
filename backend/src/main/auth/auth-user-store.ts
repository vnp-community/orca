/**
 * Auth User Store
 *
 * Manages CRUD operations for users stored in `orca_users` table.
 * Handles local (email+password) and SSO (OAuth2/OIDC) users.
 * Passwords are hashed with bcrypt (12 rounds) — never stored plaintext.
 *
 * @module main/auth/auth-user-store
 */

import { randomUUID } from 'node:crypto'
import { hash as bcryptHash, compare as bcryptCompare } from 'bcrypt'
import type { IDatabase } from '../db/types'
import type { OrcaUser } from '../../shared/rbac-types'
import type { LocalUserInput, SsoUserInput, OrcaSessionUser } from './auth-types'

/** bcrypt cost factor — 12 rounds is a good balance of security and speed */
const BCRYPT_ROUNDS = 12

export class AuthUserStore {
  constructor(private readonly db: IDatabase) {}

  /**
   * Create a local (email+password) user.
   * Password is hashed with bcrypt before storage.
   * Throws on duplicate email (SQLite UNIQUE constraint).
   */
  async createLocalUser(input: LocalUserInput): Promise<OrcaSessionUser> {
    const id           = randomUUID()
    const passwordHash = await bcryptHash(input.password, BCRYPT_ROUNDS)
    const now          = Date.now()

    const stmt = await this.db.prepare(`
      INSERT INTO orca_users
        (id, email, name, password_hash, role, provider, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, 'none', ?, 1)
    `)
    await stmt.run(id, input.email, input.name, passwordHash, input.role, now)

    return { id, email: input.email, name: input.name, role: input.role, provider: 'none' }
  }

  /**
   * Verify email + plaintext password.
   * Returns null if: email not found, wrong password, or user deactivated.
   * Never exposes password_hash in the return value.
   */
  async verifyPassword(email: string, password: string): Promise<OrcaSessionUser | null> {
    const stmt = await this.db.prepare(`
      SELECT id, email, name, role, provider, password_hash, is_active
      FROM orca_users
      WHERE email = ? AND provider = 'none'
    `)
    const row = await stmt.get(email) as Record<string, unknown> | undefined
    if (!row) {return null}
    if (!row['is_active']) {return null}

    const isValid = await bcryptCompare(password, row['password_hash'] as string)
    if (!isValid) {return null}

    return {
      id:       row['id']       as string,
      email:    row['email']    as string,
      name:     row['name']     as string,
      role:     row['role']     as OrcaUser['role'],
      provider: 'none'
    }
  }

  /**
   * Upsert an SSO user: create on first login, update name/avatar on subsequent logins.
   * Lookup key is (provider, providerUserId) — not email (email can change at provider).
   */
  async upsertSsoUser(input: SsoUserInput): Promise<OrcaSessionUser> {
    const existingStmt = await this.db.prepare(`
      SELECT id, role
      FROM orca_users
      WHERE provider = ? AND provider_user_id = ?
    `)
    const existing = await existingStmt.get(input.provider, input.providerUserId) as Record<string, unknown> | undefined

    if (existing) {
      const updateStmt = await this.db.prepare(`
        UPDATE orca_users
        SET name = ?, avatar_url = ?, last_login_at = ?
        WHERE id = ?
      `)
      await updateStmt.run(input.name, input.avatarUrl ?? null, Date.now(), existing['id'])
      return {
        id:       existing['id']   as string,
        email:    input.email,
        name:     input.name,
        role:     existing['role'] as OrcaUser['role'],
        provider: input.provider
      }
    }

    // First SSO login: create as developer
    const id  = randomUUID()
    const now = Date.now()
    const insertStmt = await this.db.prepare(`
      INSERT INTO orca_users
        (id, email, name, provider, provider_user_id, avatar_url, role, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, ?, 'developer', ?, 1)
    `)
    await insertStmt.run(id, input.email, input.name, input.provider, input.providerUserId, input.avatarUrl ?? null, now)

    return { id, email: input.email, name: input.name, role: 'developer', provider: input.provider }
  }

  /** Get user by ID (returns null if not found). */
  async getUser(id: string): Promise<OrcaSessionUser | null> {
    const stmt = await this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      WHERE id = ?
    `)
    const row = await stmt.get(id) as Record<string, unknown> | undefined
    return row ? this.rowToUser(row) : null
  }

  /** List ALL users (including inactive), ordered by created_at DESC. */
  async listUsers(): Promise<OrcaSessionUser[]> {
    const stmt = await this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      ORDER BY created_at DESC
    `)
    const rows = await stmt.all()
    return rows.map((r) => this.rowToUser(r))
  }

  /** List only active users (is_active = 1). */
  async listActiveUsers(): Promise<OrcaSessionUser[]> {
    const stmt = await this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      WHERE is_active = 1
      ORDER BY created_at DESC
    `)
    const rows = await stmt.all()
    return rows.map((r) => this.rowToUser(r))
  }

  /** Soft-delete: set is_active = 0. Row is preserved for audit trail. */
  async deactivateUser(id: string): Promise<void> {
    const stmt = await this.db.prepare(`UPDATE orca_users SET is_active = 0 WHERE id = ?`)
    await stmt.run(id)
  }

  /** Count active admin users. Used by first-run-setup to detect whether seeding is needed. */
  async countAdmins(): Promise<number> {
    const stmt = await this.db.prepare(
      `SELECT COUNT(*) AS n FROM orca_users WHERE role = 'admin' AND is_active = 1`
    )
    const row = await stmt.get() as Record<string, unknown> | undefined
    return (row?.['n'] as number) ?? 0
  }

  private rowToUser(row: Record<string, unknown>): OrcaSessionUser {
    return {
      id:       row['id']       as string,
      email:    row['email']    as string,
      name:     row['name']     as string,
      role:     row['role']     as OrcaUser['role'],
      provider: row['provider'] as OrcaUser['provider']
    }
  }
}
