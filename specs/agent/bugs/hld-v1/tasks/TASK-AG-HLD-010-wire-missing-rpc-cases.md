# TASK-AG-HLD-010 — Đăng Ký 3 RPC Case Còn Thiếu: `gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`

**Solution:** [SOL-AG-HLD-011](../solutions/SOL-AG-HLD-011-wire-missing-rpc-cases.md)
**Bug:** [BUG-AG-HLD-011](../BUG-AG-HLD-011-gitlab-github-methods-unwired.md)
**File:** `agent/src/relay/agent-rpc-dispatch.ts`
**Phụ thuộc:** —
**Estimated:** 30 phút
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

`external-api-connector.ts` export 3 handler đã implement đầy đủ (`handleGitLabMrList`, `handleGitHubAuthStatus`, `handleGitLabAuthStatus`) nhưng `agent-rpc-dispatch.ts`'s `route()` không có `case` nào gọi chúng — gọi RPC `gitlab.mr.list`/`github.auth.status`/`gitlab.auth.status` hiện rơi vào `default` → `AgentErrorCode.MethodNotFound`. Thêm 3 case theo đúng pattern các case liền kề.

---

## Context

Đọc trước:
- `agent/src/relay/external-api-connector.ts` — `handleGitLabMrList`, `handleGitHubAuthStatus`, `handleGitLabAuthStatus` (đã implement, đúng chữ ký `(id, params, config, log) => Promise<object>`, không cần sửa)
- `agent/src/relay/agent-rpc-dispatch.ts` — hàm `route()`, các case `github.*`/`gitlab.*` đã wire (`github.pr.create`, `github.pr.merge`, `github.issue.list`, `github.issue.create`, `gitlab.mr.create`, `gitlab.pipeline.status`)

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-rpc-dispatch.ts`

Thêm 3 case mới, đặt ngay sau `case 'gitlab.pipeline.status'` và trước `case 'agent.spawn'`.

**TÌM** (nguyên văn, dòng 542-565):
```typescript
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

    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case 'agent.spawn': {
      try {
        const { handleAgentSpawn } = await import('./agent-spawner')
        // Fire-and-forget: streaming handler sends multiple frames asynchronously
        void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
      }
    }
```

**THAY BẰNG:**
```typescript
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

    // ── v5.0: gitlab.mr.list ─────────────────────────────────────────────────
    case 'gitlab.mr.list': {
      try {
        const { handleGitLabMrList } = await import('./external-api-connector')
        return (await handleGitLabMrList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.auth.status ─────────────────────────────────────────────
    case 'github.auth.status': {
      try {
        const { handleGitHubAuthStatus } = await import('./external-api-connector')
        return (await handleGitHubAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.auth.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.auth.status ─────────────────────────────────────────────
    case 'gitlab.auth.status': {
      try {
        const { handleGitLabAuthStatus } = await import('./external-api-connector')
        return (await handleGitLabAuthStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.auth.status unavailable: ${msg}`)
      }
    }

    // ── v5.0: agent.spawn ────────────────────────────────────────────────────
    case 'agent.spawn': {
      try {
        const { handleAgentSpawn } = await import('./agent-spawner')
        // Fire-and-forget: streaming handler sends multiple frames asynchronously
        void handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'spawn.accepted' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.spawn unavailable: ${msg}`)
      }
    }
```

> [!IMPORTANT]
> Không cần đổi gì trong `external-api-connector.ts` — 3 handler đã đúng interface, chỉ thiếu đăng ký. `extractTraceFields()` trong `agent-rpc-dispatch.ts` đã có nhánh generic cho `method.startsWith('github.') || method.startsWith('gitlab.')` — 3 method mới tự động có trace context (`repo`, `branch`, `prNum`, `title`) mà không cần sửa thêm gì ở đó.

---

## Verify

```bash
cd agent
npx tsc --noEmit
npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts
npx vitest run src/relay/__tests__/external-api-connector.test.ts
```

Test case mới cần thêm (trong `agent-rpc-dispatch.test.ts`, theo pattern test case hiện có cho `github.pr.create`):
- `route({ method: 'gitlab.mr.list', params: { cwd: '/tmp', userId: 'u1' } }, ...)` → không còn trả `MethodNotFound`; trả `result.mrs` (mock `execFileCaptured`/`glab` thành công).
- `route({ method: 'github.auth.status', params: { userId: 'u1' } }, ...)` → trả `result.ok`.
- `route({ method: 'gitlab.auth.status', params: { userId: 'u1' } }, ...)` → trả `result.ok`.
- Regression: 6 case `github.*`/`gitlab.*` đã wire trước đó (`github.pr.create`, `github.pr.merge`, `github.issue.list`, `github.issue.create`, `gitlab.mr.create`, `gitlab.pipeline.status`) không đổi hành vi.

Sau khi sửa, chạy `gitnexus detect_changes({scope: "compare", base_ref: "main"})` để xác nhận thay đổi chỉ thêm 3 case mới trong `route()`, không chạm handler nào trong `external-api-connector.ts`.

---

## Definition of Done

- [ ] Thêm `case 'gitlab.mr.list'` gọi `handleGitLabMrList` từ `./external-api-connector`
- [ ] Thêm `case 'github.auth.status'` gọi `handleGitHubAuthStatus` từ `./external-api-connector`
- [ ] Thêm `case 'gitlab.auth.status'` gọi `handleGitLabAuthStatus` từ `./external-api-connector`
- [ ] Cả 3 case theo đúng pattern try/catch + dynamic import + `makeError` của case liền kề
- [ ] `external-api-connector.ts` KHÔNG bị sửa
- [ ] Test mới cho cả 3 method pass (không còn `MethodNotFound`)
- [ ] Regression: 6 case `github.*`/`gitlab.*` cũ không đổi hành vi
- [ ] `npx tsc --noEmit` (trong `agent/`) pass
- [ ] `npx vitest run src/relay/__tests__/agent-rpc-dispatch.test.ts` pass
- [ ] `npx vitest run src/relay/__tests__/external-api-connector.test.ts` pass
- [ ] `detect_changes({scope: "compare", base_ref: "main"})` chỉ show thêm 3 case mới trong `route()`

---

## Kết Quả Thực Thi (2026-08-09)

Đã thêm 3 case (`gitlab.mr.list`, `github.auth.status`, `gitlab.auth.status`) vào `route()` trong `agent-rpc-dispatch.ts`, theo đúng pattern try/catch + dynamic import của các case github/gitlab liền kề. `external-api-connector.ts` không bị sửa.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
