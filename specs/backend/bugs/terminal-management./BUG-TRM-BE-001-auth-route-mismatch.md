# BUG-TRM-BE-001 — Auth Route Mismatch: `/auth/login` Không Tồn Tại

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-TRM-005,007  
**Note:** ws-session-router.ts: 4401 close code + web-session-client.ts handler  

**ID:** BUG-TRM-BE-001
**Mức độ:** 🔴 Critical
**Module:** HTTP Server auth routing
**Phát hiện:** 2026-07-31
**Status:** 🔴 Open

---

## Mô Tả

Orca Server mount auth router tại endpoint `/auth/local` nhưng frontend (hoặc user) có thể gọi `/auth/login`. Route này trả về **HTTP 404** → user không login được → session cookie không tồn tại → mọi WebSocket request đến WsSessionRouter đều bị từ chối với `4401`.

Đây là root cause đầu tiên trong chuỗi lỗi dẫn đến Terminal không hoạt động.

---

## Root Cause

**[`http-server.ts:91`](../../../../src/server/http-server.ts):**

```typescript
app.use('/auth', createAuthRouter(options.authManager))
console.log('[HttpServer] Auth endpoints: POST /auth/local, POST /auth/logout, GET /auth/me')
```

Server log rõ ràng: endpoint là **`/auth/local`**, không phải `/auth/login`.

**Xác nhận bằng curl:**

```bash
$ curl -s -o /dev/null -w "HTTP: %{http_code}" https://b15.openledger.vn/auth/login
HTTP: 404

$ curl -s -o /dev/null -w "HTTP: %{http_code}" https://b15.openledger.vn/auth/local
HTTP: 200  # (hoặc 401 nếu không có body)
```

**Nguyên nhân gốc:** `createAuthRouter` trong [`auth-router.ts`](../../../../src/main/auth/auth-router.ts) định nghĩa `POST /local` (relative path). Khi mount tại `/auth`, endpoint đầy đủ là `/auth/local`. Nếu frontend hoặc tài liệu nội bộ ghi `/auth/login`, sẽ nhận 404.

---

## Tái Hiện

**Đã xác nhận bằng curl (2026-07-31):**

```bash
$ curl -s -o /dev/null -w "HTTP: %{http_code}" https://b15.openledger.vn/auth/login
HTTP: 404   ← ✅ Xác nhận: route này không tồn tại

$ curl -s -o /dev/null -w "HTTP: %{http_code}" https://b15.openledger.vn/api/auth/login
HTTP: 504   ← Gateway Timeout (path cũng không đúng)

$ curl -s -X POST https://b15.openledger.vn/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"test","password":"test"}'
# → HTML error page (504 Gateway Timeout)
```

**Endpoint đúng theo backend-server-architecture.md §4:**
```
POST /auth/local  ← đây mới là route hợp lệ (mount tại /auth + handler /local)
```

**Cách reproduce đầy đủ:**
1. `POST https://b15.openledger.vn/auth/login` → HTTP 404
2. Không có session cookie → WS upgrade → `ws.close(4401, 'Authentication required')`
3. Browser không gửi được `terminal.create` RPC → Terminal fail hoàn toàn

---

## Hậu Quả

- **Login hoàn toàn không hoạt động** nếu frontend gọi sai endpoint
- Không có thông báo lỗi rõ ràng cho user (404 JSON không được hiển thị)
- Terminal, Git, và mọi tính năng authenticated đều không thể dùng

---

## Fix Đề Xuất

### Phương án A — Thêm redirect/alias (không phá vỡ existing clients)

```typescript
// http-server.ts hoặc auth-router.ts
router.post('/login', (req, res, next) => {
  // alias → forward đến /local handler
  req.url = '/local'
  next()
})
```

### Phương án B — Cập nhật frontend để dùng `/auth/local`

Đảm bảo tất cả API calls trong frontend dùng đúng endpoint. Kiểm tra:
- `src/renderer/src/` — bất kỳ hardcoded `/auth/login` nào
- Tài liệu onboarding

### Phương án C — Mount cả hai routes

```typescript
app.use('/auth', createAuthRouter(options.authManager))
// Router define cả /local và /login pointing to same handler
```

---

## Files Liên Quan

| File | Vị trí | Vai trò |
|------|--------|---------|
| [`server/http-server.ts`](../../../../src/server/http-server.ts) | L91 | Mount auth router tại `/auth` |
| [`main/auth/auth-router.ts`](../../../../src/main/auth/auth-router.ts) | `POST /local` | Định nghĩa endpoint thực tế |
| `src/renderer/src/` | Login form | Nơi gọi auth API (cần kiểm tra) |
