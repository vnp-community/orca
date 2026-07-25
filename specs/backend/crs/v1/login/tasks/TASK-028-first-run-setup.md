# TASK-028: Tạo `src/main/admin/first-run-setup.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 4 — Admin Panel
**Solution:** [SOL-LG-004](../solutions/SOL-LG-004-admin-ui.md) §4.9
**Depends on:** TASK-004 (user-store), TASK-001 (migration)
**Blocks:** TASK-031 (admin-router), TASK-029 (http-server integration)

---

## Mục tiêu

Tạo `ensureFirstAdminUser()` — idempotently seed admin user khi server khởi động lần đầu. Print credentials ra stdout.

---

## File cần tạo

**Path:** `src/main/admin/first-run-setup.ts`

---

## Nội dung

```typescript
// src/main/admin/first-run-setup.ts
import { randomBytes } from 'node:crypto'
import type { IDatabase } from '../db/types'
import type { AuthUserStore } from '../auth/auth-user-store'

/**
 * Đảm bảo có ít nhất 1 admin user trong database.
 * Chỉ tạo nếu KHÔNG có admin nào (first run).
 * Idempotent — an toàn gọi nhiều lần.
 *
 * Credentials được đọc từ env vars:
 *   ORCA_ADMIN_EMAIL    (default: admin@localhost)
 *   ORCA_ADMIN_PASSWORD (default: random 16-char hex)
 */
export async function ensureFirstAdminUser(
  db: IDatabase,
  userStore: AuthUserStore
): Promise<void> {
  // Kiểm tra đã có admin chưa
  const adminCount = userStore.countAdmins()
  if (adminCount > 0) return  // Admin đã tồn tại — skip

  const adminEmail    = process.env.ORCA_ADMIN_EMAIL    ?? 'admin@localhost'
  const adminPassword = process.env.ORCA_ADMIN_PASSWORD ?? randomBytes(8).toString('hex')
  const isRandom      = !process.env.ORCA_ADMIN_PASSWORD

  await userStore.createLocalUser({
    email:    adminEmail,
    name:     'Administrator',
    password: adminPassword,
    role:     'admin'
  })

  // Print credentials to stdout (visible in server logs on first boot)
  console.log('')
  console.log('══════════════════════════════════════════════════════')
  console.log('  ⚠️   FIRST RUN: Admin account created')
  console.log(`       Email:    ${adminEmail}`)
  console.log(`       Password: ${adminPassword}${isRandom ? ' (auto-generated)' : ''}`)
  console.log('')
  if (isRandom) {
    console.log('  ▶  Change the password immediately after first login!')
    console.log('  ▶  Or set ORCA_ADMIN_EMAIL / ORCA_ADMIN_PASSWORD env vars')
    console.log('     before starting the server to use custom credentials.')
  }
  console.log('══════════════════════════════════════════════════════')
  console.log('')
}
```

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `ensureFirstAdminUser()` là async function
- [x] Không tạo user nếu đã có admin (idempotent)
- [x] Dùng `ORCA_ADMIN_EMAIL` env nếu có
- [x] Dùng `ORCA_ADMIN_PASSWORD` env nếu có, ngược lại generate random 16-char hex
- [x] Print credentials ra stdout với format rõ ràng
- [x] Không throw khi gọi lần 2 (admin đã tồn tại)
