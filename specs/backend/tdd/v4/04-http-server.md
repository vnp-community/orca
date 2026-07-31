# TDD-BE-04: HTTP Server

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/server/http-server.ts`

---

## 1. Architecture

```
createServer() → Node.js http.Server
  │
  ├─ Express app (API layer)
  │   ├─ cookie-parser
  │   ├─ createAuthMiddleware(authManager)   ← populate req.orcaSession
  │   ├─ createAuthRouter()          → /auth/*
  │   ├─ createAdminRouter(...)      → /admin/api/*
  │   └─ /health/*                  → HealthEndpoint
  │
  └─ Raw HTTP handler (static files)
      ├─ options.apiHandler()        ← /api/* intercept
      ├─ serveFile(webRoot + url)    ← exact path match
      └─ serveFile(webRoot/index.html) ← SPA fallback
```

---

## 2. `HttpServerOptions`

```typescript
export interface HttpServerOptions {
  dbMonitor?:  HealthChecker         // /health/* routes
  authManager?: AuthManager          // /auth/* và /admin/api/*
  db?:          ISyncDatabase        // Admin stats + audit logger
  apiHandler?:  (req, res) => boolean // /api/* intercept; return true = handled
}
```

---

## 3. Route Mounting Order

1. `cookie-parser` — parse session cookie trước mọi route
2. `AuthMiddleware` — populate `req.orcaSession` nếu cookie hợp lệ
3. `AuthRouter` (`/auth`) — public, không cần auth
   - `POST /auth/local` — email/password login
   - `POST /auth/logout` — invalidate session
   - `GET /auth/me` — current user info
   - `GET /auth/sso/:provider` — SSO redirect
4. `AdminRouter` (`/admin/api`) — `requireAdmin` guard
5. `HealthEndpoint` (`/health`) — public
6. `apiHandler` — `/api/*` (e.g., agent-token)
7. Static file serving (`out/web/`)
8. SPA fallback → `index.html`

---

## 4. MIME Types

| Extension | MIME |
|-----------|------|
| `.html` | `text/html; charset=utf-8` |
| `.js` | `application/javascript; charset=utf-8` |
| `.css` | `text/css; charset=utf-8` |
| `.json` | `application/json; charset=utf-8` |
| `.png` / `.jpg` / `.gif` / `.webp` / `.avif` | `image/*` |
| `.svg` | `image/svg+xml` |
| `.woff` / `.woff2` / `.ttf` | `font/*` |
| `.ico` | `image/x-icon` |

---

## 5. SPA Fallback Logic

```typescript
// Nếu URL không match file nào → serve index.html (React Router client-side routing)
// Ngoại lệ: URL bắt đầu bằng /auth/, /admin/, /health/, /api/ → KHÔNG fallback
// Lý do: API routes không có client-side counterpart
```

---

## 6. Security Headers

Express routes tự động set:
- `Cache-Control: no-store` cho API responses
- Không set HSTS/CSP (để reverse proxy xử lý trong production)

---

## 7. WebSocket Upgrade Handling

HTTP Server cùng port nhận cả WebSocket upgrades:
- `/agent` → `AgentWebSocketServer` (wired bởi `agentWsServer.attach(httpServer)`)
- Paths khác + `ORCA_MULTI_USER=1` → `WsSessionRouter` → per-user Unix socket
- Paths khác + `ORCA_MULTI_USER=0` → ignored (client kết nối vào port 6768)

---

## 8. Admin Panel

Admin SPA được serve tại `/admin/` từ `adminRoot = out/web/admin/`:
- `GET /admin/` → `admin/index.html`
- `GET /admin/*` → SPA fallback
- API tại `/admin/api/*` → Express Router (xử lý JSON, không file)
