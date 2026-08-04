# CR-TRACE-014 — Remote Integration Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-014 |
| **Tên** | Remote Source Control Integrations (GitHub/GitLab CLI + Credentials + Preflight) — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/remote-integration.md`, `src/main/credentials/web-credential-store.ts`, `src/main/project/GitProviderCredentialService.ts`, `src/main/runtime/rpc/methods/credentials.ts`, `src/relay/external-api-connector.ts`, `src/main/ipc/preflight.ts`, `src/main/runtime/rpc/methods/preflight.ts`, `src/relay/preflight-handler.ts` |

---

## 1. Vấn đề

Flow doc mô tả 3 sub-flow xoay quanh credential mã hoá và CLI proxy, nhưng **implementation thực tế khác đáng kể so với mô tả HLD**, và cả hai đều chưa có tracing:

- **BL-INT-01 (CLI Auth Proxy)**: flow doc mô tả một cơ chế request/response thời gian thực — `gh`/`glab` CLI trên Dev Server gọi `git-credential-orca` helper, helper gửi `credential.request` qua SSH tunnel về Main, Main decrypt token và trả `credential.response`. **Không tìm thấy implementation khớp mô tả này trong code** (không có `CliAuthProxy`, không có message type `credential.request`/`credential.response`, không có "git-credential-orca" helper). Thực tế: `gh`/`glab` CLI trên Dev Server tự dùng config dir riêng theo user (`GH_CONFIG_DIR`, `GLAB_CONFIG_DIR` — `src/relay/external-api-connector.ts:74-90`, hàm `buildGhEnv()`/`buildGlabEnv()`), nghĩa là auth state được `gh auth login`/`glab auth login` lưu sẵn trên Dev Server theo từng user, không phải proxy theo từng request. Khi `gh`/`glab` CLI fail vì thiếu auth, hiện tại không có cách phân biệt "chưa từng login" với "token hết hạn" với "config dir sai path" ngoài đọc `stderr` thô.
- **BL-INT-02 (WebCredentialStore)**: `WebCredentialStore` (`src/main/credentials/web-credential-store.ts`) là AES-256-GCM store thật, dùng cho `bitbucket`/`azure-devops`/`gitea`/`linear`/`jira` (KHÔNG có `github`/`gitlab` trực tiếp trong `CredentialService` type — `GitProviderCredentialService` "mượn" slot `bitbucket` cho GitHub PAT và `gitea` cho GitLab PAT, xem comment dòng 36-37 `GitProviderCredentialService.ts`). Không có tracing nghĩa là không tách được thời gian decrypt (scrypt + AES-256-GCM, có thể chậm nếu scrypt cost cao) khỏi thời gian I/O đọc file `.enc`.
- **BL-INT-03 (Preflight)**: flow doc mô tả `PreflightService.check()` merge "local checks" (git status, tests) + "remote checks" (relay git status, GitHub CI check-runs, existing PR lookup). **Thực tế preflight hiện có (`preflight.check` RPC method, `src/main/runtime/rpc/methods/preflight.ts`) là một tính năng khác**: kiểm tra `git`/`gh`/`glab` CLI đã cài đặt + đã authenticated hay chưa (`runPreflightCheck()`, `src/main/ipc/preflight.ts:227`), route sang relay (`PreflightHandler.checkFullPreflight()`, `src/relay/preflight-handler.ts`) khi có `devServerId`. Không có unified service nào merge CI check-runs + existing-PR lookup như flow doc mô tả — logic check-runs (`src/main/github/client.ts:3441`) tồn tại nhưng phục vụ PR review panel (domain code-review, CR-TRACE-005), không phải preflight. Khi "Preflight Check" chậm, không biết đang kẹt ở việc detect `gh`/`glab` trên máy local hay đang chờ relay RPC (`preflight.check`) trả lời từ Dev Server.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thành phần thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------------------|-------------------------------|-------|-----------|-------------------------------|
| CliAuthProxy | Không tồn tại — chưa xác định file cụ thể, cần điều tra thêm khi triển khai. Thành phần gần nhất: `buildGhEnv()`/`buildGlabEnv()` (`src/relay/external-api-connector.ts:74-90`) | Remote Bridge | in-process (Dev Server) | — |
| WebCredentialStore | `WebCredentialStore` (`src/main/credentials/web-credential-store.ts`), `GitProviderCredentialService` (`src/main/project/GitProviderCredentialService.ts`) | Security | in-process (Main) | Không băng qua network — chỉ `step()` nếu decrypt có khả năng chậm (mục 5 CR-TRACE-000) |
| Main Process (CliAuthProxy/RemoteIntegrationService) | RPC methods `credentials.set`/`credentials.revoke`/`credentials.status`/`credentials.list` (`src/main/runtime/rpc/methods/credentials.ts`) | Business Logic | WebSocket RPC (Browser ↔ Orca Server) | WebSocket RPC row |
| GitHub/GitLab REST API | `src/relay/external-api-connector.ts` (`handleGitHubAuthStatus`, `handleGitLabAuthStatus`, dùng `gh`/`glab` CLI qua `execFileCaptured()`) | External | CLI exec (không phải HTTPS trực tiếp từ Dev Server code — `gh`/`glab` tự gọi API) | Không có hàng propagation — CLI exec cục bộ trên Dev Server, không phải network call từ Orca |
| SSH Relay | `relay.call()` nói chung đã có `relayCallTracer` (`relay:agentCall`, `src/main/dev-server/dev-server-relay-bridge.ts:21`) — preflight dùng relay để gọi `preflight.check` trên Dev Server | Transport | `relay.call()` | `relay.call()` row |
| (thực tế) Preflight orchestration | `runPreflightCheck()` (`src/main/ipc/preflight.ts:227`), `preflight.check` RPC (`src/main/runtime/rpc/methods/preflight.ts`), `PreflightHandler.checkFullPreflight()` (`src/relay/preflight-handler.ts`) | Business Logic | WebSocket RPC + `relay.call()` | WebSocket RPC row (Browser→Main) + `relay.call()` row (Main→Dev Server) |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  remoteIntegrationCredentialDecryptFlow: createTracer('remoteIntegration:credentialDecrypt'), // BL-INT-01: đọc + giải mã token cho gh/glab
  remoteIntegrationCredentialStoreFlow:   createTracer('remoteIntegration:credentialStore'),   // BL-INT-02: store/revoke token qua IPC/RPC
  remoteIntegrationGhExecFlow:            createTracer('remoteIntegration:ghExec'),            // BL-INT-01 (phần Dev Server): auth-status check qua gh/glab CLI
  remoteIntegrationPreflightFlow:         createTracer('remoteIntegration:preflight'),         // BL-INT-03: preflight check (local + relay)
}
```

**Ghi chú đặt tên**: BL-INT-01 nhận **2 tracer** (`credentialDecrypt` ở Main process, `ghExec` ở Dev Server/relay process) thay vì 1 — vì đây là 2 layer khác nhau, mỗi layer đo latency riêng theo đúng nguyên tắc CR-TRACE-000 §3.1 ("mỗi layer vẫn tính từ `startMs` cục bộ của chính layer đó"), giống cách `devServer:browseDir` (RPC layer) và `relay:agentCall` (transport layer) là 2 tracer riêng cho cùng 1 flow nghiệp vụ. Đây KHÔNG vi phạm nguyên tắc "1 tracer = 1 sub-flow" vì cả hai cùng phục vụ BL-INT-01, chỉ khác layer — tương tự cách CR-TRACE-006 xử lý.

## 4. Instrumentation theo từng sub-flow

### BL-INT-01 — CLI Auth Proxy (GitHub/GitLab qua SSH Relay)

> Vì `CliAuthProxy`/luồng "credential.request qua SSH tunnel" không tồn tại trong code, bảng dưới đây instrument đúng thứ tồn tại: (a) Main phía đọc/giải mã PAT khi cần, (b) Dev Server phía kiểm tra trạng thái auth trước khi chạy `gh`/`glab`.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| (Main) Bắt đầu đọc token cho 1 provider | `start` | `provider: 'github'\|'gitlab'`, `userId` (KHÔNG log token) | `src/main/project/GitProviderCredentialService.ts` (`getGitHubPAT()`/`getGitLabPAT()`) |
| (Main) Decrypt AES-256-GCM | `step('decrypt')` | `provider` | `src/main/credentials/web-credential-store.ts:127` (`getToken()`) |
| (Main) Hoàn tất | `ok` | `provider`, `found: boolean` | — |
| (Dev Server) Kiểm tra auth status trước khi exec | `start` | `cli: 'gh'\|'glab'`, `userId` | `src/relay/external-api-connector.ts` (`handleGitHubAuthStatus()` dòng 301, `handleGitLabAuthStatus()` dòng 428) |
| (Dev Server) Exec CLI | `step('exec')` | `cli`, `exitCode` | `execFileCaptured()` (`external-api-connector.ts:34`) được gọi bên trong 2 hàm trên |
| (Dev Server) Hoàn tất | `ok`/`fail` | `cli`, `authenticated: boolean` | — |

```typescript
// src/main/project/GitProviderCredentialService.ts — getGitHubPAT()
async getGitHubPAT(userId: string): Promise<string | null> {
  const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'github', userId })
  const store = this.getUserStore(userId)
  span.step('decrypt', { provider: 'github' })
  const token = await store.getToken('bitbucket') // FIX-note: slot 'bitbucket' tái dùng cho github, xem comment gốc dòng 36
  span.ok({ provider: 'github', found: token !== null })
  return token
}
```

```typescript
// src/relay/external-api-connector.ts — handleGitHubAuthStatus() (dòng 301)
export async function handleGitHubAuthStatus(/* ...existing params... */) {
  const span = Tracers.remoteIntegrationGhExecFlow.start({ cli: 'gh' })
  const result = await execFileCaptured('gh', ['auth', 'status'], { /* ...cwd, env, timeout... */ })
  span.step('exec', { cli: 'gh', exitCode: result.exitCode })
  const authenticated = result.exitCode === 0
  authenticated ? span.ok({ cli: 'gh', authenticated }) : span.fail(result.stderr, { cli: 'gh' })
  return { authenticated /* ...existing return shape... */ }
}
```

**Ràng buộc bảo mật (bắt buộc)**: không field nào trong `remoteIntegration:credentialDecrypt` hoặc `remoteIntegration:ghExec` được chứa giá trị token/PAT đã giải mã — chỉ `provider`/`userId`/`cli`/`found`/`authenticated`/`exitCode`. Đây là dữ liệu AES-256-GCM decrypted credential — vi phạm nguyên tắc này là lộ secret vào console log/TracePanel (fields hiển thị plaintext trong `serializeFields()`, `src/shared/trace/index.ts:98-106`, không có redaction tự động).

### BL-INT-02 — WebCredentialStore (API Token Management)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu (store) | `start` | `service` (KHÔNG log token) | `src/main/runtime/rpc/methods/credentials.ts` (`credentials.set`, dòng 50) |
| Encrypt + write `.enc` | `step('encryptWrite')` | `service` | `src/main/credentials/web-credential-store.ts:113` (`setToken()`) |
| Hoàn tất | `ok` | `service` | — |
| Revoke | `start`/`ok` | `service` | `credentials.ts` (`credentials.revoke`, dòng 70) → `web-credential-store.ts:150` (`deleteToken()`) |
| Lỗi (decrypt fail, file corrupt, v.v.) | `fail` | `service`, `errCode` | — |

```typescript
// src/main/runtime/rpc/methods/credentials.ts — 'credentials.set' handler
handler: async (params, ctx) => {
  const span = Tracers.remoteIntegrationCredentialStoreFlow.start({ service: params.service })
  try {
    span.step('encryptWrite', { service: params.service })
    await ctx.credentialStore.setToken(params.service, params.token, { userId: ctx.userId })
    span.ok({ service: params.service })
    return { ok: true }
  } catch (err) {
    span.fail(err, { service: params.service })
    throw err
  }
}
```

**Ràng buộc bảo mật**: field `params.token` (raw PAT/API key) **không bao giờ** được đưa vào `TraceFields` — chỉ `service` (tên service: `github`/`gitlab`/`bitbucket`/...) và `userId`/`errCode`. Áp dụng cả cho `set`/`get`/`revoke`.

### BL-INT-03 — Preflight Status Merge (Local + Remote)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `devServerId?`, `force: boolean` | `src/main/runtime/rpc/methods/preflight.ts` (`preflight.check` handler) |
| Local mode (không có `devServerId`) | `step('localCheck')` | — | `src/main/ipc/preflight.ts:227` (`runPreflightCheck()`) |
| Remote mode: relay call | `step('relayDelegate')`, forward `traceId` sang `relay:agentCall` | `devServerId` | `preflight.ts` (RPC method) gọi `relay.call('preflight.check', {}, 30_000)` — hop này tự động có `relay:agentCall` span sẵn (không cần thêm) |
| (Dev Server) Full CLI check | gộp field vào `ok()` của span phía Dev Server nếu instrument riêng, KHÔNG bắt buộc (out of scope — relay process riêng) | `gitInstalled`, `ghAuthenticated`, `glabAuthenticated` | `src/relay/preflight-handler.ts` (`checkFullPreflight()`) |
| Hoàn tất | `ok` | `devServerId?`, `ghAuthenticated`, `glabAuthenticated` | — |
| Lỗi (relay not connected) | `fail` | `devServerId` | `preflight.ts`: `throw new Error("Dev server '...' relay is not connected...")` |

```typescript
// src/main/runtime/rpc/methods/preflight.ts — 'preflight.check' handler
handler: async (params, ctx) => {
  const span = Tracers.remoteIntegrationPreflightFlow.start({
    devServerId: params.devServerId, force: params.force ?? false
  })
  try {
    if (params.devServerId && ctx.devServerManager) {
      const relay = ctx.devServerManager.getRelay(params.devServerId)
      if (!relay) {
        span.fail('relay-not-connected', { devServerId: params.devServerId })
        throw new Error(`Dev server '${params.devServerId}' relay is not connected...`)
      }
      span.step('relayDelegate', { devServerId: params.devServerId })
      // relay.call() bên dưới đã tự có span `relay:agentCall` riêng (không cần forward traceId thủ công
      // cho tới khi relay.call() hỗ trợ params.traceId theo CR-TRACE-000 §3.3 hàng relay.call())
      const result = await relay.call<Record<string, unknown>>('preflight.check', {}, 30_000)
      span.ok({ devServerId: params.devServerId })
      return result
    }
    span.step('localCheck')
    const result = await runPreflightCheck(params.force)
    span.ok({ mode: 'local' })
    return result
  } catch (err) {
    span.fail(err, { devServerId: params.devServerId })
    throw err
  }
}
```

## 5. Lan truyền traceId qua transport của flow này

1. **Browser → WebSocket RPC (`credentials.set`/`credentials.revoke`/`preflight.check`)**: theo CR-TRACE-000 §3.3 hàng "WebSocket RPC (Browser ↔ Orca Server)", request envelope nên mang field `traceId` cạnh `method`/`params`; `defineMethod()` handler đọc `params.traceId` (nếu Browser side đã có tracer riêng) và resume `remoteIntegration*Flow.start(fields, params.traceId ? { id: params.traceId } : undefined)`. Hiện tại không có field này trong `PreflightCheck`/credential schemas (`z.object({...})`) — cần thêm optional `traceId: z.string().optional()` khi core API (CR-TRACE-000 mục 3) ship.
2. **Main → Dev Server qua `relay.call('preflight.check', ...)`**: theo hàng `relay.call()` trong §3.3, `traceId` nên nằm trong params envelope. Code hiện tại gọi `relay.call<Record<string, unknown>>('preflight.check', {}, 30_000)` với params rỗng — khi core API ship, sửa thành `relay.call('preflight.check', { traceId: span.id }, 30_000)` để `relayCallTracer` (`relay:agentCall`) resume đúng `id`, cho phép TracePanel nối `remoteIntegration:preflight` (Main) với `relay:agentCall` (transport) thành 1 trace liên tục.
3. **`credentialDecrypt`/`ghExec` không băng qua network** — cả hai chạy in-process (Main hoặc Dev Server riêng biệt), không có hàng propagation nào trong §3.3 áp dụng trực tiếp; nếu muốn nối `remoteIntegrationCredentialDecryptFlow` (Main) với `remoteIntegrationGhExecFlow` (Dev Server) thành 1 trace, cần một transport nối 2 process đó — hiện tại **không tồn tại** transport thật cho việc này (đúng như phát hiện ở mục 1: không có `CliAuthProxy` request/response), nên 2 tracer này **độc lập, không resume lẫn nhau** cho tới khi transport đó được xây (ngoài phạm vi CR này).
4. **`gh`/`glab` REST API (bên trong CLI, không phải code Orca gọi trực tiếp)**: không có hàng propagation — external 3rd-party, `traceId` dừng lại ở lớp gọi CLI (`ghExec`), không có cách nào gắn vào request HTTP nội bộ của bản thân `gh`/`glab` binary.

## Acceptance Criteria

- [ ] `remoteIntegration:credentialDecrypt` và `remoteIntegration:credentialStore` không bao giờ chứa giá trị token/PAT plaintext trong bất kỳ field nào — chỉ `provider`/`service`/`userId`/`found`/`errCode`
- [ ] `remoteIntegration:ghExec` phân biệt được `cli: 'gh'` vs `cli: 'glab'` và `exitCode` trong mọi event
- [ ] `remoteIntegration:preflight` phân biệt rõ `mode: 'local'` vs relay-delegated (`devServerId` có giá trị) trong `ok()`
- [ ] Khi relay không connected, `remoteIntegration:preflight` gọi `fail()` với `reason: 'relay-not-connected'` trước khi throw, không chỉ dựa vào exception phía caller
- [ ] Xác nhận lại (trước khi implement) rằng `CliAuthProxy`/`credential.request` request-response bridge thực sự không tồn tại — nếu phát hiện tồn tại ở nơi khác chưa được grep tới, cập nhật CR trước khi viết tracer `ghExec`/`credentialDecrypt` vào sai vị trí
- [ ] Không tracer nào trong CR này trùng tên với `agent:ext-api` (đã có, trace `github.pr.create`/`github.pr.merge` trong cùng file `external-api-connector.ts`) hoặc `agent:credential` (đã có, trace AI provider credential — khác domain, xem CR-TRACE-016)
- [ ] `relay.call('preflight.check', ...)` được cập nhật gửi `traceId` trong params khi CR-TRACE-000 mục 3 (core API resume) ship
