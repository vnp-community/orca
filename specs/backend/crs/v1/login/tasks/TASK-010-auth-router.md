# TASK-010: Tạo `src/main/auth/auth-router.ts`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.5
**Depends on:** TASK-008, TASK-009
**Blocks:** TASK-012

---

## Mục tiêu

Tạo Express Router xử lý các HTTP endpoints `/auth/*`.

---

## File cần tạo

**Path:** `src/main/auth/auth-router.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-router.ts
import { Router, type Request, type Response } from 'express'
import type { AuthManager } from './auth-manager'

const COOKIE_MAX_AGE_MS = 8 * 60 * 60 * 1000  // 8h

export function createAuthRouter(auth: AuthManager): Router {
  const router = Router()

  // ── POST /auth/local ──────────────────────────────────────────
  // Login với email + password
  router.post('/local', async (req: Request, res: Response) => {
    const { email, password } = req.body ?? {}
    if (!email || !password) {
      res.status(400).json({ error: 'missing_fields', required: ['email', 'password'] })
      return
    }

    const ip     = (req.headers['x-forwarded-for'] as string)?.split(',')[0]?.trim() ?? req.ip ?? '0.0.0.0'
    const ua     = req.headers['user-agent'] ?? ''
    const result = await auth.login({ email, password }, ip, ua)

    if (!result.success) {
      const status = result.error === 'validation_error' ? 400 : 401
      res.status(status).json({ error: result.error, detail: result.detail })
      return
    }

    res.cookie('orca_session', result.sessionId, {
      httpOnly: true,
      secure:   process.env.NODE_ENV === 'production',
      sameSite: 'lax',
      maxAge:   COOKIE_MAX_AGE_MS
    })
    res.json({
      ok:    true,
      user:  { id: result.user.id, email: result.user.email, name: result.user.name, role: result.user.role }
    })
  })

  // ── POST /auth/logout ─────────────────────────────────────────
  router.post('/logout', (req: Request, res: Response) => {
    const sessionId = extractSessionCookie(req.headers.cookie)
    if (sessionId) auth.logout(sessionId)
    res.clearCookie('orca_session')
    res.json({ ok: true })
  })

  // ── GET /auth/me ──────────────────────────────────────────────
  // Lấy thông tin user hiện tại (dùng cho frontend SPA)
  router.get('/me', (req: Request, res: Response) => {
    const session = auth.validateRequest(req.headers.cookie)
    if (!session) {
      res.status(401).json({ error: 'unauthenticated' })
      return
    }
    const user = auth.userStore.getUser(session.userId)
    if (!user) {
      res.status(401).json({ error: 'user_not_found' })
      return
    }
    res.json({ id: user.id, email: user.email, name: user.name, role: user.role, provider: user.provider })
  })

  // ── GET /auth/sso/:provider ───────────────────────────────────
  // Redirect sang OAuth2 provider (Phase 2 — stub)
  router.get('/sso/:provider', (req: Request, res: Response) => {
    const { provider } = req.params
    res.status(501).json({
      error:    'sso_not_implemented',
      provider,
      message:  'SSO will be implemented in Phase 2'
    })
  })

  return router
}

function extractSessionCookie(cookieHeader: string | undefined): string | null {
  if (!cookieHeader) return null
  const match = cookieHeader.match(/orca_session=([a-f0-9]{64})/)
  return match ? match[1]! : null
}
```

---

## HTTP Endpoints

| Method | Path | Body | Response | Cookie |
|--------|------|------|----------|--------|
| `POST` | `/auth/local` | `{email, password}` | `{ok, user}` 200 / 401 | Set `orca_session` |
| `POST` | `/auth/logout` | (none) | `{ok: true}` | Clear `orca_session` |
| `GET` | `/auth/me` | (none) | `{id, email, name, role}` / 401 | Read cookie |
| `GET` | `/auth/sso/:provider` | (none) | 501 stub | — |

---

## Acceptance Criteria

- [x] File tồn tại, TypeScript compile sạch
- [x] `POST /auth/local` với đúng credentials → 200 + `Set-Cookie: orca_session=<64hex>; HttpOnly`
- [x] `POST /auth/local` với sai credentials → 401 JSON `{ error: 'invalid_credentials' }`
- [x] `POST /auth/logout` → clear cookie + 200 `{ok: true}`
- [x] `GET /auth/me` với cookie hợp lệ → user info
- [x] `GET /auth/me` không có cookie → 401
- [x] Cookie có `httpOnly: true`, `sameSite: 'lax'`
- [x] `GET /auth/sso/:provider` → 501 (không crash)
