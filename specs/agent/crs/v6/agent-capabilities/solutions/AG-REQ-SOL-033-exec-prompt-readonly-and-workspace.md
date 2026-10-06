# AG-REQ-SOL-033-A: `agent.execPrompt` chế độ chỉ đọc và vùng làm việc

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" dưới đây là đọc code, chưa chạy gì.

**CR:** [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md) mục 2.1, 2.2, 2.3
**Service:** `agent/` (Dev Server Agent), thư mục `agent/src/relay/`
**Nhóm solution:** A trong ba nhóm của CR-REQ-033 (B: khối kết quả và danh sách file đổi, C: báo cáo năng lực, handshake, `ai.complete`)
**TDD tham chiếu:** [v5/02-wire-protocol](../../../../tdd/v5/02-wire-protocol.md), [v5/04-handshake-session](../../../../tdd/v5/04-handshake-session.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/12-agent-spawner](../../../../tdd/v5/12-agent-spawner.md), [v5/11-fs-handler-extension](../../../../tdd/v5/11-fs-handler-extension.md). Bản v4 cùng tên có ở `specs/agent/tdd/v4/`; v5 mới hơn (có 09 đến 13) nên dùng v5.
**Mẫu định dạng:** `specs/agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md`

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent/src/relay/agent-print-mode-exec.ts` (382 dòng, đúng số CR nêu), `agent-rpc-dispatch-agent-exec.ts` (218 dòng), `agent-binary-specs.ts`, `agent-spawn-env.ts`, `agent-print-mode-exec.test.ts`, `agent/package.json`, `agent/vitest.config.ts`.

| Điểm | Hiện trạng thật | Hệ quả cho solution |
|---|---|---|
| Hai handler | `handleAgentExecPrompt` (trả một phản hồi, có `notify` chunk) và `handleAgentExecPromptStream` (frame `stream.chunk`/`stream.end`) lặp lại nguyên khối đọc tham số, kiểm model, dựng `args`, `buildAgentEnv` | Tham số mới phải áp cho cả hai; gom phần đọc và kiểm tham số mới vào file mới để không lặp lần thứ ba |
| `args` | `['--print', prompt]` (có `initFile` thì `initFile\nprompt`), thêm `YOLO_TUI_AGENT_ARGS.claude` (`--dangerously-skip-permissions`) khi `trustPreset === 'full'`; prompt là phần tử argv thứ hai, cờ thêm đặt SAU prompt | Cờ chỉ đọc cũng nối sau prompt. `--tools` có dạng `<tools...>` (variadic) nên phải dùng đúng một phần tử gộp bằng dấu phẩy (xem 3.2) |
| `worktreePath` | Chỉ kiểm không rỗng; không kiểm tồn tại. Đường dẫn sai làm `spawn` phát `error` ENOENT, kết quả `exitCode: null`, `stderr` là thông điệp lỗi (nhánh `child.on('error')`) | `workspaceKind` mới cho phép báo lỗi sớm, có mã. `worktree` giữ nguyên (không kiểm thêm) để không đổi hành vi |
| Handler stream thiếu `taskId`/`projectId` | `handleAgentExecPromptStream` dựng env với `taskId: stepId ?? ''` và không truyền `projectId`, khác handler thường (đọc `params.taskId`, `params.projectId`) | Khác biệt có sẵn, không thuộc CR-033. Task A-01 ghi lại, không sửa ngầm (câu hỏi mở 1) |
| Loại model | `resolveAgentSpec(modelId)` rồi từ chối nếu `spec.binary !== 'claude'` bằng `InvalidParams` kèm chuỗi `UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC` trong `message` | Mã lỗi mới theo cùng quy ước: chuỗi mã trong `message` và thêm `error.data.reason` |
| Tham số lạ | Cả hai handler chỉ đọc các khoá đã biết, bỏ qua khoá khác, không lỗi | **Agent cũ chạy ở chế độ ghi khi nhận `accessMode=readonly`** mà không báo. Đây là rủi ro tương thích lớn nhất (mục 3.5) |
| Hai bản agent | `agent/src/relay/` có `agent-print-mode-exec.ts`; `grep -rn execPrompt desktop/src/relay` không ra file nào; `desktop/src/relay/ai-complete-handler.ts` có (bản lệch) | Xác minh lại kết luận CR: chỉ sửa `agent/`. Chi tiết ở README solutions mục "Hai bản agent" |
| Test hiện có | `agent-print-mode-exec.test.ts` mock `node:child_process.spawn` bằng `FakeChild`, `waitForSpawn()`, `MOCK_CONFIG`, `readDecryptedKeyMock`; 2 `describe` (`handleAgentExecPrompt`, `handleAgentExecPromptStream`) | Test mới theo đúng khuôn này; test hồi quy là các `it` hiện có phải xanh không sửa |
| Lỗi `max-lines` | `.oxlintrc.json` có các ngưỡng 300, 400, 600 dòng (đã thấy, chưa kiểm glob nào áp cho file nào); AGENTS.md cấm `max-lines` disable | Mọi logic mới ở file mới, `agent-print-mode-exec.ts` chỉ thêm vài dòng gọi |

Correction relative to CR-REQ-033: (1) CR viết "trả `stderr` với `exitCode=null`" cho đường dẫn sai: đúng, nhưng nhánh timeout cũng trả `exitCode: null`, nên phía backend không phân biệt được hai trường hợp nếu chỉ nhìn `exitCode`. (2) CR không nói `agent.execPromptStream` trả những trường mới ở frame nào; solution này chốt: đặt vào frame `stream.end` (mục 2.5).

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/
  agent-exec-prompt-options.ts              (mới) đọc và kiểm accessMode, workspaceKind, reportChanges, resultBlock, maxOutputBytes
  agent-exec-prompt-options.test.ts         (mới)
  agent-readonly-tool-policy.ts             (mới) dò cờ claude, dựng args chỉ đọc, cache help 10 phút
  agent-readonly-tool-policy.test.ts        (mới)
  agent-workspace-validation.ts             (mới) kiểm worktree|repo_root|scratch
  agent-workspace-validation.test.ts        (mới)
  agent-print-mode-exec.ts                  (sửa) gọi các hàm trên ở cả hai handler
  agent-print-mode-exec.test.ts             (sửa) thêm describe cho tham số mới, giữ nguyên test cũ
```

Nhóm B thêm `maxOutputBytes`, `reportChanges`, `resultBlock` vào cùng điểm gọi; A chỉ cung cấp bộ đọc tham số chung nên B phụ thuộc A-01.

### 2.2 Tham số mới (hợp đồng với backend)

| Tham số | Kiểu | Mặc định | Giá trị hợp lệ | Sai thì |
|---|---|---|---|---|
| `accessMode` | string | `"write"` | `"write"`, `"readonly"` | `InvalidParams`, `INVALID_ACCESS_MODE` |
| `workspaceKind` | string | `"worktree"` | `"worktree"`, `"repo_root"`, `"scratch"` | `InvalidParams`, `INVALID_WORKSPACE_KIND` |

`reportChanges`, `resultBlock`, `maxOutputBytes` thuộc nhóm B và được `agent-exec-prompt-options.ts` đọc cùng lượt (A-01 chỉ định nghĩa kiểu và để nhóm B điền kiểm tra, xem B-05 đến B-08). Kiểu trả về của bộ đọc:

```ts
export type ExecPromptOptions = {
  accessMode: 'write' | 'readonly'
  workspaceKind: 'worktree' | 'repo_root' | 'scratch'
  explicit: { accessMode: boolean; workspaceKind: boolean } // có gửi tham số hay không
  reportChanges: boolean
  resultBlockNonce: string | null
  maxOutputBytes: number
}
export type ExecPromptOptionsError = { code: ExecPromptErrorCode; message: string }
export function parseExecPromptOptions(
  params: Record<string, unknown>
): { ok: true; value: ExecPromptOptions } | { ok: false; error: ExecPromptOptionsError }
```

Quy tắc đọc: giá trị không phải chuỗi/boolean/số đúng kiểu thì lỗi, không âm thầm đổi về mặc định (khác `timeoutMs` hiện kẹp trong khoảng). Lý do: tham số an toàn (`accessMode`) mà bị đọc sai thành mặc định `write` là hạ cấp âm thầm.

### 2.3 Chế độ chỉ đọc

`agent-readonly-tool-policy.ts`:

```ts
export type ClaudeFlagSupport = { tools: boolean; permissionMode: boolean; disallowedTools: boolean }
export type ClaudeHelpProbe = (env: NodeJS.ProcessEnv) => Promise<string>   // chạy `claude --help`, timeout 5000 ms
export const READONLY_DEFAULT_TOOLS = ['Read', 'Glob', 'Grep'] as const     // tên công cụ CHƯA KIỂM CHỨNG
export async function detectClaudeFlags(env, probe?: ClaudeHelpProbe, now?: () => number): Promise<ClaudeFlagSupport | null>
export function buildReadonlyArgs(flags: ClaudeFlagSupport): string[]
// -> ['--permission-mode', 'plan', '--tools', 'Read,Glob,Grep']
export function readonlyUnsupportedReason(flags: ClaudeFlagSupport | null): string | null
```

Thuật toán: `detectClaudeFlags` lưu cache theo module, TTL 10 phút (`CLAUDE_FLAGS_TTL_MS = 600_000`), khoá cache là `env.PATH` (vì PATH khác có thể trỏ binary `claude` khác). Trả `null` khi probe lỗi hoặc timeout. Cờ có mặt khi help chứa `\b--tools\b`, `\b--permission-mode\b`, `\b--disallowedTools\b` (hoặc `--disallowed-tools`, chưa kiểm dạng nào xuất hiện). `buildReadonlyArgs` chỉ gọi khi `tools && permissionMode`; ngược lại handler trả lỗi (fail closed):

| Tình huống | Phản hồi |
|---|---|
| Probe `claude --help` lỗi, hết hạn, ENOENT | `InvalidParams -32602`, `message` chứa `READONLY_MODE_UNSUPPORTED`, `data: { reason: 'READONLY_MODE_UNSUPPORTED', detail: 'CLAUDE_HELP_UNAVAILABLE' }` |
| Help không có `--tools` | cùng mã, `data.detail: 'FLAG_TOOLS_MISSING'` |
| Help không có `--permission-mode` | cùng mã, `data.detail: 'FLAG_PERMISSION_MODE_MISSING'` |

Mã JSON-RPC theo CR là `InvalidParams`. Dùng `makeError(id, AgentErrorCode.InvalidParams, message, data)` (đã có tham số `data` thứ tư ở `agent-rpc-dispatch.ts`).

`trustPreset=full` với `readonly`: không thêm `YOLO_TUI_AGENT_ARGS.claude`, ghi `log.warn`, trả `warnings: ['TRUST_PRESET_IGNORED_READONLY']` trong kết quả (handler thường) hoặc trong `stream.end` (handler stream).

Chỉ dùng cờ CR-033 đã nêu: `--permission-mode plan`, `--tools`. Không thêm `--disallowedTools`, `--allowedTools`, `--output-format`, `--max-budget-usd`, `--json-schema`: những cờ này chỉ được dùng khi CR khác cho phép. Mọi hành vi thật của hai cờ trên với `--print` **chưa kiểm chứng**; bằng chứng duy nhất hiện có là `claude --help` liệt kê chúng và repo đã dùng `--permission-mode plan` ở `frontend/src/shared/commit-message-agent-spec.ts` (chế độ khác, không phải `--print` một lần).

### 2.4 Vùng làm việc

`agent-workspace-validation.ts`:

```ts
export type WorkspaceCheckInput = {
  kind: 'worktree' | 'repo_root' | 'scratch'
  path: string
  accessMode: 'write' | 'readonly'
  scratchRoots: string[]          // [os.tmpdir(), join(config.workDir, '.orca-scratch')]
  gitToplevel?: (cwd: string) => Promise<string | null>   // tiêm để test; mặc định execFile git rev-parse
}
export type WorkspaceCheckResult =
  | { ok: true; realPath: string }
  | { ok: false; code: WorkspaceErrorCode; message: string }
export async function validateWorkspace(input: WorkspaceCheckInput): Promise<WorkspaceCheckResult>
```

| `code` | Khi nào | JSON-RPC |
|---|---|---|
| `REPO_ROOT_REQUIRES_READONLY` | `kind=repo_root` và `accessMode=write` (kiểm trước mọi truy cập file) | `InvalidParams` |
| `WORKSPACE_PATH_NOT_FOUND` | `realpath` lỗi ENOENT | `InvalidParams` (agent hiện trả `PathNotFound -33003` ở nơi khác; xem câu hỏi mở 2) |
| `WORKSPACE_NOT_A_DIRECTORY` | không phải thư mục | `InvalidParams` |
| `WORKSPACE_NOT_A_GIT_REPO` | `repo_root` mà `git rev-parse --show-toplevel` thất bại | `InvalidParams` |
| `SCRATCH_OUTSIDE_ALLOWED_ROOTS` | `scratch` mà `realpath` không nằm DƯỚI một trong `scratchRoots` (cũng từ chối đúng bằng thư mục gốc, vì `os.tmpdir()` dùng chung) | `InvalidParams` |

Kiểm bao hàm bằng `path.relative(root, real)` không bắt đầu bằng `..` và không tuyệt đối (tránh lỗi tiền tố `/tmp` so với `/tmpfoo`, và đúng trên Windows). `realpath` được gọi cho cả `root` lẫn đường dẫn để chống thoát bằng liên kết tượng trưng. Gọi Git chỉ bằng `execFile('git', ['rev-parse', '--show-toplevel'], { cwd, timeout: 5000 })`, không đi qua shell, không tuỳ chọn toàn cục, nên không đổi baseline Git 2.25 của `guides/reference/git-compatibility.md`. Không cần `GitCapabilityCache` vì lệnh có từ trước 2.25 (ghi trong task A-03). Agent chạy trên chính host nên đúng cho SSH và WSL (agent chạy trong distro).

`kind=worktree` trả `{ ok: true, realPath: path }` không đụng hệ tệp (giữ hành vi cũ, kể cả ENOENT chỉ lộ ở bước spawn).

### 2.5 Nối vào hai handler

Thứ tự bước mới (cả hai handler, các bước 1 đến 3 là mới):

1. `parseExecPromptOptions(params)`; lỗi thì trả lỗi (handler thường: giá trị trả về; handler stream: một frame lỗi, không có `stream.chunk` nào).
2. `validateWorkspace(...)` (bỏ qua nhanh nếu `kind=worktree`).
3. Kiểm model như cũ, sau đó `buildAgentEnv` như cũ.
4. Nếu `readonly`: `detectClaudeFlags(env)` rồi `buildReadonlyArgs`; lỗi thì `READONLY_MODE_UNSUPPORTED` và KHÔNG gọi `spawn`.
5. Dựng `args`: `['--print', prompt]`, rồi (chỉ khi `write` và `trustPreset=full`) cờ YOLO, rồi (chỉ khi `readonly`) `...readonlyArgs`.
6. `spawn` như cũ.

Phản hồi có "echo" áp dụng (mở rộng ngoài CR, xem mục 3.5): khi người gọi gửi tường minh `accessMode` hoặc `workspaceKind`, kết quả thêm `applied: { accessMode, workspaceKind }`. Backend dùng nó để phát hiện agent cũ đã bỏ qua tham số (agent cũ không có `applied`). Không gửi tham số mới thì không có `applied`, kết quả y hệt hiện nay. Với handler stream, `applied` nằm trong `stream.end`; backend CHỈ được tin `readonly` khi thấy `applied.accessMode === 'readonly'` ở frame cuối (ngoài việc kiểm `features`).

## 3. Quyết định thiết kế

| # | Quyết định | Lý do | Phương án đã bỏ |
|---|---|---|---|
| 1 | Logic mới ở file mới, handler chỉ gọi | Giữ `agent-print-mode-exec.ts` dưới ngưỡng `max-lines`, không thêm disable | Viết thẳng vào handler |
| 2 | Từ chối thay vì hạ cấp khi CLI thiếu cờ | Hạ cấp âm thầm biến chỉ đọc thành lời hứa suông | Tự động chạy ở chế độ ghi kèm cảnh báo |
| 3 | `--tools` truyền một phần tử `Read,Glob,Grep` (dạng dấu phẩy) | Cờ variadic sau prompt dễ nuốt phần tử kế tiếp; dạng dấu phẩy **chưa kiểm chứng** là được `claude` chấp nhận (help ghi `<tools...>`), nên test đối kháng trên máy thật (task A-04) phải xác nhận, nếu sai thì chuyển sang ba phần tử đặt cuối argv | Ba phần tử cách nhau dấu cách |
| 4 | Giá trị tham số sai kiểu thì lỗi, không về mặc định | `accessMode` sai thành `write` là hạ cấp âm thầm | Kẹp như `timeoutMs` |
| 5 | `repo_root` bắt buộc `readonly` | Ghi vào checkout chính của project là rủi ro lớn nhất của `spike`/`question`/`diagnosis` (CR-REQ-008) | Cảnh báo thay vì từ chối |
| 6 | Thêm `applied` echo, chỉ khi người gọi gửi tham số tường minh | Agent cũ bỏ qua tham số lạ im lặng; không có echo thì backend không phân biệt được | Chỉ dựa vào `features` ở handshake (không bắt được trường hợp `features` cũ do cache) |
| 7 | Cache help theo `PATH`, TTL 10 phút | CR nói đọc "mỗi 10 phút"; PATH khác có thể là `claude` khác | Cache toàn cục không khoá |
| 8 | Không sửa `desktop/src/relay` | Bản đã lệch, không có `execPrompt` | Đồng bộ hai bản |

### 3.5 Tương thích hai chiều

| Backend | Agent | Kết quả | Việc phải làm |
|---|---|---|---|
| Cũ (không gửi tham số mới) | Mới | Y hệt trước; không có `applied`, `warnings` | Không |
| Mới gửi `accessMode=readonly` | Cũ | Agent cũ chạy ở chế độ GHI, bỏ qua tham số, kết quả không có `applied` | Backend CHỈ gửi khi `features` chứa `agent.execPrompt.readonly` (hồ sơ năng lực), và coi kết quả thiếu `applied.accessMode === 'readonly'` là không chỉ đọc (`enforcement=prompt_only`, hoặc từ chối nếu cờ `REQUEST_REQUIRE_ENFORCED_READONLY=true`) |
| Mới | Mới nhưng `claude` thiếu cờ | `READONLY_MODE_UNSUPPORTED`, không spawn | Backend map sang đường `prompt_only` hoặc `REQUEST_ANALYSIS_AGENT_TOO_OLD` (tên do CR-REQ-008 chốt) |
| Mới gửi `workspaceKind=repo_root` | Cũ | Agent cũ không kiểm gì, chạy luôn ở `cwd` | Cũng gate bằng `features` (`agent.execPrompt.workspaceKind`) |

## 4. Phụ thuộc và thứ tự

- Không phụ thuộc CR hay solution khác để bắt đầu.
- Task: A-01 (`AG-REQ-TASK-033-01`) trước; 02 và 03 song song; 04 sau cùng (nối vào handler). Nhóm B bắt đầu được sau 01.
- `features` ở handshake (nhóm C, task 12) CHỈ được thêm `agent.execPrompt.readonly` và `agent.execPrompt.workspaceKind` sau khi 01 đến 04 xong.
- Backend: `infra-fleet-service` (hồ sơ năng lực) và `request-service` (CR-REQ-008 `AgentReadonlyRunner`) phải chọn đường theo `features` và đọc `applied` (mục 3.5). Hai phần này không nằm trong solution này.

## 5. Kiểm thử

Chạy trong `/opt/repos/orca/agent` (script `test` là `vitest run`, `vitest.config.ts` gom `src/**/*.test.ts`, môi trường node):

| Tầng | Test | Lệnh |
|---|---|---|
| Unit | `agent-exec-prompt-options.test.ts`, `agent-readonly-tool-policy.test.ts` (probe giả), `agent-workspace-validation.test.ts` (thư mục tạm thật, symlink) | `pnpm exec vitest run src/relay/agent-exec-prompt-options.test.ts src/relay/agent-readonly-tool-policy.test.ts src/relay/agent-workspace-validation.test.ts` |
| Handler (mock `spawn`) | thêm `describe('handleAgentExecPrompt readonly and workspaceKind')` và bản stream vào `agent-print-mode-exec.test.ts` | `pnpm exec vitest run src/relay/agent-print-mode-exec.test.ts` |
| Hồi quy | toàn bộ `it` cũ của file trên chạy không sửa | cùng lệnh |
| CLI giả | script `claude` giả (Node, in help có hoặc không có `--tools`) đặt trong thư mục tạm, đưa vào `env.PATH` qua `params.env` | trong test, bỏ qua trên Windows |
| `claude` thật | `agent-readonly-adversarial.e2e.test.ts` (mới, tắt mặc định bằng `describe.skipIf(!process.env.ORCA_REAL_CLAUDE_E2E)`), mô tả ở task A-04. **Chưa chạy** | `ORCA_REAL_CLAUDE_E2E=1 pnpm exec vitest run src/relay/agent-readonly-adversarial.e2e.test.ts` |
| Toàn gói | không hỏng test khác | `pnpm test` |

Kiểm kiểu: `agent/package.json` không có script typecheck; dùng `npx tsc --noEmit` trong `agent/`. Tài liệu v4 ghi 53 lỗi có sẵn ở file test không liên quan (ghi ngày 2026-09-09, chưa chạy lại ở đợt này): chỉ so sánh trước và sau, không sửa lỗi cũ.

## 6. Rủi ro và điểm chưa kiểm chứng

- Tên công cụ `Read`, `Glob`, `Grep` và việc `--permission-mode plan` kết hợp `--print` có thật sự chặn mọi đường ghi: chưa kiểm chứng. Nếu `--tools` nhận tên khác, `claude` có thể báo lỗi hoặc im lặng bỏ công cụ; kết quả của đợt thử đối kháng quyết định có bật `REQUEST_REQUIRE_ENFORCED_READONLY` hay không.
- Có thể `plan` mode làm `claude --print` dừng chờ phê duyệt kế hoạch thay vì trả kết quả. Chưa biết; dò ở task A-04 bằng đo thời gian và nội dung `stdout`.
- Dạng `--tools Read,Glob,Grep` (dấu phẩy) chưa kiểm chứng.
- Cache help theo `PATH`: nếu `claude` được nâng cấp giữa hai lần chạy, tối đa 10 phút dùng cờ cũ.
- `realpath` trên Windows và WSL: chưa chạy test trên hai nền tảng này (chỉ viết test dùng `path` thuần).
- Hiệu năng `git rev-parse` trên repo lớn không đáng kể nhưng chưa đo.

## 7. Câu hỏi mở

1. Có sửa luôn khác biệt `taskId`/`projectId` của handler stream (mục 1) trong đợt này không? Đề xuất: tách việc, vì đổi `ORCA_TASK_ID` của đường stream là đổi hành vi hiện có.
2. `WORKSPACE_PATH_NOT_FOUND` nên là `InvalidParams` (như CR nêu mã `InvalidParams` cho lỗi tham số) hay `PathNotFound -33003` (đã có trong `AgentErrorCode`)? Solution dùng `InvalidParams` kèm `data.reason`; cần backend (infra-fleet) xác nhận cách nó map mã lỗi.
3. Có cho `Bash` giới hạn (`Bash(git log *)`) sau thử nghiệm đối kháng? (Trùng câu hỏi mở 1 của CR; ngoài phạm vi v1.)

## 8. Tham chiếu

- `/opt/repos/orca/agent/src/relay/agent-print-mode-exec.ts`, `agent-print-mode-exec.test.ts`, `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch.ts` (hàm `makeError` có tham số `data`), `agent-binary-specs.ts`, `agent-spawn-env.ts`, `agent-spawner.ts`
- `/opt/repos/orca/agent/src/shared/tui-agent-permissions.ts` (`YOLO_TUI_AGENT_ARGS.claude`), `agent/src/shared/agent-wire-protocol.ts` (`AgentErrorCode`)
- `/opt/repos/orca/agent/package.json`, `agent/vitest.config.ts`
- `/opt/repos/orca/frontend/src/shared/commit-message-agent-spec.ts` (tiền lệ `--permission-mode plan`)
- `/opt/repos/orca/guides/reference/git-compatibility.md`
- CR: `/opt/repos/orca/docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md`; CR-REQ-008 (AgentReadonlyRunner)
