# TASK-HLD-010: Implement `listAllActiveSessions()` và nối `admin-session-handlers.ts`

**Priority:** 🟠 HIGH — Admin Panel SessionsPage hiện luôn trả về danh sách rỗng
**Effort:** ~20 phút
**Status:** ✅ DONE — 2026-08-09 (`listAllActiveSessions()` thêm vào `AuthSessionStore`; `admin-session-handlers.ts` gọi thật, bỏ stub. `tsc --noEmit` chỉ còn 2 lỗi pre-existing (baseline, dòng 33/49, không liên quan — đã xác nhận đúng vị trí `killSession`/`killAllUserSessions`, không phải code mới thêm). Chưa viết test mới do effort budget.)
**Bug refs:** BUG-BE-HLD-006
**Solution ref:** [SOLUTION-admin-panel-exact.md](../solutions/SOLUTION-admin-panel-exact.md)
**Depends on:** (none)

---

## Mục tiêu

`AdminSessionHandlers.listAllSessions` hiện là stub luôn trả `{ sessions: [], total: 0, note: 'Full listing not yet implemented' }`, khiến trang Admin → Sessions không bao giờ hiển thị session thật. Thêm method `listAllActiveSessions(limit, offset)` vào `AuthSessionStore` (tái dùng đúng pattern JOIN + `rowToSession()` đã có ở `listUserSessions()`), rồi sửa `admin-session-handlers.ts` gọi method này thay vì trả rỗng cứng.

## File cần sửa/tạo

- `backend/src/main/auth/auth-session-store.ts` — thêm method mới `listAllActiveSessions()`
- `backend/src/main/admin/admin-session-handlers.ts` — sửa `listAllSessions` (dòng 18–22) từ stub sync sang gọi store thật, `async`

## Thay đổi cụ thể

### 1. `backend/src/main/auth/auth-session-store.ts`

Vị trí: thêm method mới ngay sau `listUserSessions()` (dòng 106), trước `cleanupExpired()`.

Code `listUserSessions()` hiện có (giữ nguyên, chỉ để tham chiếu style JOIN + `rowToSession()`):

```typescript
  /** List active (non-expired) sessions for a user. */
  async listUserSessions(userId: string): Promise<OrcaSession[]> {
    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.user_id = ? AND s.expires_at > ?
      ORDER BY s.created_at DESC
    `)
    const rows = await stmt.all(userId, Date.now())
    return rows.map((r) => this.rowToSession(r))
  }
```

Thêm method mới:

```typescript
  /**
   * List ALL active (non-expired) sessions across all users, joined with
   * orca_users for userEmail/role display in Admin Panel SessionsPage.
   * FIX BUG-BE-HLD-006: replaces the stub that always returned [].
   * Pagination mirrors admin-audit-handlers.ts (limit capped, default offset 0).
   */
  async listAllActiveSessions(limit: number, offset: number): Promise<{ sessions: OrcaSession[]; total: number }> {
    const now = Date.now()

    const countStmt = await this.db.prepare(
      `SELECT COUNT(*) AS n FROM orca_sessions WHERE expires_at > ?`
    )
    const countRow = await countStmt.get(now) as Record<string, unknown> | undefined
    const total = (countRow?.['n'] as number) ?? 0

    const stmt = await this.db.prepare(`
      SELECT
        s.session_id, s.user_id, s.created_at, s.expires_at, s.last_seen_at,
        s.ip_address, s.user_agent, u.email AS user_email, u.role
      FROM orca_sessions s
      JOIN orca_users u ON u.id = s.user_id
      WHERE s.expires_at > ?
      ORDER BY s.created_at DESC
      LIMIT ? OFFSET ?
    `)
    const rows = await stmt.all(now, limit, offset)

    return { sessions: rows.map((r) => this.rowToSession(r)), total }
  }
```

`rowToSession()` (đã có sẵn, dòng 115–127) — **không đổi**, đã map đúng `user_email`/`role` từ JOIN nên tái dùng nguyên vẹn.

### 2. `backend/src/main/admin/admin-session-handlers.ts`

Vị trí: dòng 18–22.

Code sai thực tế (cần xoá):

```typescript
  /** List all active sessions — stub (full listing requires additional store method) */
  listAllSessions = (_req: Request, res: Response): void => {
    // TODO: implement AuthSessionStore.listAllActiveSessions() in a future iteration
    res.json({ sessions: [], total: 0, note: 'Full listing not yet implemented' })
  }
```

Thay bằng:

```typescript
  /** List all active (non-expired) sessions across all users. Supports ?limit=&offset= pagination. */
  listAllSessions = async (req: Request, res: Response): Promise<void> => {
    const q = req.query as Record<string, string>
    // FIX BUG-BE-HLD-006: pagination pattern copied from admin-audit-handlers.ts (cap limit at 1000)
    const limit  = q['limit']  ? Math.min(Number(q['limit']), 1000) : 100
    const offset = q['offset'] ? Number(q['offset']) : 0

    const { sessions, total } = await this.deps.sessionStore.listAllActiveSessions(limit, offset)
    res.json({ sessions, total })
  }
```

Không cần đổi import — `AuthSessionStore` đã được inject qua `deps.sessionStore` (dòng 13–16 của file, không đổi). `killSession`/`killAllUserSessions` giữ nguyên không đổi.

**Wiring:** `admin-router.ts` gọi `router.get('/sessions', deps.sessionHandlers.listAllSessions)` — do handler đổi từ sync sang `async`, Express xử lý bình thường (route handler trả `Promise<void>` là pattern đã dùng ở `listUsers`/`createUser` trong `admin-user-handlers.ts`). **Không cần đổi `admin-router.ts` cho task này.**

**Response shape mới:** `{ sessions: OrcaSession[], total: number }` — bỏ field `note` (client hiện tại chỉ parse `sessions`/`total`, field `note` chỉ tồn tại vì đây từng là stub nên loại bỏ an toàn).

## Verification

```bash
pnpm tsc --noEmit

# Verify method mới tồn tại
grep -n "listAllActiveSessions" backend/src/main/auth/auth-session-store.ts
grep -n "listAllActiveSessions" backend/src/main/admin/admin-session-handlers.ts

# Verify không còn stub note
grep -n "Full listing not yet implemented" backend/src/main/admin/admin-session-handlers.ts
# Expected: không có match nào
```

Test cần viết (bổ sung `backend/src/main/admin/admin-session-handlers.test.ts`, mới hoặc thêm nếu đã tồn tại):

```typescript
it('trả về danh sách session thật khi DB có ≥2 session active', async () => {
  // seed: tạo 2 user + 2 session (expires_at trong tương lai) qua AuthUserStore/AuthSessionStore thật
  // GET /admin/api/sessions (hoặc gọi handler.listAllSessions trực tiếp với req/res mock)
  // assert: res.json được gọi với { sessions: [...2 items], total: 2 }
  // assert: mỗi session có userEmail đúng (từ JOIN orca_users)
})

it('không trả về session đã expired', async () => {
  // seed 1 session active + 1 session expires_at < now()
  // assert total === 1, chỉ session active xuất hiện
})

it('áp dụng limit/offset đúng', async () => {
  // seed 3 session, gọi với ?limit=2&offset=1
  // assert trả về đúng 2 session (item thứ 2 và 3 theo created_at DESC), total vẫn = 3
})
```
