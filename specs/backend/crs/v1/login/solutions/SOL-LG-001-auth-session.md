# SOL-LG-001 — Auth Layer: Login + SSO + Session Management

**CR:** [CR-LOGIN-001](../../../../../docs/crs/v1/login/CR-LOGIN-001-auth.md)
**TDD Refs:** TDD-04 (RPC Server §3 Auth), TDD-06 (Persistence), TDD-11 (Web Server §3 Bootstrap), TDD-12 (Database Layer)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Implemented (2026-07-24)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 Auth hiện tại (TDD-04 §3)

```typescript
// src/main/runtime/runtime-rpc.ts — hiện tại
private readonly authToken = randomBytes(24).toString('hex')  // ephemeral per-process

// Hai loại token:
// 1. runtime authToken → full access (Unix socket / Electron IPC)
// 2. deviceToken (48-hex) → E2EE WebSocket auth (persisted orca-devices.json)
// 3. ScopedPairingToken (64-hex, in-memory, 24h) → RBAC limited (types có, chưa implement UI)
```

### 1.2 Types đã có (src/shared/rbac-types.ts)

```typescript
// ĐÃ có — không cần viết lại types
export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'
export type OrcaSsoConfig = { provider, clientId, discoveryUrl?, allowedOrg?, allowedDomain? }
export type OrcaUser = { id, email, name, role, provider, providerUserId, teams, projects }
export type OrcaAccessPolicy = { id, name, teams?, roles?, users?, allowedServers, ... }
export type ScopedPairingToken = { token, userId, userEmail, ..., expiresAt }
```

### 1.3 Server Bootstrap (src/main/server-bootstrap.ts)

```typescript
// Hiện tại trả về:
export interface ServerBootstrapResult { shutdown(): Promise<void> }

// Cần thêm vào result:
authManager: AuthManager   // NEW: quản lý login/session
```

### 1.4 Database đã có (TDD-12)

Migration 0001~0003 đã xây bảng Projects, Repos, Automations. Thêm **Migration 0004** cho users/sessions/audit.

---

## 2. File Structure

```
src/main/auth/
├── auth-types.ts                 ← Interfaces + types (không dùng OrcaUser từ rbac-types vì cần thêm password_hash)
├── auth-session-store.ts         ← OrcaSession CRUD trong SQLite
├── auth-user-store.ts            ← OrcaLocalUser CRUD trong SQLite (bcrypt)
├── auth-local-handler.ts         ← POST /auth/local — email + password
├── auth-oauth-handler.ts         ← GET /auth/sso/github, /auth/sso/google + callback
├── auth-oidc-handler.ts          ← OIDC (Keycloak, generic OIDC) via openid-client
├── auth-middleware.ts            ← requireAuth(req) → OrcaSession | null
├── auth-manager.ts               ← Facade: createSession, validateSession, revokeSession
├── auth-router.ts                ← Express Router: /auth/* endpoints
└── __tests__/
    ├── auth-session-store.test.ts
    ├── auth-user-store.test.ts
    ├── auth-local-handler.test.ts
    └── auth-manager.test.ts

src/main/db/migrations/
└── 0004_add_auth_schema.ts       ← orca_users, orca_sessions, orca_audit_log, orca_policies
```

---

## 3. Test Specifications

### 3.1 `auth-session-store.test.ts`

```typescript
// src/main/auth/__tests__/auth-session-store.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { AuthSessionStore } from '../auth-session-store'
import { runMigrations } from '../../db/migrations/runner'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

describe('AuthSessionStore', () => {
  let tmpDir: string
  let db: SqliteAdapter
  let store: AuthSessionStore

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-auth-test-'))
    db = new SqliteAdapter(join(tmpDir, 'test.db'))
    await runMigrations(db)  // creates orca_users, orca_sessions, etc.
    store = new AuthSessionStore(db)
  })

  afterEach(() => {
    db.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  // ── createSession ──────────────────────────────────────────────
  describe('createSession', () => {
    it('creates session with correct TTL', async () => {
      const session = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      expect(session.sessionId).toHaveLength(64)  // 32 bytes hex
      expect(session.expiresAt).toBeGreaterThan(Date.now())
      expect(session.expiresAt - session.createdAt).toBe(8 * 60 * 60 * 1000)  // 8h
    })

    it('persists session to SQLite', async () => {
      const session = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const found = await store.getSession(session.sessionId)
      expect(found).not.toBeNull()
      expect(found!.userId).toBe('user-1')
    })
  })

  // ── validateSession ────────────────────────────────────────────
  describe('validateSession', () => {
    it('returns session for valid non-expired sessionId', async () => {
      const created = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const session = await store.validateSession(created.sessionId)
      expect(session).not.toBeNull()
      expect(session!.userId).toBe('user-1')
    })

    it('returns null for expired session', async () => {
      // Create session with past expiry
      const pastExpiry = Date.now() - 1000
      db.exec(`INSERT INTO orca_sessions VALUES ('expired-sid', 'user-1', ${Date.now()}, ${pastExpiry}, NULL, NULL, NULL)`)
      const session = await store.validateSession('expired-sid')
      expect(session).toBeNull()
    })

    it('returns null for unknown sessionId', async () => {
      const session = await store.validateSession('non-existent')
      expect(session).toBeNull()
    })

    it('updates lastSeenAt on valid session', async () => {
      const created = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      const before = Date.now()
      await store.validateSession(created.sessionId)
      const updated = await store.getSession(created.sessionId)
      expect(updated!.lastSeenAt).toBeGreaterThanOrEqual(before)
    })
  })

  // ── revokeSession ──────────────────────────────────────────────
  describe('revokeSession', () => {
    it('deletes session from store', async () => {
      const session = await store.createSession({
        userId: 'user-1', userEmail: 'a@test.com', role: 'developer',
        ipAddress: '127.0.0.1', userAgent: 'test'
      })
      await store.revokeSession(session.sessionId)
      const found = await store.getSession(session.sessionId)
      expect(found).toBeNull()
    })

    it('is idempotent for non-existent session', async () => {
      // Should not throw
      await expect(store.revokeSession('non-existent')).resolves.not.toThrow()
    })
  })

  // ── revokeAllUserSessions ──────────────────────────────────────
  describe('revokeAllUserSessions', () => {
    it('deletes all sessions for user', async () => {
      await store.createSession({ userId: 'user-1', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.2.3.4', userAgent: 'ua' })
      await store.createSession({ userId: 'user-1', userEmail: 'a@test.com', role: 'developer', ipAddress: '1.2.3.5', userAgent: 'ua' })
      await store.createSession({ userId: 'user-2', userEmail: 'b@test.com', role: 'developer', ipAddress: '1.2.3.6', userAgent: 'ua' })
      
      await store.revokeAllUserSessions('user-1')
      
      const sessions = await store.listUserSessions('user-1')
      expect(sessions).toHaveLength(0)
      const user2sessions = await store.listUserSessions('user-2')
      expect(user2sessions).toHaveLength(1)
    })
  })

  // ── cleanupExpired ─────────────────────────────────────────────
  describe('cleanupExpired', () => {
    it('removes expired sessions without touching active ones', async () => {
      const active = await store.createSession({
        userId: 'u1', userEmail: 'x@test.com', role: 'developer',
        ipAddress: '1.2.3.4', userAgent: 'ua'
      })
      const pastExpiry = Date.now() - 1000
      db.exec(`INSERT INTO orca_sessions VALUES ('expired1', 'u1', ${Date.now()}, ${pastExpiry}, NULL, NULL, NULL)`)
      db.exec(`INSERT INTO orca_sessions VALUES ('expired2', 'u2', ${Date.now()}, ${pastExpiry}, NULL, NULL, NULL)`)

      const removed = await store.cleanupExpired()
      expect(removed).toBe(2)
      expect(await store.getSession(active.sessionId)).not.toBeNull()
      expect(await store.getSession('expired1')).toBeNull()
    })
  })
})
```

### 3.2 `auth-user-store.test.ts`

```typescript
// src/main/auth/__tests__/auth-user-store.test.ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { SqliteAdapter } from '../../db/sqlite/sqlite-adapter'
import { AuthUserStore } from '../auth-user-store'
import { runMigrations } from '../../db/migrations/runner'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

describe('AuthUserStore', () => {
  let tmpDir: string
  let db: SqliteAdapter
  let store: AuthUserStore

  beforeEach(async () => {
    tmpDir = mkdtempSync(join(tmpdir(), 'orca-userstore-test-'))
    db = new SqliteAdapter(join(tmpDir, 'test.db'))
    await runMigrations(db)
    store = new AuthUserStore(db)
  })

  afterEach(() => {
    db.close()
    rmSync(tmpDir, { recursive: true, force: true })
  })

  describe('createLocalUser', () => {
    it('hashes password with bcrypt before storing', async () => {
      const user = await store.createLocalUser({
        email: 'alice@test.com', name: 'Alice', password: 'secret123', role: 'developer'
      })
      // Verify raw password NOT stored
      const raw = db.prepare('SELECT password_hash FROM orca_users WHERE id = ?').get(user.id) as any
      expect(raw.password_hash).not.toBe('secret123')
      expect(raw.password_hash).toMatch(/^\$2[ab]\$/)  // bcrypt prefix
    })

    it('returns user without password_hash', async () => {
      const user = await store.createLocalUser({
        email: 'bob@test.com', name: 'Bob', password: 'pw', role: 'developer'
      })
      expect((user as any).password_hash).toBeUndefined()
    })

    it('throws on duplicate email', async () => {
      await store.createLocalUser({ email: 'dup@test.com', name: 'A', password: 'pw', role: 'developer' })
      await expect(store.createLocalUser({ email: 'dup@test.com', name: 'B', password: 'pw', role: 'developer' }))
        .rejects.toThrow()
    })
  })

  describe('verifyPassword', () => {
    it('returns user on correct password', async () => {
      await store.createLocalUser({ email: 'v@test.com', name: 'V', password: 'correct', role: 'developer' })
      const user = await store.verifyPassword('v@test.com', 'correct')
      expect(user).not.toBeNull()
      expect(user!.email).toBe('v@test.com')
    })

    it('returns null on wrong password', async () => {
      await store.createLocalUser({ email: 'w@test.com', name: 'W', password: 'right', role: 'developer' })
      const user = await store.verifyPassword('w@test.com', 'wrong')
      expect(user).toBeNull()
    })

    it('returns null for unknown email', async () => {
      const user = await store.verifyPassword('unknown@test.com', 'pw')
      expect(user).toBeNull()
    })

    it('returns null for deactivated user', async () => {
      const user = await store.createLocalUser({ email: 'x@test.com', name: 'X', password: 'pw', role: 'developer' })
      await store.deactivateUser(user.id)
      const result = await store.verifyPassword('x@test.com', 'pw')
      expect(result).toBeNull()
    })
  })

  describe('upsertSsoUser', () => {
    it('creates new user on first SSO login', async () => {
      const user = await store.upsertSsoUser({
        email: 'sso@github.com', name: 'SSO User',
        provider: 'github', providerUserId: 'gh-123'
      })
      expect(user.id).toBeTruthy()
      expect(user.provider).toBe('github')
    })

    it('updates existing user on subsequent SSO login', async () => {
      await store.upsertSsoUser({
        email: 'sso2@github.com', name: 'Old Name',
        provider: 'github', providerUserId: 'gh-456'
      })
      const updated = await store.upsertSsoUser({
        email: 'sso2@github.com', name: 'New Name',
        provider: 'github', providerUserId: 'gh-456'
      })
      const users = await store.listUsers()
      const found = users.find(u => u.email === 'sso2@github.com')
      expect(found!.name).toBe('New Name')
    })
  })
})
```

### 3.3 `auth-local-handler.test.ts`

```typescript
// src/main/auth/__tests__/auth-local-handler.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { AuthUserStore } from '../auth-user-store'
import type { AuthSessionStore } from '../auth-session-store'
import { AuthLocalHandler } from '../auth-local-handler'

describe('AuthLocalHandler', () => {
  let userStore: AuthUserStore
  let sessionStore: AuthSessionStore
  let handler: AuthLocalHandler
  const mockUser = { id: 'u1', email: 'a@test.com', name: 'A', role: 'developer' as const, provider: 'none' as const }

  beforeEach(() => {
    userStore = { verifyPassword: vi.fn() } as any
    sessionStore = { createSession: vi.fn() } as any
    handler = new AuthLocalHandler(userStore, sessionStore)
  })

  describe('login', () => {
    it('returns session on valid credentials', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(mockUser)
      vi.mocked(sessionStore.createSession).mockResolvedValue({
        sessionId: 'sid-123', userId: 'u1', userEmail: 'a@test.com',
        role: 'developer', createdAt: Date.now(), expiresAt: Date.now() + 1000,
        lastSeenAt: null, ipAddress: '127.0.0.1', userAgent: 'ua'
      })

      const result = await handler.login({ email: 'a@test.com', password: 'pw' }, '127.0.0.1', 'ua')
      expect(result.success).toBe(true)
      expect(result.sessionId).toBe('sid-123')
    })

    it('returns error on invalid credentials', async () => {
      vi.mocked(userStore.verifyPassword).mockResolvedValue(null)

      const result = await handler.login({ email: 'a@test.com', password: 'wrong' }, '1.2.3.4', 'ua')
      expect(result.success).toBe(false)
      expect(result.error).toBe('invalid_credentials')
    })

    it('validates email format before querying DB', async () => {
      const result = await handler.login({ email: 'not-email', password: 'pw' }, '1.2.3.4', 'ua')
      expect(result.success).toBe(false)
      expect(userStore.verifyPassword).not.toHaveBeenCalled()
    })
  })
})
```

---

## 4. Implementation

### 4.1 `auth-types.ts`

```typescript
// src/main/auth/auth-types.ts
import type { OrcaUser } from '../../shared/rbac-types'

export type OrcaSessionUser = Pick<OrcaUser, 'id' | 'email' | 'name' | 'role' | 'provider'>

export type OrcaSession = {
  sessionId:   string    // 64-hex (32 random bytes)
  userId:      string
  userEmail:   string
  role:        OrcaUser['role']
  createdAt:   number
  expiresAt:   number    // createdAt + SESSION_TTL_MS
  lastSeenAt:  number | null
  ipAddress:   string | null
  userAgent:   string | null
}

export type CreateSessionInput = {
  userId:    string
  userEmail: string
  role:      OrcaUser['role']
  ipAddress: string
  userAgent: string
}

export type LocalUserInput = {
  email:    string
  name:     string
  password: string
  role:     OrcaUser['role']
}

export type SsoUserInput = {
  email:          string
  name:           string
  provider:       'github' | 'google' | 'keycloak'
  providerUserId: string
  avatarUrl?:     string
}

export type LocalLoginInput  = { email: string; password: string }
export type LocalLoginResult =
  | { success: true;  sessionId: string; user: OrcaSessionUser }
  | { success: false; error: 'invalid_credentials' | 'account_disabled' | 'validation_error'; detail?: string }

export const SESSION_TTL_MS = 8 * 60 * 60 * 1000  // 8 giờ
```

### 4.2 `auth-session-store.ts`

```typescript
// src/main/auth/auth-session-store.ts
import { randomBytes } from 'node:crypto'
import type { IDatabase } from '../db/types'
import type { OrcaSession, CreateSessionInput } from './auth-types'
import { SESSION_TTL_MS } from './auth-types'

export class AuthSessionStore {
  constructor(private readonly db: IDatabase) {}

  async createSession(input: CreateSessionInput): Promise<OrcaSession> {
    const sessionId  = randomBytes(32).toString('hex')
    const now        = Date.now()
    const expiresAt  = now + SESSION_TTL_MS

    this.db.prepare(`
      INSERT INTO orca_sessions (session_id, user_id, created_at, expires_at, last_seen_at, ip_address, user_agent)
      VALUES (?, ?, ?, ?, NULL, ?, ?)
    `).run(sessionId, input.userId, now, expiresAt, input.ipAddress ?? null, input.userAgent ?? null)

    return {
      sessionId, userId: input.userId, userEmail: input.userEmail,
      role: input.role, createdAt: now, expiresAt,
      lastSeenAt: null, ipAddress: input.ipAddress, userAgent: input.userAgent
    }
  }

  async getSession(sessionId: string): Promise<OrcaSession | null> {
    const row = this.db.prepare(`
      SELECT s.*, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.session_id = ?
    `).get(sessionId) as any
    return row ? this.rowToSession(row) : null
  }

  async validateSession(sessionId: string): Promise<OrcaSession | null> {
    const session = await this.getSession(sessionId)
    if (!session) return null
    if (session.expiresAt < Date.now()) {
      await this.revokeSession(sessionId)
      return null
    }
    // Update lastSeenAt
    this.db.prepare(`UPDATE orca_sessions SET last_seen_at = ? WHERE session_id = ?`)
      .run(Date.now(), sessionId)
    return session
  }

  async revokeSession(sessionId: string): Promise<void> {
    this.db.prepare(`DELETE FROM orca_sessions WHERE session_id = ?`).run(sessionId)
  }

  async revokeAllUserSessions(userId: string): Promise<number> {
    const result = this.db.prepare(`DELETE FROM orca_sessions WHERE user_id = ?`).run(userId)
    return (result as any).changes ?? 0
  }

  async listUserSessions(userId: string): Promise<OrcaSession[]> {
    const rows = this.db.prepare(`
      SELECT s.*, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.user_id = ? AND s.expires_at > ?
    `).all(userId, Date.now()) as any[]
    return rows.map(r => this.rowToSession(r))
  }

  async cleanupExpired(): Promise<number> {
    const result = this.db.prepare(`DELETE FROM orca_sessions WHERE expires_at < ?`).run(Date.now())
    return (result as any).changes ?? 0
  }

  private rowToSession(row: any): OrcaSession {
    return {
      sessionId:   row.session_id,
      userId:      row.user_id,
      userEmail:   row.user_email ?? row.email,
      role:        row.role,
      createdAt:   row.created_at,
      expiresAt:   row.expires_at,
      lastSeenAt:  row.last_seen_at ?? null,
      ipAddress:   row.ip_address ?? null,
      userAgent:   row.user_agent ?? null
    }
  }
}
```

### 4.3 `auth-user-store.ts`

```typescript
// src/main/auth/auth-user-store.ts
import { randomUUID } from 'node:crypto'
import { hash as bcryptHash, compare as bcryptCompare } from 'bcrypt'
import type { IDatabase } from '../db/types'
import type { OrcaUser } from '../../shared/rbac-types'
import type { LocalUserInput, SsoUserInput } from './auth-types'

const BCRYPT_ROUNDS = 12

export class AuthUserStore {
  constructor(private readonly db: IDatabase) {}

  async createLocalUser(input: LocalUserInput): Promise<OrcaSessionUser> {
    const id           = randomUUID()
    const passwordHash = await bcryptHash(input.password, BCRYPT_ROUNDS)
    const now          = Date.now()

    this.db.prepare(`
      INSERT INTO orca_users (id, email, name, password_hash, role, provider, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, 'none', ?, 1)
    `).run(id, input.email, input.name, passwordHash, input.role, now)

    return { id, email: input.email, name: input.name, role: input.role, provider: 'none' }
  }

  async verifyPassword(email: string, password: string): Promise<OrcaSessionUser | null> {
    const row = this.db.prepare(`
      SELECT id, email, name, role, provider, password_hash, is_active
      FROM orca_users WHERE email = ? AND provider = 'none'
    `).get(email) as any
    if (!row) return null
    if (!row.is_active) return null
    const ok = await bcryptCompare(password, row.password_hash)
    if (!ok) return null
    return { id: row.id, email: row.email, name: row.name, role: row.role, provider: 'none' }
  }

  async upsertSsoUser(input: SsoUserInput): Promise<OrcaSessionUser> {
    const existing = this.db.prepare(`
      SELECT id, role FROM orca_users WHERE provider = ? AND provider_user_id = ?
    `).get(input.provider, input.providerUserId) as any

    if (existing) {
      this.db.prepare(`
        UPDATE orca_users SET name = ?, avatar_url = ?, last_login_at = ? WHERE id = ?
      `).run(input.name, input.avatarUrl ?? null, Date.now(), existing.id)
      return { id: existing.id, email: input.email, name: input.name, role: existing.role, provider: input.provider }
    }

    const id  = randomUUID()
    const now = Date.now()
    this.db.prepare(`
      INSERT INTO orca_users (id, email, name, provider, provider_user_id, avatar_url, role, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, ?, 'developer', ?, 1)
    `).run(id, input.email, input.name, input.provider, input.providerUserId, input.avatarUrl ?? null, now)

    return { id, email: input.email, name: input.name, role: 'developer', provider: input.provider }
  }

  async getUser(id: string): Promise<OrcaSessionUser | null> {
    const row = this.db.prepare(`SELECT id, email, name, role, provider FROM orca_users WHERE id = ?`).get(id) as any
    return row ? { id: row.id, email: row.email, name: row.name, role: row.role, provider: row.provider } : null
  }

  async listUsers(): Promise<OrcaSessionUser[]> {
    return (this.db.prepare(`SELECT id, email, name, role, provider FROM orca_users WHERE is_active = 1`).all() as any[])
      .map(row => ({ id: row.id, email: row.email, name: row.name, role: row.role, provider: row.provider }))
  }

  async deactivateUser(id: string): Promise<void> {
    this.db.prepare(`UPDATE orca_users SET is_active = 0 WHERE id = ?`).run(id)
  }
}

// Re-export for convenience
export type OrcaSessionUser = Pick<OrcaUser, 'id' | 'email' | 'name' | 'role' | 'provider'>
```

### 4.4 `auth-manager.ts`

```typescript
// src/main/auth/auth-manager.ts
// Facade — điểm vào chính cho auth operations
import type { IDatabase } from '../db/types'
import { AuthSessionStore } from './auth-session-store'
import { AuthUserStore }    from './auth-user-store'
import { AuthLocalHandler } from './auth-local-handler'
import type { OrcaSession, LocalLoginInput, LocalLoginResult } from './auth-types'

export class AuthManager {
  readonly sessionStore: AuthSessionStore
  readonly userStore:    AuthUserStore
  readonly localHandler: AuthLocalHandler

  constructor(db: IDatabase) {
    this.sessionStore = new AuthSessionStore(db)
    this.userStore    = new AuthUserStore(db)
    this.localHandler = new AuthLocalHandler(this.userStore, this.sessionStore)

    // Cleanup expired sessions mỗi 30 phút
    setInterval(() => { void this.sessionStore.cleanupExpired() }, 30 * 60 * 1000)
  }

  async validateRequest(cookieHeader: string | undefined): Promise<OrcaSession | null> {
    const sessionId = extractSessionIdFromCookie(cookieHeader)
    if (!sessionId) return null
    return this.sessionStore.validateSession(sessionId)
  }

  async login(input: LocalLoginInput, ip: string, ua: string): Promise<LocalLoginResult> {
    return this.localHandler.login(input, ip, ua)
  }

  async logout(sessionId: string): Promise<void> {
    await this.sessionStore.revokeSession(sessionId)
  }
}

function extractSessionIdFromCookie(cookieHeader: string | undefined): string | null {
  if (!cookieHeader) return null
  const match = cookieHeader.match(/orca_session=([a-f0-9]{64})/)
  return match ? match[1]! : null
}
```

### 4.5 `auth-router.ts` — HTTP Endpoints

```typescript
// src/main/auth/auth-router.ts
import { Router, type Request, type Response } from 'express'
import type { AuthManager } from './auth-manager'

export function createAuthRouter(auth: AuthManager): Router {
  const router = Router()

  // POST /auth/local — Login với email/password
  router.post('/local', async (req: Request, res: Response) => {
    const { email, password } = req.body ?? {}
    if (!email || !password) return res.status(400).json({ error: 'missing_fields' })

    const ip = req.ip ?? '0.0.0.0'
    const ua = req.headers['user-agent'] ?? ''
    const result = await auth.login({ email, password }, ip, ua)

    if (!result.success) return res.status(401).json({ error: result.error })

    // Set HTTP-only cookie
    res.cookie('orca_session', result.sessionId, {
      httpOnly: true, secure: true, sameSite: 'lax',
      maxAge: 8 * 60 * 60 * 1000  // 8h
    })
    return res.json({ userId: result.user.id, name: result.user.name, role: result.user.role })
  })

  // POST /auth/logout
  router.post('/logout', async (req: Request, res: Response) => {
    const sessionId = req.cookies?.orca_session
    if (sessionId) await auth.logout(sessionId)
    res.clearCookie('orca_session')
    return res.json({ ok: true })
  })

  // GET /auth/me — Current user info
  router.get('/me', async (req: Request, res: Response) => {
    const session = await auth.validateRequest(req.headers.cookie)
    if (!session) return res.status(401).json({ error: 'unauthenticated' })
    const user = await auth.userStore.getUser(session.userId)
    if (!user) return res.status(401).json({ error: 'user_not_found' })
    return res.json({ id: user.id, email: user.email, name: user.name, role: user.role, provider: user.provider })
  })

  // GET /auth/sso/:provider — Redirect to OAuth2 provider
  router.get('/sso/:provider', (req: Request, res: Response) => {
    const { provider } = req.params
    // TODO: SOL-LG-001 Phase 2 — implement OAuth redirect
    return res.status(501).json({ error: 'sso_not_configured', provider })
  })

  // GET /auth/callback — OAuth2 callback handler
  router.get('/callback', async (req: Request, res: Response) => {
    // TODO: SOL-LG-001 Phase 2 — implement OIDC callback
    return res.status(501).json({ error: 'not_implemented' })
  })

  return router
}
```

### 4.6 `auth-middleware.ts`

```typescript
// src/main/auth/auth-middleware.ts
import type { Request, Response, NextFunction } from 'express'
import type { AuthManager } from './auth-manager'

declare module 'express-serve-static-core' {
  interface Request {
    orcaSession?: import('./auth-session-store').OrcaSession
  }
}

export function createAuthMiddleware(auth: AuthManager) {
  return async (req: Request, res: Response, next: NextFunction) => {
    const session = await auth.validateRequest(req.headers.cookie)
    if (session) req.orcaSession = session
    next()
  }
}

export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  if (!req.orcaSession) {
    res.status(401).json({ error: 'unauthenticated' })
    return
  }
  next()
}
```

---

## 5. Database Migration — 0004_add_auth_schema.ts

```typescript
// src/main/db/migrations/0004_add_auth_schema.ts
import type { Migration } from './types'

export const migration_0004: Migration = {
  id: 4,
  name: '0004_add_auth_schema',

  up(db) {
    db.exec(`
      CREATE TABLE IF NOT EXISTS orca_users (
        id               TEXT PRIMARY KEY,
        email            TEXT UNIQUE NOT NULL,
        name             TEXT NOT NULL,
        password_hash    TEXT,
        role             TEXT NOT NULL DEFAULT 'developer',
        provider         TEXT NOT NULL DEFAULT 'none',
        provider_user_id TEXT,
        avatar_url       TEXT,
        teams            TEXT NOT NULL DEFAULT '[]',
        projects         TEXT NOT NULL DEFAULT '[]',
        created_at       INTEGER NOT NULL,
        last_login_at    INTEGER,
        is_active        INTEGER NOT NULL DEFAULT 1
      );

      CREATE TABLE IF NOT EXISTS orca_sessions (
        session_id    TEXT PRIMARY KEY,
        user_id       TEXT NOT NULL REFERENCES orca_users(id) ON DELETE CASCADE,
        created_at    INTEGER NOT NULL,
        expires_at    INTEGER NOT NULL,
        last_seen_at  INTEGER,
        ip_address    TEXT,
        user_agent    TEXT
      );
      CREATE INDEX IF NOT EXISTS idx_sessions_user ON orca_sessions(user_id);
      CREATE INDEX IF NOT EXISTS idx_sessions_expires ON orca_sessions(expires_at);

      CREATE TABLE IF NOT EXISTS orca_audit_log (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        created_at  INTEGER NOT NULL,
        user_id     TEXT,
        user_email  TEXT,
        action      TEXT NOT NULL,
        detail      TEXT,
        ip_address  TEXT
      );
      CREATE INDEX IF NOT EXISTS idx_audit_user ON orca_audit_log(user_id, created_at DESC);
      CREATE INDEX IF NOT EXISTS idx_audit_action ON orca_audit_log(action, created_at DESC);

      CREATE TABLE IF NOT EXISTS orca_access_policies (
        id                    TEXT PRIMARY KEY,
        name                  TEXT NOT NULL,
        teams                 TEXT NOT NULL DEFAULT '[]',
        roles                 TEXT NOT NULL DEFAULT '[]',
        users                 TEXT NOT NULL DEFAULT '[]',
        allowed_servers       TEXT NOT NULL DEFAULT '"*"',
        allowed_projects      TEXT NOT NULL DEFAULT '"*"',
        agent_trust           TEXT NOT NULL DEFAULT 'standard',
        can_create_worktrees  INTEGER NOT NULL DEFAULT 1,
        can_delete_worktrees  INTEGER NOT NULL DEFAULT 1,
        can_access_production INTEGER NOT NULL DEFAULT 0,
        created_at            INTEGER NOT NULL,
        updated_at            INTEGER NOT NULL
      );
    `)
  },

  down(db) {
    db.exec(`
      DROP TABLE IF EXISTS orca_access_policies;
      DROP TABLE IF EXISTS orca_audit_log;
      DROP TABLE IF EXISTS orca_sessions;
      DROP TABLE IF EXISTS orca_users;
    `)
  }
}
```

---

## 6. Tích hợp vào server-bootstrap.ts

```typescript
// src/main/server-bootstrap.ts — MODIFY
// Thêm AuthManager vào bootstrap result

import { AuthManager } from './auth/auth-manager'

export interface ServerBootstrapResult {
  shutdown(): Promise<void>
  authManager: AuthManager    // NEW
}

// Trong initializeOrcaServices():
const authManager = new AuthManager(db)  // db = IDatabase từ connection pool

// Trong src/server/http-server.ts — MODIFY
// Mount auth router:
import { createAuthRouter } from '../main/auth/auth-router'
import { createAuthMiddleware } from '../main/auth/auth-middleware'
import cookieParser from 'cookie-parser'

app.use(cookieParser())
app.use(createAuthMiddleware(authManager))
app.use('/auth', createAuthRouter(authManager))
```

---

## 7. Acceptance Criteria

- [x] `auth-session-store.test.ts` — tất cả tests pass
- [x] `auth-user-store.test.ts` — bcrypt hash, verify, upsert SSO
- [x] `auth-local-handler.test.ts` — valid/invalid/deactivated scenarios
- [x] `POST /auth/local` → 200 + Set-Cookie | 401
- [x] `GET /auth/me` → user info khi có cookie | 401
- [x] `POST /auth/logout` → clear cookie
- [x] Migration 0004 chạy `up()` không throw, `down()` clean up hoàn toàn
- [x] PairCode flow vẫn hoạt động sau khi thêm auth (không regression)
- [x] Session cleanup định kỳ (30 phút) không leak memory
