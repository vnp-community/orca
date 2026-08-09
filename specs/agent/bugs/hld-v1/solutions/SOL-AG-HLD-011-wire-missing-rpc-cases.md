# SOL-AG-HLD-011 — Đăng ký 3 RPC case còn thiếu: `gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`

**Fixes:** [BUG-AG-HLD-011](../BUG-AG-HLD-011-gitlab-github-methods-unwired.md)
**TDD Ref:** TDD-AG-13 §3.4 (All GitHub RPC handlers — `handleGitHubAuthStatus`), §4.2 (All GitLab RPC handlers — `handleGitLabMrList`, `handleGitLabAuthStatus`), §6 (RPC Method Registration pattern), §9 Implementation Checklist (3 mục cuối chưa tick)
**File:** `agent/src/relay/agent-rpc-dispatch.ts`
**Effort:** 0.5-1 giờ
**Status:** 🔴 TODO

---

## Phân Tích

Đọc trực tiếp `external-api-connector.ts` xác nhận cả 3 handler đã implement đầy đủ, đúng chữ ký `(id, params, config, log) => Promise<object>` giống mọi handler khác đã wire:

- `handleGitHubAuthStatus` — dòng 311-339
- `handleGitLabMrList` — dòng 394-423
- `handleGitLabAuthStatus` — dòng 460-488

Đọc trực tiếp `agent-rpc-dispatch.ts`'s `route()` xác nhận đúng như bug report: các case đã wire cho `external-api-connector.ts` là `github.pr.create` (488), `github.pr.merge` (499), `github.issue.list` (510), `github.issue.create` (521), `gitlab.mr.create` (532), `gitlab.pipeline.status` (543) — **6/9 handler xuất khẩu từ file này có case, 3 handler còn lại không có**. Không có `default` fallback nào âm thầm route các method này sang nơi khác — gọi RPC `gitlab.mr.list`/`github.auth.status`/`gitlab.auth.status` hiện tại rơi thẳng vào nhánh `default` cuối `route()` → `AgentErrorCode.MethodNotFound`.

GitNexus (`context({name: "handleGitLabMrList"})`, disambiguated về `agent/src/relay/external-api-connector.ts`) xác nhận hàm không có incoming call edge nào trong call graph tĩnh — nhất quán với việc chưa được gọi ở bất kỳ đâu (kể cả dynamic import), khác với 6 handler đã wire (những handler đó cũng không có static edge vì gọi qua `await import(...)`, nhưng ít nhất source đọc trực tiếp xác nhận có case gọi chúng — 3 handler này thì không có case nào cả). Risk: **LOW** — thêm case mới không đổi hành vi bất kỳ method nào đã tồn tại, chỉ mở thêm 3 method mới.

## Thay Đổi Cần Thực Hiện

### `agent/src/relay/agent-rpc-dispatch.ts` — thêm 3 case, đặt cạnh nhóm `github.*`/`gitlab.*` hiện có (sau dòng 551, trước `agent.spawn`)

Theo đúng pattern try/catch + dynamic import + `makeError` của 6 case liền kề (`github.pr.create` v.v., dòng 488-551):

```diff
     // ── v5.0: gitlab.pipeline.status ─────────────────────────────────────────
     case 'gitlab.pipeline.status': {
       try {
         const { handleGitLabPipelineStatus } = await import('./external-api-connector')
         return (await handleGitLabPipelineStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
       } catch (err: unknown) {
         const msg = err instanceof Error ? err.message : String(err)
         return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.pipeline.status unavailable: ${msg}`)
       }
     }

+    // ── v5.0: gitlab.mr.list ─────────────────────────────────────────────────
+    case 'gitlab.mr.list': {
+      try {
+        const { handleGitLabMrList } = await import('./external-api-connector')
+        return (await handleGitLabMrList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.list unavailable: ${msg}`)
+      }
+    }
+
+    // ── v5.0: github.auth.status ─────────────────────────────────────────────
+    case 'github.auth.status': {
+      try {
+        const { handleGitHubAuthStatus } = await import('./external-api-connector')
+        return (await handleGitHubAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `github.auth.status unavailable: ${msg}`)
+      }
+    }
+
+    // ── v5.0: gitlab.auth.status ─────────────────────────────────────────────
+    case 'gitlab.auth.status': {
+      try {
+        const { handleGitLabAuthStatus } = await import('./external-api-connector')
+        return (await handleGitLabAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.auth.status unavailable: ${msg}`)
+      }
+    }
+
     // ── v5.0: agent.spawn ────────────────────────────────────────────────────
     case 'agent.spawn': {
```

Không cần đổi gì trong `external-api-connector.ts` — 3 handler đã đúng interface, chỉ thiếu đăng ký.

`extractTraceFields()` (dòng 69-178) đã có nhánh generic cho `method.startsWith('github.') || method.startsWith('gitlab.')` (dòng 99-106) — 3 method mới tự động có trace context (`repo`, `branch`, `prNum`, `title`) mà không cần sửa thêm gì ở đó.

## Verification

```bash
cd agent
npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
npx vitest run src/relay/__tests__/external-api-connector.test.ts
```

Test case mới (trong `agent-rpc-dispatch.test.ts`, theo pattern test case hiện có cho `github.pr.create`):

- `route({ method: 'gitlab.mr.list', params: { cwd: '/tmp', userId: 'u1' } }, ...)` → không còn trả `MethodNotFound`; trả `result.mrs` (mock `execFileCaptured`/`glab` thành công).
- `route({ method: 'github.auth.status', params: { userId: 'u1' } }, ...)` → trả `result.ok`.
- `route({ method: 'gitlab.auth.status', params: { userId: 'u1' } }, ...)` → trả `result.ok`.
- Regression: 6 case `github.*`/`gitlab.*` đã wire trước đó không đổi hành vi.

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ thêm 3 case mới trong `route()`, không chạm handler nào trong `external-api-connector.ts`.

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-rpc-dispatch.ts` | Thêm 3 `case` mới vào `route()` |
| `agent/src/relay/external-api-connector.ts` | Chứa 3 handler đã implement sẵn — không cần sửa |
| `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts` | Thêm test case cho 3 method mới |
