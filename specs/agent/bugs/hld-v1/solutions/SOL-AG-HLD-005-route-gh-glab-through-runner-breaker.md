# SOL-AG-HLD-005 — Route `gh`/`glab` calls qua `runner.ts`'s circuit breaker, ưu tiên PR/MR create

**Fixes:** [BUG-AG-HLD-005](../BUG-AG-HLD-005-gh-rate-limit-breaker-not-wired.md)
**TDD Ref:** TDD-AG-13 §3.3/§4.1 (`handleGitHubPrCreate`/`handleGitLabMrCreate` — call sites cần đổi), §5 (`execFileCaptured` — helper sẽ được thay ở các call site `gh`/`glab`), §8 (Design Principles: "Timeout mandatory", "CLI-based, not SDK" — không đổi, chỉ đổi *cách gọi* CLI)
**File:** `agent/src/relay/external-api-connector.ts`, `agent/src/relay/agent-git-handler.ts`, `agent/src/relay/git-handler.ts`, `agent/src/main/git/runner.ts`
**Effort:** 8-14 giờ (chia nhỏ theo phase — xem bên dưới); **Phase 1 (ưu tiên) riêng: 2-3 giờ**
**Status:** 🔴 TODO

---

## Phân Tích

**Xác nhận bằng GitNexus + đọc trực tiếp mã nguồn** (`impact({target: "ghExecFileAsync", direction: "upstream"})`, disambiguated về `agent/src/main/git/runner.ts:ghExecFileAsync`):

```
impactedCount: 0, direct: 0, risk: LOW
```

`grep -rln "ghExecFileAsync|glabExecFileAsync" agent/` → **chỉ khớp `agent/src/main/git/runner.ts` chính nó** — không một file nào khác trong `agent/` (kể cả `agent/src/main/text-generation/commit-message-text-generation.ts`) import hay gọi hai hàm này.

**Điều chỉnh so với bug report:** bug report ghi "chỉ một caller duy nhất: `commit-message-text-generation.ts`". Thực tế tại thời điểm audit này (`agent/src/main/text-generation/commit-message-text-generation.ts` hiện chỉ import `wslAwareSpawn` từ `runner.ts`, không còn import `ghExecFileAsync`) — **breaker hiện là dead code hoàn toàn, 0 caller**, tệ hơn mức bug report mô tả. Điều này củng cố thêm mức độ ưu tiên của fix, không làm giảm nó — không cần đổi mức độ 🟠 High.

Toàn bộ đường PR/MR/issue thật (`external-api-connector.ts`, `agent-git-handler.ts`, `git-handler.ts`) tự dựng `spawn`/`execFile` riêng (`execFileCaptured()` trong `external-api-connector.ts`, `spawn()` trực tiếp trong `agent-git-handler.ts`) — không đi qua `ghExecFileAsync`/`glabExecFileAsync`, nên hoàn toàn bypass:
- Circuit breaker chống rate-limit (`classifyGhRateLimitBucket`/`getGhRateLimitBlockedUntilMs`/`notifyGhPrimaryRateLimit`)
- Retry transient 5xx/429 với backoff + idempotency-aware retry gate
- WSL/host command routing (`resolveCommand`)

**Trở ngại kỹ thuật khi route qua `runner.ts`:** `ghExecFileAsync`/`glabExecFileAsync` có hợp đồng khác `execFileCaptured`:

| | `execFileCaptured` (external-api-connector.ts, hiện tại) | `ghExecFileAsync`/`glabExecFileAsync` (runner.ts) |
|---|---|---|
| Lỗi | **không throw** — trả `{ stdout, stderr, exitCode }`, exitCode ≠ 0 khi lỗi | **throw** (giống `execFileAsync`), lỗi mang `.stderr`/`.stdout` |
| Timeout hết hạn | trả `exitCode: 124` | throw lỗi timeout của `execFile` |
| Retry | không có | có, tự động cho lỗi transient + idempotent call |
| WSL routing | không có | có (`resolveCommand`) |
| Rate-limit breaker | không có | có (chỉ `gh`, không áp dụng `glab`) |

→ Mọi call site đổi sang `ghExecFileAsync`/`glabExecFileAsync` phải đổi từ `if (result.exitCode !== 0)` sang `try/catch` + `extractExecError(err)`.

## Chiến Lược: Chia Nhỏ Theo Phase

Effort tổng (8-14h) quá lớn để làm một lần — bug report cũng gợi ý "chia nhỏ theo file để làm dần". Đề xuất 3 phase, ưu tiên đúng theo yêu cầu: `github.pr.create`/`gitlab.mr.create` trước (nhiều fan-out nhất — mỗi task/worktree hoàn thành có thể tạo 1 PR đồng loạt).

| Phase | File | RPC methods | Effort |
|-------|------|--------------|--------|
| **1 (ưu tiên)** | `external-api-connector.ts` | `github.pr.create`, `gitlab.mr.create` | 2-3h |
| 2 | `external-api-connector.ts` (phần còn lại) | `github.pr.merge`, `github.issue.list/create`, `github.auth.status`, `gitlab.mr.list`, `gitlab.pipeline.status`, `gitlab.auth.status` | 3-4h |
| 3 | `agent-git-handler.ts`, `git-handler.ts` | mọi `spawn('git', ...)` gọi `gh`/`glab` gián tiếp (không áp dụng — các file này gọi `git` binary, không gọi `gh`/`glab` trực tiếp; xem ghi chú) | 3-5h |

> **Ghi chú Phase 3:** đọc lại `agent-git-handler.ts` và `git-handler.ts` xác nhận **không có lệnh gọi `gh`/`glab` nào** trong hai file này (chỉ gọi binary `git`) — khác với mô tả trong bug report evidence block ("`agent/src/relay/git-handler.ts`, `agent-git-handler.ts`, `external-api-connector.ts` → tự execFile/spawn riêng"). `git-handler.ts`/`agent-git-handler.ts` chỉ chạy `git`, không phải `gh`/`glab`, nên breaker của `runner.ts` (chỉ bảo vệ `gh`/`glab`, không bảo vệ `git`) không áp dụng ở đây. **Điều chỉnh phạm vi:** Phase 3 không cần thiết — toàn bộ bề mặt `gh`/`glab` thực sự nằm gọn trong `external-api-connector.ts` (Phase 1 + 2 là đủ để đóng bug này hoàn toàn).

## Thay Đổi Cần Thực Hiện (Phase 1 — chi tiết)

### 1. `agent/src/relay/external-api-connector.ts` — helper adapter `execGhCaptured`/`execGlabCaptured`

Giữ nguyên `ExecResult { stdout, stderr, exitCode }` contract cho phần còn lại của file (Phase 2 chưa đổi) bằng một adapter mỏng bọc `ghExecFileAsync`/`glabExecFileAsync`, dịch throw → `{exitCode}`:

```typescript
// agent/src/relay/external-api-connector.ts
import { ghExecFileAsync, glabExecFileAsync } from '../main/git/runner'
import { extractExecError } from '../main/git/runner'

// ─── gh/glab via runner.ts (circuit breaker + retry) ──────────────────────────
// Why: BUG-AG-HLD-005 — runner.ts's rate-limit breaker only protects calls
// that go through ghExecFileAsync/glabExecFileAsync. Adapts their throw-on-
// nonzero-exit contract back to this file's { stdout, stderr, exitCode }
// shape so callers below don't need a second error-handling style.
async function execGhCaptured(
  args: string[],
  opts: { cwd: string; env: NodeJS.ProcessEnv; timeout: number; idempotent?: boolean }
): Promise<ExecResult> {
  try {
    const { stdout, stderr } = await ghExecFileAsync(args, {
      cwd: opts.cwd, env: opts.env, timeout: opts.timeout, idempotent: opts.idempotent
    })
    return { stdout, stderr, exitCode: 0 }
  } catch (err: unknown) {
    const { stdout, stderr } = extractExecError(err)
    return { stdout, stderr, exitCode: 1 }
  }
}

async function execGlabCaptured(
  args: string[],
  opts: { cwd: string; env: NodeJS.ProcessEnv; timeout: number; idempotent?: boolean }
): Promise<ExecResult> {
  try {
    const { stdout, stderr } = await glabExecFileAsync(args, {
      cwd: opts.cwd, env: opts.env, timeout: opts.timeout, idempotent: opts.idempotent
    })
    return { stdout, stderr, exitCode: 0 }
  } catch (err: unknown) {
    const { stdout, stderr } = extractExecError(err)
    return { stdout, stderr, exitCode: 1 }
  }
}
```

### 2. `handleGitHubPrCreate()` — dùng `execGhCaptured` cho lệnh `gh pr create` (không đổi `git rev-parse`/`gh pr list` idempotency probe ở Phase 1, xem Phase 2)

```diff
   const ghArgs = [
     'pr', 'create',
     '--title', title,
     '--body',  body,
     '--base',  base,
     '--json',  'url,number,title,state',
   ]
   if (draft) ghArgs.push('--draft')

   try {
-    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
+    // BUG-AG-HLD-005: route through runner.ts's ghExecFileAsync so the
+    // primary-rate-limit circuit breaker protects PR creation — the RPC
+    // path most likely to fan out (one call per finished task/worktree).
+    // `gh pr create` is a write — never auto-retried as idempotent.
+    const result = await execGhCaptured(ghArgs, { cwd, env, timeout: 30_000, idempotent: false })
     if (result.exitCode !== 0) {
       log.error(`github.pr.create failed: ${result.stderr}`)
       span.fail(result.stderr || 'gh pr create failed', { method: 'github.pr.create', exitCode: result.exitCode })
       return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr create failed' } }
     }
```

### 3. `handleGitLabMrCreate()` — dùng `execGlabCaptured` cho lệnh `glab mr create`

```diff
   const env = buildGlabEnv(userId, config.toolEnv)

   try {
-    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
+    // BUG-AG-HLD-005: route through runner.ts's glabExecFileAsync — retry +
+    // WSL-aware resolution, mirroring the gh path. `glab mr create` is a
+    // write — never auto-retried as idempotent.
+    const result = await execGlabCaptured(glabArgs, { cwd, env, timeout: 30_000, idempotent: false })
     if (result.exitCode !== 0) {
       span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
       return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
     }
```

> `execFileCaptured` (spawn-based, dùng bởi `checkExistingPr`/`getCurrentBranch`/mọi handler khác chưa migrate) **giữ nguyên** — không xoá, Phase 2 sẽ migrate phần còn lại.

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/external-api-connector.test.ts
npx vitest run src/main/git/__tests__/runner.test.ts
npx vitest run src/main/git/__tests__/gh-rate-limit-breaker.test.ts
```

Test case mới:
- Mock `notifyGhPrimaryRateLimit`/`getGhRateLimitBlockedUntilMs`: khi bucket `core` đang blocked, `handleGitHubPrCreate` trả `AgentErrorCode.ServerError` **ngay lập tức không spawn `gh`** (assert spy trên `child_process.execFile` không được gọi).
- `handleGitHubPrCreate` với `idempotent: false` không tự retry khi `gh` trả lỗi 502/503 giả lập (khác với các lệnh đọc, vốn được retry theo `argsLookIdempotent`).
- `handleGitLabMrCreate` tương tự cho `glab`.
- Regression: idempotency check hiện có (`checkExistingPr` → PR đã tồn tại) vẫn hoạt động không đổi (Phase 1 không chạm đường này).

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận blast radius chỉ nằm trong `external-api-connector.ts` + import mới từ `runner.ts`, không lan ra `agent-git-handler.ts`/`git-handler.ts` (đúng như kết luận "Phase 3 không cần thiết" ở trên).

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/external-api-connector.ts` | Phase 1: `handleGitHubPrCreate`/`handleGitLabMrCreate` đổi sang `execGhCaptured`/`execGlabCaptured`; Phase 2: migrate 6 handler còn lại |
| `agent/src/main/git/runner.ts` | Nguồn `ghExecFileAsync`/`glabExecFileAsync`/`extractExecError` — không đổi, chỉ thêm caller |
| `agent/src/main/git/gh-rate-limit-breaker.ts` | Không đổi — breaker logic đã đúng, chỉ thiếu người gọi (chính là bug này) |
| `agent/src/main/text-generation/commit-message-text-generation.ts` | Ghi chú: KHÔNG còn là caller của `ghExecFileAsync` hiện tại (khác bug report — xem "Điều chỉnh so với bug report" ở trên); không cần sửa file này |
| `agent/src/relay/agent-git-handler.ts`, `agent/src/relay/git-handler.ts` | Đã xác nhận không gọi `gh`/`glab` trực tiếp — ngoài phạm vi fix này |
| Liên quan | BUG-AG-HLD-004 (hợp nhất `git.pr.create`→`handleGitHubPrCreate` trước — nên làm SOL-AG-HLD-004 trước SOL-AG-HLD-005 để tránh sửa 2 implementation PR-create song song) |
