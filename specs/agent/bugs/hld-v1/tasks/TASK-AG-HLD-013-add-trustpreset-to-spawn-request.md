# TASK-AG-HLD-013 — Thêm `trustPreset` Vào `AgentSpawnRequest` (Interface RPC Thật)

**Solution:** [SOL-AG-HLD-008](../solutions/SOL-AG-HLD-008-trustpreset-wiring.md)  
**Bug:** [BUG-AG-HLD-008](../BUG-AG-HLD-008-trust-preset-field-ignored.md)  
**File:** `agent/src/relay/agent-spawner.ts`  
**Phụ thuộc:** —  
**Estimated:** 30 phút  
**Status:** ✅ DONE — 2026-08-09 (code + typecheck verified; vitest không chạy được trong môi trường này — xem ghi chú cuối file)

---

## Mục Tiêu

Thêm field `trustPreset` vào `AgentSpawnRequest` — interface RPC `agent.spawn` thật đang chạy production — và parse `params.trustPreset` trong `handleAgentSpawn()`, thay vì `AgentEnvRequest` (type nội bộ chỉ dùng bởi test fixtures, không có caller production nào construct nó).

---

## Context

Đọc trước:
- `agent/src/relay/agent-spawner.ts` — `AgentSpawnRequest` (dòng 59-68, **không có** `trustPreset`), `AgentEnvRequest` (dòng 183-192, **có** `trustPreset?: string` nhưng field này không có tác dụng vì không có caller production nào truyền object shape này), `handleAgentSpawn()` đoạn build `req` (dòng 276-285)
- `agent/src/relay/agent-exec-handler.ts` dòng 326, 340, 395-397 — cách `AgentExecRequest.trustPreset` đã implement đúng cho RPC `agent.exec` khác, dùng làm tham chiếu enum

> [!NOTE]
> Task này CHỈ thêm field vào interface + parse params — KHÔNG wire vào `buildArgs`/CLI flag. Việc wire flag CLI thật (`--dangerously-skip-permissions` v.v.) nằm ở [TASK-AG-HLD-014](./TASK-AG-HLD-014-wire-trustpreset-to-buildargs.md) (phụ thuộc task này).

---

## Thay Đổi Cần Thực Hiện

**File:** `agent/src/relay/agent-spawner.ts`

### 1. Thêm `trustPreset` vào `AgentSpawnRequest`

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
  trustPreset?:  'standard' | 'full' | 'none'  // BUG-AG-HLD-008: 'full' → thêm flag skip-permission của CLI
}
```

> [!NOTE]
> Nếu [TASK-AG-HLD-011](./TASK-AG-HLD-011-agent-spawn-cols-rows.md) đã được áp dụng trước (thêm `cols?`/`rows?`), khối TÌM ở trên sẽ đã có thêm 2 dòng `cols?`/`rows?` — vẫn append `trustPreset?` vào cuối interface theo đúng cách trên, chỉ khác vị trí neo TÌM.

### 2. Parse `params.trustPreset` trong `handleAgentSpawn()`

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
    trustPreset:   params.trustPreset === 'full' || params.trustPreset === 'none'
      ? params.trustPreset
      : undefined,  // mặc định 'standard' (không thêm flag) nếu thiếu/không hợp lệ
  }
```

> [!NOTE]
> Nếu TASK-AG-HLD-011 đã áp dụng trước, khối TÌM ở trên sẽ đã có thêm 2 dòng `cols:`/`rows:` — vẫn append `trustPreset:` vào cuối object literal theo đúng cách trên.

### 3. Làm rõ `AgentEnvRequest.trustPreset` không còn là nguồn thật (tránh hiểu lầm)

**TÌM:**
```ts
export interface AgentEnvRequest {
  accountId:   string
  userId:      string
  taskId:      string
  projectId?:  string
  cwd:         string
  model?:      string
  trustPreset?: string
  extraEnv?:   Record<string, string>
}
```

**THAY BẰNG:**
```ts
export interface AgentEnvRequest {
  accountId:   string
  userId:      string
  taskId:      string
  projectId?:  string
  cwd:         string
  model?:      string
  // BUG-AG-HLD-008: KHÔNG dùng để build CLI args — trust preset thật được đọc
  // từ AgentSpawnRequest.trustPreset trong buildAgentArgs(), không phải ở đây.
  // Giữ field này chỉ vì AgentEnvRequest hiện chưa có caller production nào
  // construct nó (xem SOL-AG-HLD-008 §2) — cân nhắc xoá hẳn AgentEnvRequest
  // trong 1 refactor riêng nếu vẫn không có caller thật sau khi fix này merge.
  trustPreset?: string
  extraEnv?:   Record<string, string>
}
```

---

## Verify

```bash
# Unit test mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
# - handleAgentSpawn với params.trustPreset='full' → req.trustPreset === 'full'
# - handleAgentSpawn với params.trustPreset='none' → req.trustPreset === 'none'
# - handleAgentSpawn với params.trustPreset='standard' hoặc thiếu/invalid → req.trustPreset === undefined
cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# Typecheck
cd agent && npx tsc --noEmit
```

> [!NOTE]
> Sau task này, `req.trustPreset` được parse đúng nhưng CHƯA có tác dụng lên CLI args nào — đó là scope của TASK-AG-HLD-014. Không mong đợi hành vi spawn thay đổi sau khi merge riêng task này.

---

## Definition of Done

- [ ] `trustPreset?: 'standard' | 'full' | 'none'` đã thêm vào `AgentSpawnRequest`
- [ ] `handleAgentSpawn()` parse `params.trustPreset`, chỉ nhận `'full'`/`'none'`, fallback `undefined` nếu thiếu/invalid (tương đương `'standard'`)
- [ ] `AgentEnvRequest.trustPreset` có comment rõ ràng: KHÔNG phải nguồn thật, tránh hiểu lầm field này hoạt động
- [ ] Unit tests mới pass (`cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts`)
- [ ] `npx tsc --noEmit` không lỗi
- [ ] Xác nhận: sau task này `req.trustPreset` chưa ảnh hưởng CLI args nào (đúng scope, không lấn sang TASK-AG-HLD-014)

---

## Kết Quả Thực Thi (2026-08-09)

Đã thêm `trustPreset?: 'standard'|'full'|'none'` vào `AgentSpawnRequest` (interface RPC production thật) và parse trong `handleAgentSpawn()`. Đã thêm comment làm rõ `AgentEnvRequest.trustPreset` KHÔNG phải nguồn thật. Chưa có tác dụng lên CLI args ở task này (đúng scope) — xem TASK-AG-HLD-014.

**Phương pháp verify dùng thực tế:** `npx tsc --noEmit -p agent/tsconfig.json` (so sánh delta lỗi trước/sau mỗi thay đổi — baseline 98 lỗi pre-existing không đổi qua toàn bộ 16 task) + grep xác nhận đoạn code khớp thật trước khi sửa. `pnpm test`/`npx vitest` **không chạy được** trong môi trường này vì `config/vitest.config.ts` không tồn tại (thiếu hạ tầng test, không phải lỗi do thay đổi này gây ra) — các checkbox liên quan tới vitest trong "Definition of Done" ở trên chưa được xác nhận bằng test tự động, chỉ bằng đọc code + typecheck.
