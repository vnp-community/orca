# SOL-AG-HLD-008 — Wire `trustPreset` Vào `buildAgentArgs()` Bằng Flag Đã Verify Trong Repo

**Fixes:** [BUG-AG-HLD-008](../BUG-AG-HLD-008-trust-preset-field-ignored.md)
**TDD Ref:** [TDD-AG-12 §3 — `AgentSpawnRequest.trustPreset`](../../../tdd/v5/12-agent-spawner.md) và §4 (`buildClaudeArgs` dùng `req.trustPreset` để build `--trust <preset>`)
**File:** `agent/src/relay/agent-spawner.ts`
**Effort:** 4-6 giờ (mở rộng interface + signature `buildArgs` cho 5 spec + tests + đối chiếu tài liệu)
**Status:** 🔴 TODO

---

## Phân Tích

### 1. Args mặc định hiện tại có an toàn không?

Đọc nguyên văn `AGENT_SPECS` (dòng 119-142) trong `agent/src/relay/agent-spawner.ts`:

| Agent | `buildArgs()` mặc định (không resumeId) | Có auto-skip permission không? |
|---|---|---|
| claude | `['--output-format', 'stream-json', '--verbose']` | Không |
| codex | `[]` | Không |
| gemini | `['--stream']` | Không |
| opencode | `[]` | Không |
| ollama | `[]` | Không (local inference, không có khái niệm permission prompt) |

**Kết luận: an toàn.** Không có agent nào tự động thêm flag "skip permission" mặc định — kịch bản rủi ro bảo mật mà bug report nêu ("UI hiển thị restricted nhưng agent thực chạy permissive") **không xảy ra**. Defect thật là chiều ngược lại: `trustPreset` không có tác dụng gì cả theo cả 2 hướng — kể cả khi caller **chủ động** yêu cầu preset "full-trust" (auto-approve), agent vẫn luôn dừng lại hỏi permission như mặc định, vì flag skip-permission chưa bao giờ được thêm.

### 2. Vị trí khai báo field bị lệch khỏi RPC entrypoint thật

Bug report trỏ vào `AgentEnvRequest` (dòng 183-192) — đúng, field `trustPreset?: string` có khai báo ở đó (dòng 190). Nhưng đọc kỹ luồng RPC thật:

- `handleAgentSpawn()` (dòng 261-268) build `req` với type **`AgentSpawnRequest`** (dòng 276-285) — interface này (dòng 59-68) **không có field `trustPreset` nào cả**, và code parse params cũng không đọc `params.trustPreset`.
- `buildAgentEnv()` nhận `req: AgentEnvRequest | AgentSpawnRequest` (dòng 195) — union type — nhưng **caller thật duy nhất** (dòng 338, trong `handleAgentSpawn`) luôn truyền `req` kiểu `AgentSpawnRequest`. Đã grep toàn bộ `agent/src` (non-test): **không có nơi nào khác gọi `buildAgentEnv()` với object thật sự có shape `AgentEnvRequest`** — nhánh `AgentEnvRequest` trong union chỉ được dùng bởi test fixtures (`agent/src/relay/__tests__/agent-spawner.test.ts:228-237`), không phải bởi code production.

→ Ngay cả khi patch `buildAgentEnv()` để đọc `req.trustPreset`, giá trị đó **vẫn không bao giờ đến được** từ RPC `agent.spawn` thật, vì `handleAgentSpawn()` chưa từng parse `params.trustPreset` vào `req`.

### 3. Vị trí sửa đúng là `buildAgentArgs`/`buildArgs`, không phải `buildAgentEnv`

`trustPreset` điều khiển **flag CLI** (vd `--dangerously-skip-permissions`), không phải biến môi trường — nên logic đúng thuộc về `buildAgentArgs()` (dòng 165-167) / `AgentBinarySpec.buildArgs` (dòng 52-57), tương tự cách `AgentExecRequest.trustPreset` đã được implement cho RPC `agent.exec` (một handler khác, đã hoạt động đúng):

```ts
// agent/src/relay/agent-exec-handler.ts:395-397 (đã implement, dùng làm tham chiếu)
if (req.trustPreset && req.trustPreset !== 'standard') {
  args.push('--allowedTools', req.trustPreset === 'full' ? 'all' : 'none')
}
```

### 4. Flag thật cho từng CLI — đã verify sẵn trong repo, không đoán

`agent/src/shared/tui-agent-permissions.ts` có map `YOLO_TUI_AGENT_ARGS` — nguồn flag "skip permission" **đã dùng thật** cho TUI launcher (không phải đoán, comment liền kề trong `backend/src/main/agent-trust-presets.ts` còn ghi rõ đã verify từng flag theo `--help`/bundle source của từng CLI):

```ts
export const YOLO_TUI_AGENT_ARGS: Partial<Record<TuiAgent, string>> = {
  claude: '--dangerously-skip-permissions',
  codex:  '--dangerously-bypass-approvals-and-sandbox',
  gemini: '--yolo',
  ...
}
```

`opencode` **không** có trong map này, nhưng `agent/src/shared/tui-agent-launch-defaults.ts` cho thấy opencode **luôn** được launch với `--dangerously-skip-permissions` làm default args ở nơi khác trong app:

```ts
opencode: ['--dangerously-skip-permissions'],
```

→ Đây là flag thật của CLI opencode (dùng production ở chỗ khác), nhưng KHÔNG nằm trong bảng trust-preset chính thức (`YOLO_TUI_AGENT_ARGS`) — độ tin cậy thấp hơn claude/codex/gemini một chút vì chưa được xác nhận là "điều kiện theo preset" (ở launch-defaults nó luôn bật, không theo preset). Ghi rõ mức độ tin cậy khác nhau trong bảng bên dưới.

`ollama` không có entry nào trong cả 2 map — hợp lý vì local inference không có khái niệm permission prompt; `trustPreset` với ollama là no-op **có chủ đích**, không phải thiếu sót.

### Blast radius (GitNexus)

`AGENT_SPECS`/`buildAgentArgs` risk LOW, không có caller tĩnh khác ngoài nội bộ `agent-spawner.ts`. Thay đổi signature `AgentBinarySpec.buildArgs` là **breaking nội bộ module** (5 object literal trong `AGENT_SPECS` đều implement interface này) — nhưng vì đây là type nội bộ file, không export ra ngoài (`AgentBinarySpec` interface có export nhưng không thấy implementer nào khác ngoài `AGENT_SPECS` qua `codegraph_explore`/grep), rủi ro thấp. Test fixtures trong `agent-spawner.test.ts` (dòng 239, 241) tự định nghĩa `claudeSpec`/`ollamaSpec` object literal riêng với `buildArgs: () => []` — cần cập nhật nếu test gọi `buildArgs({trustPreset: ...})`.

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-spawner.ts`

**1. Thêm `trustPreset` vào `AgentSpawnRequest` (interface RPC thật) — dùng cùng enum đã có ở `AgentExecRequest` (`agent-exec-handler.ts:326`) để nhất quán:**

```ts
export interface AgentSpawnRequest {
  taskId:        string
  userId:        string
  modelId:       string
  accountId:     string
  cwd?:          string
  resumeId?:     string
  worktreePath?: string
  branchName?:   string
+ trustPreset?:  'standard' | 'full' | 'none'  // BUG-AG-HLD-008: 'full' → thêm flag skip-permission của CLI
}
```

**2. Mở rộng signature `AgentBinarySpec.buildArgs` để nhận `trustPreset`:**

```ts
export interface AgentBinarySpec {
  readonly binary:         string
- readonly buildArgs:      (req?: { resumeId?: string }) => string[]
+ readonly buildArgs:      (req?: { resumeId?: string; trustPreset?: 'standard' | 'full' | 'none' }) => string[]
  readonly apiKeyEnvVar:   string | null
  readonly localInference?: boolean
}
```

**3. Import map flag đã verify (tránh duplicate/đoán lại flag):**

```ts
import { YOLO_TUI_AGENT_ARGS } from '../shared/tui-agent-permissions'
```

**4. Sửa `AGENT_SPECS` — thêm nhánh trust cho claude/codex/gemini (nguồn: `YOLO_TUI_AGENT_ARGS`, độ tin cậy CAO) và opencode (nguồn: `tui-agent-launch-defaults.ts`, độ tin cậy TRUNG BÌNH — flag y hệt nhưng chưa verify theo preset). Ollama không đổi (no-op có chủ đích):**

```ts
const AGENT_SPECS: AgentBinarySpec[] = [
  // index 0: claude
  {
    binary: 'claude',
-   buildArgs: (req) => req?.resumeId
-     ? ['--resume', req.resumeId]
-     : ['--output-format', 'stream-json', '--verbose'],
+   buildArgs: (req) => {
+     const args = req?.resumeId
+       ? ['--resume', req.resumeId]
+       : ['--output-format', 'stream-json', '--verbose']
+     // BUG-AG-HLD-008: flag verified trong YOLO_TUI_AGENT_ARGS (dùng thật cho TUI launcher)
+     if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.claude!)
+     return args
+   },
    apiKeyEnvVar: 'ANTHROPIC_API_KEY',
  },
  // index 1: codex
  {
    binary: 'codex',
-   buildArgs: (req) => req?.resumeId
-     ? ['--session-file', `~/.codex/${req.resumeId}.json`]
-     : [],
+   buildArgs: (req) => {
+     const args = req?.resumeId
+       ? ['--session-file', `~/.codex/${req.resumeId}.json`]
+       : []
+     if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.codex!)
+     return args
+   },
    apiKeyEnvVar: 'OPENAI_API_KEY',
  },
  // index 2: gemini (giả định đã áp SOL-AG-HLD-007 cho phần resumeId; nếu áp
  // riêng lẻ, giữ nhánh resumeId cũ `['--stream']` không đổi)
  {
    binary: 'gemini',
    buildArgs: (req) => {
      const args = req?.resumeId ? ['--resume', req.resumeId] : ['--stream']
+     if (req?.trustPreset === 'full') args.push(YOLO_TUI_AGENT_ARGS.gemini!)
      return args
    },
    apiKeyEnvVar: 'GEMINI_API_KEY',
  },
  // index 3: opencode — flag verified ở tui-agent-launch-defaults.ts (độ tin
  // cậy trung bình — chưa verify là "theo preset", chỉ biết CLI chấp nhận flag)
  {
    binary: 'opencode',
    buildArgs: (req) => {
      const args = req?.resumeId ? ['--session', req.resumeId] : []
+     if (req?.trustPreset === 'full') args.push('--dangerously-skip-permissions')
      return args
    },
    apiKeyEnvVar: null,
  },
  // index 4: ollama — KHÔNG thêm nhánh trustPreset: local inference không có
  // permission prompt, no-op có chủ đích (không phải thiếu sót).
  { binary: 'ollama', buildArgs: () => [], apiKeyEnvVar: null, localInference: true },
]
```

**5. Forward `trustPreset` trong `buildAgentArgs()` (dòng 165-167):**

```ts
function buildAgentArgs(spec: AgentBinarySpec, req: AgentSpawnRequest): string[] {
- return spec.buildArgs({ resumeId: req.resumeId })
+ return spec.buildArgs({ resumeId: req.resumeId, trustPreset: req.trustPreset })
}
```

**6. Parse `params.trustPreset` trong `handleAgentSpawn()` (dòng ~276-285):**

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
+   trustPreset:   params.trustPreset === 'full' || params.trustPreset === 'none'
+     ? params.trustPreset
+     : undefined,  // mặc định 'standard' (không thêm flag) nếu thiếu/không hợp lệ
  }
```

**7. `AgentEnvRequest.trustPreset` (dòng 190) — không xoá, nhưng làm rõ nó không còn là nguồn thật:**

`AgentEnvRequest` không được bất kỳ caller production nào construct (chỉ union-type nội bộ + test fixtures — xem Phân Tích §2). Xoá hẳn interface này là refactor rộng hơn scope bug này (ảnh hưởng type union của `buildAgentEnv()` + test fixtures). Trong scope fix này, chỉ thêm comment tránh hiểu lầm rằng field này có tác dụng:

```ts
export interface AgentEnvRequest {
  accountId:   string
  userId:      string
  taskId:      string
  projectId?:  string
  cwd:         string
  model?:      string
- trustPreset?: string
+ // BUG-AG-HLD-008: KHÔNG dùng để build CLI args — trust preset thật được đọc
+ // từ AgentSpawnRequest.trustPreset trong buildAgentArgs(), không phải ở đây.
+ // Giữ field này chỉ vì AgentEnvRequest hiện chưa có caller production nào
+ // construct nó (xem SOL-AG-HLD-008 §2) — cân nhắc xoá hẳn AgentEnvRequest
+ // trong 1 refactor riêng nếu vẫn không có caller thật sau khi fix này merge.
  trustPreset?: string
  extraEnv?:   Record<string, string>
}
```

(Ghi chú: nếu review thấy comment 2 dòng field trùng tên gây rối, có thể gộp thành 1 dòng field + JSDoc phía trên — điểm chính là: **không im lặng để field trông như đang hoạt động**.)

---

## Khuyến Nghị: Option A (implement) an toàn hơn Option B (xoá field) trong trường hợp này

Bug report cho 2 lựa chọn: (a) implement nếu biết chắc flag, (b) xoá field nếu không chắc. Khuyến nghị chọn **(a)** vì:

- Đã tìm được flag verified sẵn có trong repo (`YOLO_TUI_AGENT_ARGS`) cho claude/codex/gemini — không phải đoán.
- Opencode dùng flag đã chạy thật ở nơi khác trong repo (`tui-agent-launch-defaults.ts`), độ tin cậy trung bình nhưng vẫn tốt hơn giả định ngẫu nhiên.
- Xoá field (option b) sẽ để mất khả năng "full-trust spawn" hoàn toàn — trong khi tính năng này rõ ràng có nhu cầu thật (đã implement tương tự cho `agent.exec`), xoá đi rồi phải làm lại sau là lãng phí.
- Rủi ro residual duy nhất (opencode's flag chưa verify "theo preset") được xử lý bằng smoke test bắt buộc trước khi merge — xem Verification.

---

## Verification

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

# 2. Smoke test THẬT cho opencode (bắt buộc — flag chưa verify "theo preset"):
#    Spawn agent.spawn model=opencode trustPreset=full trên 1 dev server có
#    opencode cài sẵn → xác nhận CLI không hỏi permission prompt cho tool call
#    đầu tiên. Nếu fail → tạm bỏ nhánh opencode khỏi map (revert riêng phần đó),
#    giữ claude/codex/gemini (đã verify cao hơn).

# 3. detect_changes() trước khi commit — theo yêu cầu AGENTS.md/CLAUDE.md:
#    xác nhận thay đổi signature AgentBinarySpec.buildArgs không làm vỡ
#    implementer nào khác ngoài 5 entry trong AGENT_SPECS.
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-spawner.ts` | `AgentSpawnRequest`, `AgentBinarySpec`, `AGENT_SPECS`, `buildAgentArgs()`, `handleAgentSpawn()` — nơi sửa chính |
| `agent/src/shared/tui-agent-permissions.ts` | `YOLO_TUI_AGENT_ARGS` — nguồn flag verified cho claude/codex/gemini, import trực tiếp thay vì duplicate |
| `agent/src/shared/tui-agent-launch-defaults.ts` | Nguồn flag `--dangerously-skip-permissions` cho opencode (dùng ở launcher khác, độ tin cậy trung bình cho use-case này) |
| `agent/src/relay/agent-exec-handler.ts` | `AgentExecRequest.trustPreset` — implementation tương tự đã hoạt động cho RPC `agent.exec` khác, dùng làm tham chiếu pattern |
| `desktop/src/relay/agent-spawner.ts` | Bản mirror cùng cấu trúc trong package `desktop/` — cần áp cùng patch (ngoài scope, task riêng) |
| `agent/src/relay/__tests__/agent-spawner.test.ts` | Cập nhật test fixtures `claudeSpec`/`ollamaSpec` (dòng 239, 241) + thêm test cases trustPreset |
| `docs/logic/agent-orchestration/BL-PRF-04*.md` | Tài liệu thiết kế gốc mô tả `trustPreset` — đối chiếu lại enum `'standard' \| 'full' \| 'none'` khớp với implementation |
