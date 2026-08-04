# CR-TRACE-011 — Auth & User Management Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-011 |
| **Tên** | Auth & User Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/auth.md`, `src/main/auth/auth-manager.ts`, `src/main/auth/auth-router.ts`, `src/main/auth/auth-local-handler.ts`, `src/main/auth/auth-session-store.ts`, `src/main/auth/auth-user-store.ts`, `src/main/auth/audit-logger.ts`, `src/main/session/session-manager.ts`, `src/main/session/ws-session-router.ts`, `src/main/admin/admin-router.ts`, `src/main/admin/admin-user-handlers.ts`, `src/main/admin/admin-session-handlers.ts`, `src/main/admin/admin-audit-handlers.ts` |

---

## 1. Vấn đề

**Ràng buộc bảo mật bắt buộc:** tất cả tracer field trong CR này TUYỆT ĐỐI không được chứa `password`, `passwordHash`, session token thô, hay bất kỳ giá trị bí mật nào. Chỉ log `userId`, `email` (đã biết công khai qua audit log hiện có), `role`, `sessionId` đã tồn tại trong DB (không phải plaintext token dùng để auth), kết quả boolean, mã lỗi. Xem chi tiết ở mục "Ràng buộc bảo mật" cuối §4.

Khi điều tra source thật, phát hiện **2 trong 5 sub-flow đã có tracer riêng lẻ, không theo namespace `auth:` mà CR-TRACE-000 §4 đề xuất**:
- `src/main/session/ws-session-router.ts:17` — `const wsRouter = createTracer('wsSession:route')`
- `src/main/session/session-manager.ts:17` — `const sessionTracer = createTracer('session:spawn')`

Cả hai tracer này **không nằm trong danh sách 5 tracer đã biết** mà CR-TRACE-000 liệt kê ở GAP-3 (`browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow` + `relay:agentCall`, `agent:rpc`) — tài liệu gap-analysis đã lỗi thời ngay từ khi viết. CR này **không đổi tên** hai tracer đã tồn tại (tránh phá vỡ log/dashboard đang dùng chúng) mà bổ sung các sub-flow còn thiếu dưới namespace `auth:`, đồng thời khuyến nghị CR-TRACE-000 cập nhật GAP-3 để phản ánh đúng hiện trạng.

Vấn đề cụ thể khi troubleshoot hôm nay:
- BL-AUTH-01 (Local Login): không biết login chậm là do `bcrypt.compare` (12 rounds, có thể chậm dưới tải cao) hay do DB query `SELECT user` / `INSERT session`. `AuthLocalHandler.login()` (`src/main/auth/auth-local-handler.ts:28`) hoàn toàn không có tracing.
- BL-AUTH-02/03: **đã có** `wsSession:route` và `session:spawn`, nhưng thiếu liên kết `traceId` xuyên suốt — một session login (BL-AUTH-01) không truyền `traceId` sang WS connect (BL-AUTH-02) dù về nghiệp vụ là cùng một user session.
- BL-AUTH-04 (Admin CRUD & Kill Session): admin tạo/xoá user hoặc kill session không có span nào — không biết bước nào chậm: guard `requireAdmin`, validate Zod, `bcrypt.hash`, hay ghi DB.
- BL-AUTH-05 (Audit Log): `AuditLogger.log()` (`src/main/auth/audit-logger.ts:41`) là fire-and-forget (`void this.auditLogger?.log(...)`) — nếu ghi audit log thất bại âm thầm, hiện tại không có cách phát hiện.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 áp dụng |
|------------|-------|-----------|----------------------------|
| Browser (Login form, Admin SPA) | UI | HTTP POST / WS | — |
| `createAuthRouter()` (`src/main/auth/auth-router.ts:29`) | Backend — Express | HTTP :6769 (REST) | Hàng "WebSocket RPC (Browser ↔ Orca Server)" áp dụng tương tự cho HTTP: `traceId` là sibling field trong request body hoặc response JSON |
| `AuthManager` (`src/main/auth/auth-manager.ts:28`) | Business logic | in-process | — |
| `AuthLocalHandler` (`src/main/auth/auth-local-handler.ts:16`) | Business logic | in-process | — |
| `AuthSessionStore` / `AuthUserStore` (`src/main/auth/auth-session-store.ts`, `auth-user-store.ts`) | Persistence | in-process (SQLite) | — |
| `WsSessionRouter` (`src/main/session/ws-session-router.ts:19`, tracer `wsSession:route` **đã tồn tại**) | Business logic | WebSocket :6768 (cookie auth) → Unix socket tới child process | Hàng "WebSocket RPC (Browser ↔ Orca Server)" |
| `SessionManager` (`src/main/session/session-manager.ts:24`, tracer `session:spawn` **đã tồn tại**) | Business logic | `child_process.fork()` + Unix socket | — |
| `AuditLogger` (`src/main/auth/audit-logger.ts:33`) | Business logic | in-process, ghi DB | — |
| `createAdminRouter()` (`src/main/admin/admin-router.ts:17`) → `AdminUserHandlers` / `AdminSessionHandlers` / `AdminAuditHandlers` | Backend — Express | HTTP :6769, guard `requireAdmin` | Hàng HTTP tương tự trên |
| Server Database (`orca_users`, `orca_sessions`, `orca_audit_log`) | Persistence | in-process | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged, KHÔNG đổi tên 2 tracer đã tồn tại:
  //   wsSession:route  (src/main/session/ws-session-router.ts)
  //   session:spawn    (src/main/session/session-manager.ts)
  authLoginFlow:           createTracer('auth:login'),           // BL-AUTH-01
  authAdminUserCrudFlow:   createTracer('auth:adminUserCrud'),   // BL-AUTH-04 (users)
  authAdminSessionKillFlow: createTracer('auth:adminSessionKill'), // BL-AUTH-04 (kill session)
  authAuditWriteFlow:      createTracer('auth:auditWrite'),      // BL-AUTH-05
}
```

Ghi chú: BL-AUTH-02 (Session Management) và BL-AUTH-03 (Per-User Sandbox) **đã có tracer** (`wsSession:route`, `session:spawn`) — CR này chỉ bổ sung field/step cho chúng ở mục 4, không tạo tracer trùng lặp (vi phạm nguyên tắc "1 tracer = 1 sub-flow" của CR-TRACE-000 §4 nếu tạo thêm `auth:sessionRefresh` chồng lên `wsSession:route`).

## 4. Instrumentation theo từng sub-flow

### BL-AUTH-01 — Local Login (email + password)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận POST /auth/local | `start` | `{ hasEmail: boolean }` — **không log email/password thô ở start** | `src/main/auth/auth-router.ts:34` — route `/local` |
| Verify credentials | `step('verify')` | `{ found: boolean }` — không log password/hash | `src/main/auth/auth-local-handler.ts:28` — `AuthLocalHandler.login()` |
| Tạo session | `step('session-create')` | `{ userId }` (không log token) | `auth-local-handler.ts:48` — `sessionStore.createSession()` |
| Kết quả | `ok` / `fail` | `{ userId, sessionId }` (sessionId là DB id, không phải giá trị dùng để auth) / `{ reason: 'invalid_credentials' \| 'validation_error' }` | `src/main/auth/auth-router.ts` sau `authManager.login()` |

```typescript
// src/main/auth/auth-router.ts — POST /auth/local
router.post('/local', async (req: Request, res: Response): Promise<void> => {
  const { email, password } = (req.body ?? {}) as Record<string, unknown>
  const span = Tracers.authLoginFlow.start({ hasEmail: typeof email === 'string' && email.length > 0 })

  const result = await authManager.login(
    { email: String(email ?? ''), password: String(password ?? '') },
    req.ip ?? req.socket?.remoteAddress ?? '0.0.0.0',
    req.headers['user-agent'] ?? 'unknown'
  )

  if (!result.success) {
    // Why: never include email/password in fail() fields — audit log (DB) is
    // the system of record for failed-login detail, trace is for latency/step.
    span.fail(result.error, { reason: result.error })
    res.status(result.error === 'validation_error' ? 400 : 401).json({ error: result.error })
    return
  }
  span.ok({ userId: result.user.id, sessionId: result.sessionId })
  res.cookie(SESSION_COOKIE_NAME, result.sessionId, COOKIE_OPTIONS)
  res.status(200).json({ ok: true, user: result.user })
})
```

### BL-AUTH-02 — Session Management & Isolation *(mở rộng tracer đã có `wsSession:route`)*

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| WS connect, resolve session | `start` (đã có) | *(hiện tại không field)* → đề xuất thêm `{ hasCookie: boolean }` | `src/main/session/ws-session-router.ts:48` — `WsSessionRouter.handleConnection()` |
| Auth fail | `fail` (đã có) | `{ cookie: 'present' \| 'absent' }` — giữ nguyên, đã an toàn | dòng 52 |
| Accepted | `step('accepted')` (đã có) | `{ userId }` — giữ nguyên | dòng 57 |
| Spawn/reuse child | `step('proxy-start')` (đã có) | `{ userId, socketPath }` | dòng 91 |
| **Đề xuất bổ sung:** liên kết với `auth:login` | `start(fields, resume)` | Nếu client gửi `traceId` từ session cookie context (không khả thi qua WS handshake thường — xem mục 5) thì resume; nếu không, giữ span độc lập như hiện tại | — |

Không cần code thay đổi bắt buộc ở BL-AUTH-02 ngoài việc xác nhận field hiện có đã an toàn (đã kiểm tra: không log cookie giá trị thật, chỉ log `'present'/'absent'`).

### BL-AUTH-03 — Per-User Process Sandbox *(mở rộng tracer đã có `session:spawn`)*

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu spawn | `start` (đã có) | `{ userId }` | `src/main/session/session-manager.ts:69` — `spawnUserProcess()` |
| Fork xong | `step('forked')` (đã có) | `{ pid }` | dòng 100 |
| **Đề xuất bổ sung:** crash/respawn | `step('respawn')` | `{ userId, attempt, exitCode }` | `src/main/session/session-manager.ts:149` — `child.on('exit', ...)` handler |

### BL-AUTH-04 — Admin User CRUD & Session Kill

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Guard admin | `start` → `step('admin-guard')` | `{ actorRole }` | `src/main/admin/admin-router.ts:26` (`requireAdmin` middleware) + `AdminUserHandlers.createUser` (`src/main/admin/admin-user-handlers.ts:30`) |
| Validate + hash password | `step('validate-hash')` | `{ emailValid: boolean }` — **không log password/hash** | `admin-user-handlers.ts:30` |
| Ghi DB | (gộp vào `ok`, theo nguyên tắc CR-TRACE-000 §5 — single-row INSERT không cần step riêng) | `{ userId }` | `admin-user-handlers.ts:30` |
| Deactivate user | `start` → `ok`/`fail` | `{ userId, sessionsKicked: number }` | `admin-user-handlers.ts:70` — `deactivateUser` |
| Kill session | `start` → `ok`/`fail` | `{ sessionId }` (không log token) | `src/main/admin/admin-session-handlers.ts:25` — `killSession` |

```typescript
// src/main/admin/admin-user-handlers.ts
createUser = async (req: Request, res: Response): Promise<void> => {
  const span = Tracers.authAdminUserCrudFlow.start({ op: 'create' })
  try {
    // ...existing Zod validation + email-uniqueness check...
    span.step('validate-hash', { emailValid: true })
    const user = await this.userStore.createUser({ /* ...existing fields... */ })
    span.ok({ userId: user.id })
    res.status(201).json({ user })
  } catch (err) {
    span.fail(err, { op: 'create' })
    throw err
  }
}
```

### BL-AUTH-05 — Audit Log

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Ghi audit entry | `start` → `ok`/`fail` | `{ action, targetType }` — **không log `metadata`/`ip_address` thô vào trace field nếu chứa PII nhạy cảm; chỉ log action + outcome** | `src/main/auth/audit-logger.ts:41` — `AuditLogger.log()` |
| Query audit (admin) | `start` → `ok` | `{ actionFilter, page, resultCount }` | `src/main/admin/admin-audit-handlers.ts:17` — `queryAuditLog` |

```typescript
// src/main/auth/audit-logger.ts
async log(entry: AuditEntry): Promise<void> {
  const span = Tracers.authAuditWriteFlow.start({ action: entry.action, targetType: entry.details?.targetType })
  try {
    // ...existing INSERT orca_audit_log...
    span.ok()
  } catch (err) {
    // Why: audit writes are fire-and-forget at call sites (`void auditLogger?.log(...)`)
    // — fail() here is the ONLY signal a swallowed audit-write error ever gets.
    span.fail(err, { action: entry.action })
  }
}
```

**Ràng buộc bảo mật (áp dụng cho toàn bộ CR này):**
- KHÔNG BAO GIỜ đưa `password`, `passwordHash`, session token giá trị thật, hay nội dung `Cookie` header vào bất kỳ `TraceFields` nào (`start`/`step`/`ok`/`fail`).
- `sessionId` được log trong CR này là **DB primary key** dùng để tra cứu (giống cách `orca_sessions.id` đã xuất hiện trong audit log hiện có), không phải giá trị cookie 64-hex dùng để authenticate — hai giá trị này khác nhau trong `auth-session-store.ts` (cần xác nhận field chính xác khi triển khai; nếu `sessionId` và session token là cùng một giá trị trong implementation thật, đổi sang chỉ log 8 ký tự đầu hoặc một id ẩn danh khác).
- `console.error`/`fail()` của tracer core (`src/shared/trace/index.ts:131-134`) luôn log ra console kể cả khi `ORCA_TRACE` tắt — vì vậy `fail()` fields ở auth flow càng phải kiểm tra kỹ, tránh vô tình bật `ORCA_TRACE=0` mà vẫn leak field nhạy cảm qua console.error.

## 5. Lan truyền traceId qua transport của flow này

- **HTTP `/auth/local`, `/admin/api/*`:** không có RPC envelope `id`/`method`/`params` như WS RPC — áp dụng biến thể của CR-TRACE-000 §3.3 hàng đầu tiên: `traceId` là optional field trong response JSON body (`{ ok, user, traceId }`) để hỗ trợ correlate log phía client nếu cần, nhưng KHÔNG bắt buộc client gửi `traceId` request vì login là điểm khởi đầu của toàn bộ chuỗi (span luôn `start()` mới, không `resume`).
- **WS :6768 (`WsSessionRouter`) → Unix socket → child process:** đây là nơi duy nhất trong flow này thực sự "băng qua" nhiều layer. Do WS handshake không có `params` JSON để nhét `traceId` (chỉ có Cookie header), **không áp dụng resume từ layer trước** — `wsSession:route` tiếp tục là span độc lập như hiện tại. Nếu cần liên kết với `auth:login`, cách khả thi duy nhất là thêm `X-Orca-Trace-Id` header tuỳ chọn ở lần connect đầu (không có trong code hiện tại — đề xuất tương lai, không nằm trong scope P1 của CR này).
- **`session:spawn` → child process (`fork()` + Unix socket nội bộ user):** không băng qua network, nhưng băng qua process boundary thật (`child_process.fork`) — theo CR-TRACE-000 §5 nguyên tắc 1 ("băng qua process boundary — luôn đáng trace"), giữ nguyên các step đã có (`forked`) và bổ sung `respawn` step khi crash.
- **Audit log ghi DB:** không có transport nào để lan truyền — `auth:auditWrite` luôn là span mới, không resume.

## Acceptance Criteria

- [ ] `Tracers.authLoginFlow`, `authAdminUserCrudFlow`, `authAdminSessionKillFlow`, `authAuditWriteFlow` thêm vào `tracers.ts`, không đổi tên `wsSession:route`/`session:spawn` đã tồn tại
- [ ] Không có trace field nào (start/step/ok/fail) trong toàn bộ auth flow chứa `password`, `passwordHash`, hoặc giá trị session token dùng để authenticate — review thủ công từng call site khi triển khai
- [ ] `POST /auth/local` có span bao trọn `AuthManager.login()`, phân biệt được fail do `validation_error` vs `invalid_credentials`
- [ ] `AuditLogger.log()` có `fail()` khi ghi DB lỗi — xác nhận bằng test giả lập DB write error, kiểm tra console.error xuất hiện
- [ ] Admin create/deactivate user và kill session đều có span `ok`/`fail` tương ứng response HTTP status
- [ ] CR-TRACE-000 GAP-3 được cập nhật để liệt kê thêm `wsSession:route` và `session:spawn` vào danh sách tracer đã tồn tại
- [ ] Code review checklist: mọi field mới thêm vào trace tại auth flow được kiểm tra chéo với danh sách "không log credentials" ở mục 4 trước khi merge
