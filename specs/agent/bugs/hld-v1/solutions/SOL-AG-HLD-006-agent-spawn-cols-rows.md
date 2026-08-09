# SOL-AG-HLD-006 — Nhận `cols`/`rows` từ Caller Thay Vì Hardcode PTY 220×50

**Fixes:** [BUG-AG-HLD-006](../BUG-AG-HLD-006-agent-spawn-hardcoded-pty-size.md)
**TDD Ref:** [TDD-AG-12 §7 — `handleAgentSpawn()` RPC Handler](../../../tdd/v5/12-agent-spawner.md) (đoạn `nodePty.spawn(..., { cols: 220, rows: 50 })`)
**File:** `agent/src/relay/agent-spawner.ts`
**Effort:** 1-2 giờ
**Status:** 🔴 TODO

---

## Phân Tích

Đọc nguyên văn `agent/src/relay/agent-spawner.ts` (code thật, không phải TDD mô tả):

- `AgentSpawnRequest` (dòng 59-68) chỉ có `taskId, userId, modelId, accountId, cwd, resumeId, worktreePath, branchName` — **không có `cols`/`rows`**.
- `handleAgentSpawn()` parse `req` từ `params` (dòng 276-285) — cũng không đọc `params.cols`/`params.rows`.
- `nodePty.spawn()` được gọi ở dòng 387-392 với size hardcode:

```ts
const pty = nodePty.spawn(spec.binary, args, {
  name: 'xterm-256color',
  cols: 220, rows: 50,
  cwd:  req.cwd ?? config.workDir,
  env,
})
```

Không có RPC `agent.resize` nào trong `agent-rpc-dispatch.ts` (đã kiểm tra toàn bộ `route()`) — khớp với quan sát trong bug report: không có cách resize PTY sau khi spawn. Việc thêm `agent.resize` là cải tiến riêng, **không** nằm trong scope effort 1-2h của fix này; ghi lại như đề xuất theo dõi ở cuối.

**Blast radius (GitNexus, `direction: upstream`):** `handleAgentSpawn` và `AGENT_SPECS` đều risk **LOW**, 0 caller tĩnh trong đồ thị gọi trực tiếp (lời gọi thật đến từ `agent-rpc-dispatch.ts`'s `route()` qua `await import('./agent-spawner')` — dynamic import nên không hiện thành cạnh tĩnh, nhưng đã xác nhận thủ công qua CodeGraph: `route()` case `'agent.spawn'` gọi `handleAgentSpawn` fire-and-forget). Thay đổi chỉ mở rộng field optional + thêm nhánh mặc định — không phá vỡ caller hiện có (backward-compatible).

**Lưu ý mirror:** `desktop/src/relay/agent-spawner.ts` là bản sao gần như y hệt file này trong package `desktop/` (cùng comment header, cùng cấu trúc `AGENT_SPECS`/`handleAgentSpawn`). Ngoài scope của nhiệm vụ này (chỉ sửa `agent/`), nhưng cần áp same patch ở đó để hai package không lệch hành vi — ghi vào "Files Liên Quan".

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-spawner.ts`

**1. Thêm `cols`/`rows` optional vào `AgentSpawnRequest`:**

```ts
export interface AgentSpawnRequest {
  taskId:        string
  userId:        string
  modelId:       string
  accountId:     string
  cwd?:          string
  resumeId?:     string  // ORCH-009: --resume <sessionId> for claude/codex
  worktreePath?: string  // WT-Issue-3: absolute path of worktree (usually same as cwd)
  branchName?:   string  // WT-Issue-3: git branch this worktree corresponds to
+ cols?:         number  // BUG-AG-HLD-006: real terminal width; falls back to DEFAULT_PTY_COLS
+ rows?:         number  // BUG-AG-HLD-006: real terminal height; falls back to DEFAULT_PTY_ROWS
}
```

**2. Thêm hằng số default (giữ đúng giá trị cũ để không đổi hành vi khi caller không truyền):**

```ts
// ── PTY size defaults ─────────────────────────────────────────────────────────
// BUG-AG-HLD-006: caller (Orca client) nên gửi cols/rows thật của terminal đang
// hiển thị agent panel. Giữ 220×50 làm fallback cho caller cũ chưa gửi field này.
const DEFAULT_PTY_COLS = 220
const DEFAULT_PTY_ROWS = 50
```

**3. Parse `cols`/`rows` trong `handleAgentSpawn()` (dòng ~276-285):**

```ts
  const req: AgentSpawnRequest = {
    taskId:        typeof params.taskId      === 'string' ? params.taskId      : '',
    userId:        typeof params.userId      === 'string' ? params.userId      : '',
    modelId,
    accountId:     typeof params.accountId   === 'string' ? params.accountId   : '',
    cwd:           typeof params.cwd         === 'string' ? params.cwd         : undefined,
    resumeId:      typeof params.resumeId    === 'string' ? params.resumeId    : undefined,
    worktreePath:  typeof params.worktreePath === 'string' ? params.worktreePath : undefined,
    branchName:    typeof params.branchName   === 'string' ? params.branchName   : undefined,
+   // BUG-AG-HLD-006: only accept positive integers — a malformed/negative value
+   // must not reach node-pty.spawn(), which throws on invalid cols/rows.
+   cols: Number.isInteger(params.cols) && (params.cols as number) > 0
+     ? (params.cols as number)
+     : undefined,
+   rows: Number.isInteger(params.rows) && (params.rows as number) > 0
+     ? (params.rows as number)
+     : undefined,
  }
```

**4. Truyền xuống `nodePty.spawn()` (dòng ~386-392):**

```ts
    orchSpan.step('node-pty-spawn', { binary: spec.binary, ptyId })
    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
-     cols: 220, rows: 50,
+     cols: req.cols ?? DEFAULT_PTY_COLS,
+     rows: req.rows ?? DEFAULT_PTY_ROWS,
      cwd:  req.cwd ?? config.workDir,
      env,
    })
```

Backward-compatible: caller cũ không gửi `cols`/`rows` → `undefined` → fallback đúng giá trị hardcode cũ (220×50), không có regression.

---

## Không Implement Trong Phase Này

- RPC `agent.resize` để đồng bộ kích thước PTY khi user resize panel **sau khi** agent đã spawn (bug report có đề cập nhưng đây là tính năng mới, không phải fix của bug hardcode-size — nên tách task riêng nếu cần).

---

## Verification

```bash
# Unit tests mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
# 1. handleAgentSpawn với params.cols=120, params.rows=40 → FakeAgentPty nhận đúng {cols:120, rows:40}
# 2. handleAgentSpawn KHÔNG có params.cols/rows → FakeAgentPty nhận {cols:220, rows:50} (giữ hành vi cũ)
# 3. params.cols = -5 hoặc "abc" (invalid) → fallback về DEFAULT_PTY_COLS, không throw

cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# Manual: spawn agent từ Orca UI trên cửa sổ nhỏ (vd chia 3 panel) → xác nhận
# Claude Code TUI redraw đúng width thay vì bị wrap sai.
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-spawner.ts` | `AgentSpawnRequest`, `handleAgentSpawn()`, `nodePty.spawn()` call — nơi sửa chính |
| `agent/src/relay/agent-rpc-dispatch.ts` | case `'agent.spawn'` — forward `rpc.params` nguyên vẹn xuống `handleAgentSpawn`, không cần sửa (đã truyền `params` object đầy đủ) |
| `desktop/src/relay/agent-spawner.ts` | Bản mirror của cùng module trong package `desktop/` — cần áp cùng patch để tránh lệch hành vi giữa 2 package (ngoài scope nhiệm vụ này, cần task riêng) |
| `agent/src/relay/__tests__/agent-spawner.test.ts` | Thêm test cases cols/rows |
| `docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:131-138` | Spec gốc kỳ vọng nhận cols/rows từ caller |
