# CR-TRACE-006 — Project Integration Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-006 |
| **Tên** | Project Integration (GitHub/GitLab/Linear) — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/project-integration.md`, `src/main/github/client.ts`, `src/main/github/issues.ts`, `src/main/ipc/github.ts`, `src/main/gitlab/client.ts`, `src/main/ipc/gitlab.ts`, `src/main/linear/client.ts`, `src/main/linear/issue-context.ts`, `src/main/ipc/linear.ts`, `src/main/credentials/web-credential-store.ts`, `src/main/project/GitProviderCredentialService.ts`, `src/cli/handlers/worktree-linear-issue-link.ts` |

---

## 1. Vấn đề

Luồng Project Integration (BL-PI-01→04) là nơi Orca gọi ra **3 external API khác nhau** (GitHub REST/GraphQL, GitLab REST, Linear GraphQL) cộng với việc giải mã credential (`WebCredentialStore`/`GitProviderCredentialService`) trước mỗi lần gọi — đây chính là loại luồng "dễ fail do network/API thứ 3" mà CR-TRACE-000 Phase 2 xác định là ưu tiên. Hiện tại không có tracer nào, nên khi troubleshoot:

- **BL-PI-01 (Import Issues)**: `listWorkItems()` (`src/main/github/client.ts:1350`) chạy song song 2 fetch nội bộ (`issueFetch`, `prFetch` — dòng 1284, 1311) rồi merge kết quả; `fetchIssuesAsWorkItems()` bên GitLab (`src/main/gitlab/client.ts:685`) là một call riêng biệt. Nếu sync issues chậm/treo, không biết là do credential decrypt chậm, do GitHub GraphQL rate-limit, hay do GitLab/Linear API riêng lẻ chậm — 3 provider dùng chung 1 tên RPC (`issues.sync`) ở tầng UI nhưng chạy code hoàn toàn khác nhau phía Main.
- **BL-PI-02 (Tạo Worktree từ Issue)**: sub-flow này gọi lồng vào 2 flow khác đã có sẵn (`BL-WT-01` git worktree add, `BL-AG-01` spawn agent) — không có cách nào biết bước nào trong chuỗi "load issue → tạo branch → tạo worktree → spawn agent với context" đang chậm mà không phải đọc log rời rạc của 3 subsystem khác nhau.
- **BL-PI-03 (Update Issue Status)**: `updateIssue()` (`src/main/github/issues.ts:282`) và `updateMR()`/tương đương GitLab (`src/main/gitlab/client.ts:1333`) là các PATCH/mutation network call — nếu GitHub API trả rate-limit 403 hoặc timeout, hiện tại lỗi chỉ xuất hiện dưới dạng `{ ok: false, error }` trả về IPC (`src/main/ipc/github.ts:1044`) mà không có timing/context nào để phân biệt "server chậm" với "request bị GitHub từ chối".
- **BL-PI-04 (Submit PR Review)**: `addPRReviewComment()`/`addMRComment()` (GitHub: `src/main/github/client.ts:4473`; GitLab: `src/main/gitlab/client.ts:966`) đọc token từ `WebCredentialStore`/`GitProviderCredentialService` trước khi gọi API — không tách biệt được thời gian "load + decrypt token" (AES-256-GCM) khỏi thời gian gọi REST API thật sự.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Renderer (Issues list, PR panel) | UI | WebSocket RPC / Electron IPC | Browser tạo `traceId` khi có tracer riêng |
| `src/main/github/issues.ts`, `src/main/github/client.ts` | Business Logic | in-process → HTTPS | Điểm bắt đầu span nếu không nhận `traceId` từ Renderer |
| `src/main/gitlab/client.ts` | Business Logic | in-process → HTTPS | Tương tự GitHub |
| `src/main/linear/client.ts`, `src/main/linear/issue-context.ts` | Business Logic | in-process → HTTPS (GraphQL) | Tương tự GitHub |
| `WebCredentialStore` (`src/main/credentials/web-credential-store.ts`) / `GitProviderCredentialService` | Secrets | in-process | Không băng qua network — chỉ đáng `step()` nếu decrypt có khả năng chậm/fail độc lập (theo mục 5 CR-TRACE-000: "operation có khả năng fail độc lập") |
| GitHub REST/GraphQL API, GitLab REST API, Linear GraphQL API | External | HTTPS | Không có hàng propagation riêng trong CR-TRACE-000 §3.3 — external 3rd-party, không nhận `traceId` |
| SQLite (`orca_issues` cache) | Persistence | in-process | Không `step()` riêng cho UPSERT đơn giản — gộp vào `ok()` |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  projectIntegrationSyncIssuesFlow:  createTracer('projectIntegration:syncIssues'),
  projectIntegrationLinkIssueFlow:   createTracer('projectIntegration:linkIssueToWorktree'),
  projectIntegrationUpdateStatusFlow: createTracer('projectIntegration:updateIssueStatus'),
  projectIntegrationSubmitReviewFlow: createTracer('projectIntegration:submitReview'),
}
```

Ghi chú đặt tên: `linkIssueToWorktree`/`updateIssueStatus`/`submitReview` bám sát tên hành động thực tế của BL-PI-02→04 trong `project-integration.md`; ví dụ `linkRepo`/`credentialRefresh` nêu trong CR-TRACE-000 §4 là minh hoạ chung cho domain, không phải tên bắt buộc — flow doc này chưa có sub-flow "link repo" hay "credential refresh" riêng biệt (4 BL hiện có là sync/link-issue/update-status/submit-review).

## 4. Instrumentation theo từng sub-flow

### BL-PI-01 — Import GitHub/GitLab Issues

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `provider: 'github'\|'gitlab'\|'linear'`, `repoUrl` | chưa xác định RPC method cụ thể tên `issues.sync` — cần điều tra IPC channel thật khi triển khai (ứng viên gần nhất: `gh:listIssues` tại `src/main/ipc/github.ts:393`, nhưng đây là read cache/list chứ chưa chắc là "sync" trigger) |
| Load/decrypt token | `step('loadToken')` | `provider` | `src/main/credentials/web-credential-store.ts:127` (`getToken()`) cho GitLab/Linear web-mode; GitHub Electron-mode dùng `gh` CLI auth, không qua `WebCredentialStore` (xác nhận: không tìm thấy `WebCredentialStore` import trong `src/main/github/client.ts`) |
| Gọi API provider | `step('fetchIssues')` | `provider` | GitHub: `src/main/github/client.ts:1350` (`listWorkItems()`) / GitLab: `src/main/gitlab/client.ts:685` (`fetchIssuesAsWorkItems()`) / Linear: `src/main/linear/client.ts` (chưa xác định hàm fetch issues cụ thể — client.ts chỉ export connection/auth helpers, fetch logic có thể nằm ở `src/main/linear/issue-context.ts` hoặc nơi khác chưa tìm thấy) |
| UPSERT cache | gộp vào `ok()` (single-row/batch UPSERT, không cần `step()` riêng theo mục 5 CR-TRACE-000) | `count` | — |
| Hoàn tất | `ok` | `provider`, `count` | — |
| Lỗi | `fail` | `provider` | — |

```typescript
// Mẫu instrumentation cho GitHub path — gắn vào call site gọi listWorkItems()
const span = Tracers.projectIntegrationSyncIssuesFlow.start({ provider: 'github', repoUrl })
try {
  span.step('fetchIssues', { provider: 'github' })
  const items = await listWorkItems(/* ... */)
  span.ok({ provider: 'github', count: items.length })
  return items
} catch (err) {
  span.fail(err, { provider: 'github' })
  throw err
}
```

### BL-PI-02 — Tạo Worktree từ Issue/Task

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `issueId`, `provider` | chưa xác định file cụ thể — không tìm thấy RPC method `worktree.createFromIssue`; ứng viên liên quan: `src/cli/handlers/worktree-linear-issue-link.ts` (CLI path, chỉ có flag parsing hiện tại) và `src/main/linear/issue-context.ts:29` (`readLinearIssueContext()`) |
| Load issue data | `step('loadIssue')` | `issueId` | `src/main/linear/issue-context.ts:29` (`readLinearIssueContext()`) cho Linear; GitHub/GitLab tương ứng chưa xác định |
| Tạo worktree (resume điểm nối sang BL-WT-01) | `step('createWorktree')`, forward `traceId` sang tracer `worktree:create` (CR-TRACE-001) | `traceId` | liên kết chéo domain — xem CR-TRACE-001 |
| Spawn agent với context (resume sang BL-AG-01) | `step('spawnAgent')`, forward `traceId` sang tracer `agentOrch:spawn` (CR-TRACE-002) | `traceId` | liên kết chéo domain — xem CR-TRACE-002 |
| Hoàn tất | `ok` | `worktreeId`, `issueId` | — |

```typescript
// Mẫu instrumentation — minh hoạ cách 1 span cha forward traceId cho 2 sub-flow con khác domain
const span = Tracers.projectIntegrationLinkIssueFlow.start({ issueId, provider })
span.step('loadIssue', { issueId })
const issue = await readLinearIssueContext(issueId) // hoặc tương đương GitHub/GitLab

span.step('createWorktree', { traceId: span.id })
// worktree:create tracer (CR-TRACE-001) resume bằng span.id ở đây — xem CR-TRACE-000 §3.2

span.step('spawnAgent', { traceId: span.id })
// agentOrch:spawn tracer (CR-TRACE-002) resume bằng span.id ở đây

span.ok({ issueId })
```

> Vì `worktree.createFromIssue` không tồn tại như một RPC method riêng biệt trong codebase hiện tại (chưa xác định — cần điều tra thêm khi triển khai xem logic này nằm rải rác ở đâu), CR triển khai thực tế cần xác định chính xác call site trước khi thêm span.

### BL-PI-03 — Cập nhật Trạng thái Issue

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `issueId`, `provider`, `newStatus` | `src/main/ipc/github.ts:1044` (`gh:updateIssue` handler) cho GitHub; GitLab tương ứng: `src/main/gitlab/client.ts:1333` (`updateMR()` — lưu ý: đây là update MR, chưa xác định hàm update issue riêng cho GitLab) |
| Gọi API PATCH/mutation | `step('apiUpdate')` | `provider` | `src/main/github/issues.ts:282` (`updateIssue()`) |
| UPDATE cache | gộp vào `ok()` | `issueId` | — |
| Hoàn tất | `ok` | `issueId`, `newStatus` | `src/main/ipc/github.ts:1044-1060` |
| Lỗi | `fail` | `provider`, `errorMessage` | như trên (`result.ok === false` branch) |

```typescript
// src/main/ipc/github.ts — gh:updateIssue handler
ipcMain.handle('gh:updateIssue', async (event, args) => {
  const span = Tracers.projectIntegrationUpdateStatusFlow.start({
    issueId: args.number, provider: 'github'
  })
  const repo = assertRegisteredRepo(args, store)
  span.step('apiUpdate', { provider: 'github' })
  const result = await updateIssue(repo.path, args.number, args.updates, repoConnectionId(repo))
  if (result.ok) {
    span.ok({ issueId: args.number })
    broadcastWorkItemMutated({ repoPath: repo.path, repoId: repo.id, type: 'issue', number: args.number }, event.sender.id)
  } else {
    span.fail(result.error, { provider: 'github' })
  }
  return result
})
```

### BL-PI-04 — Submit PR Review lên GitHub

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `prNumber`, `verdict` | `src/main/ipc/github.ts:792` (`gh:addPRReviewComment` handler) — lưu ý flow doc mô tả `pr.submitReview` tổng hợp nhiều comment thành 1 review, nhưng implementation thực tế hiện có (`addPRReviewComment`) thao tác theo từng comment riêng lẻ; chưa xác định method "submit toàn bộ review 1 lần" tương ứng chính xác với mô tả flow doc |
| Load token | `step('loadToken')` | — | GitHub Electron-mode dùng `gh` CLI auth (không qua `WebCredentialStore`); nếu web-mode thì `src/main/credentials/web-credential-store.ts:127` |
| Gọi GitHub REST | `step('apiSubmit')` | `prNumber` | `src/main/github/client.ts:4473` (`addPRReviewComment()`) |
| Hoàn tất | `ok` | `prNumber`, `verdict` | `src/main/ipc/github.ts:792-834` |
| Lỗi | `fail` | `prNumber` | như trên |

```typescript
// src/main/ipc/github.ts — gh:addPRReviewComment handler (điểm gần nhất với BL-PI-04)
ipcMain.handle('gh:addPRReviewComment', async (event, args) => {
  const span = Tracers.projectIntegrationSubmitReviewFlow.start({ prNumber: args.prNumber })
  try {
    span.step('apiSubmit', { prNumber: args.prNumber })
    const result = await addPRReviewComment({ /* ...args */ })
    span.ok({ prNumber: args.prNumber })
    return result
  } catch (err) {
    span.fail(err, { prNumber: args.prNumber })
    throw err
  }
})
```

## 5. Lan truyền traceId qua transport của flow này

Áp dụng CR-TRACE-000 §3.2/§3.3 cụ thể cho Project Integration:

1. **Browser/Renderer → Electron IPC (`ipcMain.handle`)**: các handler trong `src/main/ipc/github.ts`/`gitlab.ts`/`linear.ts` hiện nhận `args` thuần không có field `traceId`. Theo hàng "WebSocket RPC" của CR-TRACE-000 §3.3 (áp dụng tương tự cho Electron IPC vì cùng vai trò Browser↔Main), cần thêm optional `args.traceId` và đọc nó khi gọi `Tracers.projectIntegration*Flow.start(fields, args.traceId ? { id: args.traceId } : undefined)`.
2. **Main → GitHub/GitLab/Linear API**: không có hàng propagation trong CR-TRACE-000 §3.3 vì đây là external 3rd-party HTTPS API — `traceId` KHÔNG được gửi ra ngoài (tương tự nguyên tắc ở CR-TRACE-005 mục 5.4 cho GitHub REST). Span kết thúc tại Main process, ghi lại latency/kết quả trong `ok()`/`fail()`.
3. **Liên kết chéo domain (BL-PI-02)**: đây là flow duy nhất trong 3 CR này **chủ động forward `traceId` sang tracer của domain khác cùng process** (`worktree:create` của CR-TRACE-001, `agentOrch:spawn` của CR-TRACE-002) thay vì qua network — vẫn dùng đúng cơ chế `resume: { id }` ở mục 3.1 CR-TRACE-000 vì không có transport boundary nào ở giữa (cùng Main process, chỉ là gọi hàm/service khác).
4. **`WebCredentialStore`/`GitProviderCredentialService`**: hoàn toàn in-process, không có transport nào — không áp dụng bảng §3.3, chỉ cần `step('loadToken')` nếu muốn tách latency decrypt khỏi latency API call (theo nguyên tắc mục 5: "có khả năng chậm hoặc fail độc lập").

## Acceptance Criteria

- [ ] `Tracers.projectIntegrationSyncIssuesFlow` phân biệt `provider` (`github`/`gitlab`/`linear`) trong mọi event
- [ ] `Tracers.projectIntegrationUpdateStatusFlow` bọc đúng `src/main/ipc/github.ts:1044` (`gh:updateIssue`) và ghi `fail()` khi `result.ok === false`, không chỉ khi exception ném ra
- [ ] `Tracers.projectIntegrationSubmitReviewFlow` đo được latency riêng của `addPRReviewComment()` (network call), tách khỏi latency `loadToken`
- [ ] `Tracers.projectIntegrationLinkIssueFlow` forward đúng `span.id` sang `worktree:create` và `agentOrch:spawn` khi 2 tracer đó tồn tại (CR-TRACE-001/002) — verify bằng cách bật `ORCA_TRACE=1` và xác nhận cùng `id` xuất hiện xuyên 3 tracer trong 1 lần tạo worktree từ issue
- [ ] Không có `traceId` nào bị gửi trong request tới GitHub/GitLab/Linear API (external 3rd-party, không có hàng propagation trong CR-TRACE-000 §3.3)
- [ ] Các điểm đánh dấu "chưa xác định file cụ thể" (`issues.sync` RPC method, `worktree.createFromIssue`, Linear fetch-issues function) được điều tra và chốt call site thật trước khi implement — không tạo tracer gắn vào code không tồn tại
- [ ] Không tracer nào trong CR này trùng tên với tracer nội bộ đã có hoặc với `ssh:*`/`codeReview:*` (CR-TRACE-004/005)
