# TASK-AG-HLD-014 — Wire `trustPreset` Vào `buildAgentArgs()` Bằng Flag Đã Verify

**Solution:** [SOL-AG-HLD-008](../solutions/SOL-AG-HLD-008-trustpreset-wiring.md)  
**Bug:** [BUG-AG-HLD-008](../BUG-AG-HLD-008-trust-preset-field-ignored.md)  
**File:** `agent/src/relay/agent-spawner.ts`  
**Phụ thuộc:** [TASK-AG-HLD-013](./TASK-AG-HLD-013-add-trustpreset-to-spawn-request.md) (cần `AgentSpawnRequest.trustPreset` tồn tại trước)  
**Estimated:** 90 phút (bao gồm smoke test thật cho opencode — xem Definition of Done)  
**Status:** 🟡 CODE DONE — 2026-08-09 (typecheck verified; **smoke test BẮT BUỘC với binary CLI thật CHƯA chạy được** — môi trường này không có opencode/gemini)

---

## Mục Tiêu

Wire `req.trustPreset` vào `buildAgentArgs()`/`AGENT_SPECS` để mỗi agent thêm đúng flag CLI skip-permission đã verify trong `YOLO_TUI_AGENT_ARGS` khi `trustPreset === 'full'`.

---

## Context

Đọc trước:
- `agent/src/relay/agent-spawner.ts` — `AgentBinarySpec` (dòng 52-57), `AGENT_SPECS` (dòng 119-142), `buildAgentArgs()` (dòng 165-167). **Bắt buộc đã hoàn thành [TASK-AG-HLD-013](./TASK-AG-HLD-013-add-trustpreset-to-spawn-request.md)** — task đó thêm field `trustPreset` vào `AgentSpawnRequest`, task này chỉ forward field đó vào `buildArgs`.
- `agent/src/shared/tui-agent-permissions.ts` — `YOLO_TUI_AGENT_ARGS` (dòng 6-31): nguồn flag skip-permission đã verify, dùng thật cho TUI launcher (claude/codex/gemini/…)
- `agent/src/shared/tui-agent-launch-defaults.ts` dòng 6 — `opencode: ['--dangerously-skip-permissions']`: flag thật của opencode dùng ở launcher khác trong repo (KHÔNG nằm trong `YOLO_TUI_AGENT_ARGS`, độ tin cậy trung bình — xem Phương Án Dự Phòng)
- `agent/src/relay/agent-exec-handler.ts` dòng 395-397 — pattern tham chiếu: `AgentExecRequest.trustPreset` đã implement tương tự cho RPC `agent.exec`

> [!NOTE]
> Nếu [TASK-AG-HLD-012](./TASK-AG-HLD-012-resume-opencode-gemini.md) (SOL-AG-HLD-007) đã được áp dụng trước, entry `gemini`/`opencode` trong `AGENT_SPECS` sẽ đã ở dạng object-literal với nhánh `resumeId` mới (`--resume`/`--session`) thay vì baseline dưới đây. Trong trường hợp đó, KHÔNG dùng khối TÌM/THAY BẰNG bên dưới cho 2 entry đó — chỉ thêm dòng `if (req?.trustPreset === 'full') args.push(...)` vào đúng vị trí trong `buildArgs` đã có của chúng (xem chú thích IMPORTANT ở bước 3).

---

## Thay Đổi Cần Thực Hiện

**File:** `agent/src/relay/agent-spawner.ts`

### 1. Import map flag đã verify

**TÌM:**
```ts
import { encodeDataFrame, createWireState } from './agent-wire'
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'
import { readDecryptedKey } from './agent-credential-store'
```

**THAY BẰNG:**
```ts
import { encodeDataFrame, createWireState } from './agent-wire'
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'
import { readDecryptedKey } from './agent-credential-store'
import { YOLO_TUI_AGENT_ARGS } from '../shared/tui-agent-permissions'
```

### 2. Mở rộng signature `AgentBinarySpec.buildArgs`

**TÌM:**
```ts
export interface AgentBinarySpec {
  readonly binary:         string
  readonly buildArgs:      (req?: { resumeId?: string }) => string[]
  readonly apiKeyEnvVar:   string | null
  readonly localInference?: boolean
}
```

**THAY BẰNG:**
```ts
export interface AgentBinarySpec {
  readonly binary:         string
  readonly buildArgs:      (req?: { resumeId?: string; trustPreset?: 'standard' | 'full' | 'none' }) => string[]
  readonly apiKeyEnvVar:   string | null
  readonly localInference?: boolean
}
```

### 3. Sửa `AGENT_SPECS` — thêm nhánh trust cho claude/codex/gemini/opencode; ollama không đổi

**TÌM** (baseline — xem ghi chú ở Context nếu TASK-AG-HLD-012 đã áp dụng trước):
```ts
const AGENT_SPECS: AgentBinarySpec[] = [
  // index 0: claude — output-format stream-json for automation; --verbose for tracing
  {
    binary: 'claude',
    buildArgs: (req) => req?.resumeId
      ? ['--resume', req.resumeId]
      : ['--output-format', 'stream-json', '--verbose'],
    apiKeyEnvVar: 'ANTHROPIC_API_KEY',
  },
  // index 1: codex / openai compatible
  {
    binary: 'codex',
    buildArgs: (req) => req?.resumeId
      ? ['--session-file', `~/.codex/${req.resumeId}.json`]
      : [],
    apiKeyEnvVar: 'OPENAI_API_KEY',
  },
  // index 2: gemini
  { binary: 'gemini',   buildArgs: () => ['--stream'], apiKeyEnvVar: 'GEMINI_API_KEY' },
  // index 3: opencode — no API key needed (uses its own auth)
  { binary: 'opencode', buildArgs: () => [],            apiKeyEnvVar: null },
  // index 4: ollama — local inference, no external API key
  { binary: 'ollama',   buildArgs: () => [],            apiKeyEnvVar: null, localInference: true },
]
```

**THAY BẰNG:**
```ts
const AGENT_SPECS: AgentBinarySpec[] = [
  // index 0: claude — output-format stream-json for automation; --verbose for tracing
  {
    binary: 'claude',
    buildArgs: (req) => {
      const args = req?.resumeId
        ? ['--resume', req.resumeId]
        : ['--output-format', 'stream-json', '--verbose']
      // BUG-AG-HLD-008: flag verified trong YOLO_TUI_AGENT_ARGS (dùng thật cho TUI launcher)
      if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.claude!)
      return args
    },
    apiKeyEnvVar: 'ANTHROPIC_API_KEY',
  },
  // index 1: codex / openai compatible
  {
    binary: 'codex',
    buildArgs: (req) => {
      const args = req?.resumeId
        ? ['--session-file', `~/.codex/${req.resumeId}.json`]
        : []
      if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.codex!)
      return args
    },
    apiKeyEnvVar: 'OPENAI_API_KEY',
  },
  // index 2: gemini
  {
    binary: 'gemini',
    buildArgs: (req) => {
      const args = req?.resumeId ? [] : ['--stream']
      if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.gemini!)
      return args
    },
    apiKeyEnvVar: 'GEMINI_API_KEY',
  },
  // index 3: opencode — no API key needed (uses its own auth). Flag verified ở
  // tui-agent-launch-defaults.ts (độ tin cậy trung bình — chưa verify là "theo
  // preset", chỉ biết CLI chấp nhận flag) — xem Phương Án Dự Phòng nếu smoke test fail.
  {
    binary: 'opencode',
    buildArgs: (req) => {
      const args: string[] = []
      if (req?.trustPreset === 'full') args.push('--dangerously-skip-permissions')
      return args
    },
    apiKeyEnvVar: null,
  },
  // index 4: ollama — KHÔNG thêm nhánh trustPreset: local inference không có
  // permission prompt, no-op có chủ đích (không phải thiếu sót).
  { binary: 'ollama', buildArgs: () => [], apiKeyEnvVar: null, localInference: true },
]
```

> [!IMPORTANT]
> Nếu TASK-AG-HLD-012 đã áp dụng trước, entry `gemini` phải giữ nhánh `req?.resumeId ? ['--resume', req.resumeId] : ['--stream']` (không phải `req?.resumeId ? [] : ['--stream']` như baseline ở trên), và entry `opencode` phải giữ nhánh `req?.resumeId ? ['--session', req.resumeId] : []`. Chỉ thêm dòng `if (req?.trustPreset === 'full') args.push(...)` vào SAU nhánh resumeId hiện có của mỗi entry, không xoá nhánh đó.

### 4. Forward `trustPreset` trong `buildAgentArgs()`

**TÌM:**
```ts
function buildAgentArgs(spec: AgentBinarySpec, req: AgentSpawnRequest): string[] {
  return spec.buildArgs({ resumeId: req.resumeId })
}
```

**THAY BẰNG:**
```ts
function buildAgentArgs(spec: AgentBinarySpec, req: AgentSpawnRequest): string[] {
  return spec.buildArgs({ resumeId: req.resumeId, trustPreset: req.trustPreset })
}
```

---

## Phương Án Dự Phòng (Nếu Smoke Test Opencode Thất Bại)

Flag `--dangerously-skip-permissions` cho opencode có độ tin cậy THẤP HƠN claude/codex/gemini (chưa verify là "hoạt động theo preset trong context headless PTY", chỉ biết CLI chấp nhận flag ở launcher khác). Nếu smoke test opencode fail:

1. Tạm bỏ nhánh `trustPreset` khỏi entry `opencode` trong `AGENT_SPECS` (revert riêng phần đó về `buildArgs: (req) => req?.resumeId ? ['--session', req.resumeId] : []` hoặc `[]` tuỳ baseline).
2. Giữ nguyên nhánh `trustPreset` cho claude/codex/gemini (đã verify độ tin cậy cao hơn qua `YOLO_TUI_AGENT_ARGS`, không cần revert).
3. Ghi chú trong PR: opencode `trustPreset='full'` tạm thời no-op, cần task riêng để tìm flag đúng.

---

## Verify

```bash
# 1. Unit tests mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
#    - resolveAgentSpec('claude').buildArgs({trustPreset: 'full'}) chứa '--dangerously-skip-permissions'
#    - resolveAgentSpec('codex').buildArgs({trustPreset: 'full'}) chứa '--dangerously-bypass-approvals-and-sandbox'
#    - resolveAgentSpec('gemini').buildArgs({trustPreset: 'full'}) chứa '--yolo'
#    - resolveAgentSpec('opencode').buildArgs({trustPreset: 'full'}) chứa '--dangerously-skip-permissions'
#    - resolveAgentSpec('ollama').buildArgs({trustPreset: 'full'}) === [] (no-op xác nhận có chủ đích)
#    - trustPreset undefined/'standard'/'none' → KHÔNG thêm flag cho cả 5 agent (regression check)
#    - handleAgentSpawn với params.trustPreset='full' → FakeAgentPty nhận đúng args có flag
cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# 2. Typecheck (đồng thời xác nhận test fixtures claudeSpec/ollamaSpec trong
#    agent-spawner.test.ts dòng 239, 241 không vỡ do đổi signature buildArgs)
cd agent && npx tsc --noEmit

# 3. Smoke test THẬT cho opencode (BẮT BUỘC — flag chưa verify "theo preset"):
#    Spawn agent.spawn model=opencode trustPreset=full trên 1 dev server có
#    opencode cài sẵn → xác nhận CLI không hỏi permission prompt cho tool call
#    đầu tiên. Nếu fail → áp dụng "Phương Án Dự Phòng" ở trên.
```

---

## Definition of Done

- [ ] `YOLO_TUI_AGENT_ARGS` đã import từ `../shared/tui-agent-permissions`
- [ ] `AgentBinarySpec.buildArgs` signature nhận thêm `trustPreset?: 'standard' | 'full' | 'none'`
- [ ] Entry `claude`/`codex`/`gemini` trong `AGENT_SPECS` thêm flag từ `YOLO_TUI_AGENT_ARGS` khi `trustPreset === 'full'`
- [ ] Entry `opencode` thêm `--dangerously-skip-permissions` khi `trustPreset === 'full'`
- [ ] Entry `ollama` KHÔNG đổi (no-op có chủ đích, không phải thiếu sót)
- [ ] `buildAgentArgs()` forward `trustPreset: req.trustPreset` xuống `spec.buildArgs()`
- [ ] Test fixtures cũ (`claudeSpec`/`ollamaSpec` trong `agent-spawner.test.ts` dòng 239, 241) cập nhật nếu bị vỡ do đổi signature
- [ ] Unit tests mới pass (`cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts`)
- [ ] `npx tsc --noEmit` không lỗi
- [ ] **BẮT BUỘC**: smoke test thật cho opencode (`trustPreset=full` trên dev server có opencode cài sẵn) đã chạy và xác nhận PASS trước khi merge
- [ ] Nếu smoke test opencode fail: đã áp dụng "Phương Án Dự Phòng" (revert riêng nhánh trustPreset của opencode, giữ nguyên claude/codex/gemini)
- [ ] `detect_changes()` (GitNexus) đã chạy trước khi commit — xác nhận đổi signature `AgentBinarySpec.buildArgs` không làm vỡ implementer nào khác ngoài 5 entry trong `AGENT_SPECS`
