# TASK-004: Tạo file `src/main/auth/auth-user-store.ts`

> **Status:** ✅ DONE (2026-07-24)
> **Note:** bcrypt@6.0.0 + @types/bcrypt installed

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.3
**Depends on:** TASK-001 (migration), TASK-002 (auth-types)
**Blocks:** TASK-006 (test), TASK-007 (local-handler), TASK-008 (auth-manager)

---

## Mục tiêu

Tạo `AuthUserStore` — CRUD user trong bảng `orca_users`, dùng bcrypt để hash password.

---

## Cài đặt dependency trước

```bash
pnpm add bcrypt
pnpm add -D @types/bcrypt
```

---

## File cần tạo

**Path:** `src/main/auth/auth-user-store.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-user-store.ts
import { randomUUID } from 'node:crypto'
import { hash as bcryptHash, compare as bcryptCompare } from 'bcrypt'
import type { IDatabase } from '../db/types'
import type { OrcaUser } from '../../shared/rbac-types'
import type { LocalUserInput, SsoUserInput, OrcaSessionUser } from './auth-types'

const BCRYPT_ROUNDS = 12

export class AuthUserStore {
  constructor(private readonly db: IDatabase) {}

  async createLocalUser(input: LocalUserInput): Promise<OrcaSessionUser> {
    const id           = randomUUID()
    const passwordHash = await bcryptHash(input.password, BCRYPT_ROUNDS)
    const now          = Date.now()

    this.db.prepare(`
      INSERT INTO orca_users
        (id, email, name, password_hash, role, provider, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, 'none', ?, 1)
    `).run(id, input.email, input.name, passwordHash, input.role, now)

    return { id, email: input.email, name: input.name, role: input.role, provider: 'none' }
  }

  async verifyPassword(email: string, password: string): Promise<OrcaSessionUser | null> {
    const row = this.db.prepare(`
      SELECT id, email, name, role, provider, password_hash, is_active
      FROM orca_users
      WHERE email = ? AND provider = 'none'
    `).get(email) as any
    if (!row) return null
    if (!row.is_active) return null
    const ok = await bcryptCompare(password, row.password_hash)
    if (!ok) return null
    return { id: row.id, email: row.email, name: row.name, role: row.role, provider: 'none' }
  }

  async upsertSsoUser(input: SsoUserInput): Promise<OrcaSessionUser> {
    const existing = this.db.prepare(`
      SELECT id, role
      FROM orca_users
      WHERE provider = ? AND provider_user_id = ?
    `).get(input.provider, input.providerUserId) as any

    if (existing) {
      this.db.prepare(`
        UPDATE orca_users
        SET name = ?, avatar_url = ?, last_login_at = ?
        WHERE id = ?
      `).run(input.name, input.avatarUrl ?? null, Date.now(), existing.id)
      return { id: existing.id, email: input.email, name: input.name, role: existing.role, provider: input.provider }
    }

    const id  = randomUUID()
    const now = Date.now()
    this.db.prepare(`
      INSERT INTO orca_users
        (id, email, name, provider, provider_user_id, avatar_url, role, created_at, is_active)
      VALUES (?, ?, ?, ?, ?, ?, 'developer', ?, 1)
    `).run(id, input.email, input.name, input.provider, input.providerUserId, input.avatarUrl ?? null, now)

    return { id, email: input.email, name: input.name, role: 'developer', provider: input.provider }
  }

  getUser(id: string): OrcaSessionUser | null {
    const row = this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      WHERE id = ?
    `).get(id) as any
    return row ? { id: row.id, email: row.email, name: row.name, role: row.role, provider: row.provider } : null
  }

  listUsers(): OrcaSessionUser[] {
    return (this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      ORDER BY created_at DESC
    `).all() as any[]).map(row => ({
      id: row.id, email: row.email, name: row.name, role: row.role, provider: row.provider
    }))
  }

  listActiveUsers(): OrcaSessionUser[] {
    return (this.db.prepare(`
      SELECT id, email, name, role, provider
      FROM orca_users
      WHERE is_active = 1
      ORDER BY created_at DESC
    `).all() as any[]).map(row => ({
      id: row.id, email: row.email, name: row.name, role: row.role, provider: row.provider
    }))
  }

  deactivateUser(id: string): void {
    this.db.prepare(`UPDATE orca_users SET is_active = 0 WHERE id = ?`).run(id)
  }

  countAdmins(): number {
    const row = this.db.prepare(`SELECT COUNT(*) as n FROM orca_users WHERE role = 'admin' AND is_active = 1`).get() as any
    return row?.n ?? 0
  }
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `createLocalUser()` hash password với bcrypt (rounds=12), không store plaintext
- [x] `verifyPassword()` trả về `null` khi: email không tồn tại, sai password, user bị deactivate
- [x] `upsertSsoUser()`: tạo mới nếu chưa có, update name/avatar nếu đã có (theo provider+providerUserId)
- [x] `listUsers()` trả về tất cả users (kể cả inactive)
- [x] `listActiveUsers()` chỉ trả về users có `is_active = 1`
- [x] `countAdmins()` đúng cho logic `first-run-setup`
