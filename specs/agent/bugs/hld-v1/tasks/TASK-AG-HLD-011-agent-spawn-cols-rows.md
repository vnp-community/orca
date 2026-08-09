# TASK-AG-HLD-011 — Nhận `cols`/`rows` Từ Caller Thay Vì Hardcode PTY 220×50

**Solution:** [SOL-AG-HLD-006](../solutions/SOL-AG-HLD-006-agent-spawn-cols-rows.md)  
**Bug:** [BUG-AG-HLD-006](../BUG-AG-HLD-006-agent-spawn-hardcoded-pty-size.md)  
**File:** `agent/src/relay/agent-spawner.ts`  
**Phụ thuộc:** —  
**Estimated:** 60 phút  
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Thêm `cols`/`rows` optional vào RPC `agent.spawn`, truyền xuống `node-pty.spawn()`, giữ nguyên fallback 220×50 khi caller không gửi field này.

---

## Context

Đọc trước:
- `agent/src/relay/agent-spawner.ts` — `AgentSpawnRequest` (dòng 59-68), `handleAgentSpawn()` (dòng 261-451), đặc biệt đoạn build `req` (dòng 276-285) và lời gọi `nodePty.spawn()` (dòng 386-392)

Không cần sửa `agent/src/relay/agent-rpc-dispatch.ts` — case `'agent.spawn'` đã forward `rpc.params` nguyên vẹn xuống `handleAgentSpawn(rpc.id, rpc.params ?? {}, config, log, ws, state)`, không cần đổi gì ở dispatcher.

---

## Thay Đổi Cần Thực Hiện

**File:** `agent/src/relay/agent-spawner.ts`

### 1. Thêm `cols`/`rows` optional vào `AgentSpawnRequest`

**TÌM:**
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
}
```

**THAY BẰNG:**
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
  cols?:         number  // BUG-AG-HLD-006: real terminal width; falls back to DEFAULT_PTY_COLS
  rows?:         number  // BUG-AG-HLD-006: real terminal height; falls back to DEFAULT_PTY_ROWS
}
```

### 2. Thêm hằng số default (giữ đúng giá trị hardcode cũ làm fallback)

**TÌM:**
```ts
// ── resolveAgentSpec (pure, testable) ─────────────────────────────────────────
//
// ORCH-012: Removed invalid --no-cache flag from claude args.
// ORCH-004: Added codex (gpt-* prefix), opencode, and ollama (local inference).
// Uses prefix-matching so claude-opus-4, gpt-4o, gemini-2.0 all resolve correctly.

const AGENT_SPECS: AgentBinarySpec[] = [
```

**THAY BẰNG:**
```ts
// ── PTY size defaults ─────────────────────────────────────────────────────────
// BUG-AG-HLD-006: caller (Orca client) nên gửi cols/rows thật của terminal đang
// hiển thị agent panel. Giữ 220×50 làm fallback cho caller cũ chưa gửi field này.
const DEFAULT_PTY_COLS = 220
const DEFAULT_PTY_ROWS = 50

// ── resolveAgentSpec (pure, testable) ─────────────────────────────────────────
//
// ORCH-012: Removed invalid --no-cache flag from claude args.
// ORCH-004: Added codex (gpt-* prefix), opencode, and ollama (local inference).
// Uses prefix-matching so claude-opus-4, gpt-4o, gemini-2.0 all resolve correctly.

const AGENT_SPECS: AgentBinarySpec[] = [
```

### 3. Parse `cols`/`rows` trong `handleAgentSpawn()`

**TÌM:**
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
  }
```

**THAY BẰNG:**
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
    // BUG-AG-HLD-006: only accept positive integers — a malformed/negative value
    // must not reach node-pty.spawn(), which throws on invalid cols/rows.
    cols: Number.isInteger(params.cols) && (params.cols as number) > 0
      ? (params.cols as number)
      : undefined,
    rows: Number.isInteger(params.rows) && (params.rows as number) > 0
      ? (params.rows as number)
      : undefined,
  }
```

### 4. Truyền xuống `nodePty.spawn()`

**TÌM:**
```ts
    orchSpan.step('node-pty-spawn', { binary: spec.binary, ptyId })
    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd:  req.cwd ?? config.workDir,
      env,
    })
```

**THAY BẰNG:**
```ts
    orchSpan.step('node-pty-spawn', { binary: spec.binary, ptyId })
    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: req.cols ?? DEFAULT_PTY_COLS,
      rows: req.rows ?? DEFAULT_PTY_ROWS,
      cwd:  req.cwd ?? config.workDir,
      env,
    })
```

> [!IMPORTANT]
> Backward-compatible: caller cũ không gửi `cols`/`rows` → field là `undefined` → fallback về đúng giá trị hardcode cũ (220×50). Không có regression cho caller hiện tại.

---

## Verify

```bash
# 1. Unit tests mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
#    - handleAgentSpawn với params.cols=120, params.rows=40 → FakeAgentPty nhận đúng {cols:120, rows:40}
#    - handleAgentSpawn KHÔNG có params.cols/rows → FakeAgentPty nhận {cols:220, rows:50} (giữ hành vi cũ)
#    - params.cols = -5 hoặc "abc" (invalid) → fallback về DEFAULT_PTY_COLS, không throw
cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# 2. Typecheck
cd agent && npx tsc --noEmit

# 3. Manual: spawn agent từ Orca UI trên cửa sổ nhỏ (vd chia 3 panel) → xác nhận
#    Claude Code TUI redraw đúng width thay vì bị wrap sai.
```

---

## Definition of Done

- [ ] `cols?`/`rows?` đã thêm vào `AgentSpawnRequest`
- [ ] `DEFAULT_PTY_COLS`/`DEFAULT_PTY_ROWS` = 220/50 (giữ đúng giá trị cũ)
- [ ] `handleAgentSpawn()` parse `params.cols`/`params.rows`, chỉ nhận positive integer, fallback `undefined` nếu invalid
- [ ] `nodePty.spawn()` dùng `req.cols ?? DEFAULT_PTY_COLS` và `req.rows ?? DEFAULT_PTY_ROWS`
- [ ] Unit tests mới pass (`cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts`)
- [ ] `npx tsc --noEmit` không lỗi
- [ ] Không có regression: caller cũ (không gửi cols/rows) vẫn spawn PTY 220×50 như trước

---

## Kết Quả Thực Thi (2026-08-09)

Đã thêm `cols?`/`rows?` vào `AgentSpawnRequest`, hằng số `DEFAULT_PTY_COLS/ROWS = 220/50`, parse + validate (positive integer) trong `handleAgentSpawn()`, và truyền vào `nodePty.spawn()`. Backward-compatible: caller không gửi field vẫn nhận PTY 220×50 như cũ.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
