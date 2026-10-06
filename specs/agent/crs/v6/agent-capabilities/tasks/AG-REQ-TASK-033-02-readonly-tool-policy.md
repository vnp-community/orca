# AG-REQ-TASK-033-02: Chính sách công cụ chỉ đọc và dò cờ `claude` (`agent-readonly-tool-policy.ts`)

**From Solution:** [AG-REQ-SOL-033-exec-prompt-readonly-and-workspace](../solutions/AG-REQ-SOL-033-exec-prompt-readonly-and-workspace.md) mục 2.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-readonly-tool-policy.ts` (mới), `agent/src/relay/agent-readonly-tool-policy.test.ts` (mới)
**Depends on:** không (độc lập với task 01; task 04 nối vào handler, task 09 dùng lại hàm dò)
**Status:** [ ] TODO

## Context

CR-033 mục 2.2 yêu cầu: khi `accessMode=readonly` agent thêm `--permission-mode plan` và `--tools` chỉ gồm công cụ đọc; trước lần chạy đầu và mỗi 10 phút đọc `claude --help` (timeout 5 giây, lưu cache) để biết cờ có tồn tại, và nếu thiếu thì TỪ CHỐI (`READONLY_MODE_UNSUPPORTED`, mã `InvalidParams`, `error.data.reason`), không chạy ở chế độ ghi.

Đã đọc trong code thật: `execPrompt` chạy `claude` bằng `spawn(spec.binary, args, { cwd, env: { ...process.env, ...env } })` với `env.PATH = config.toolPath ?? process.env.PATH` (do `buildAgentEnv`, `agent-spawn-env.ts` dòng 70 trở đi). Vì vậy `claude --help` phải được dò với CÙNG `env` (merge giống hệt), nếu không PATH khác sẽ trỏ `claude` khác (hoặc không tìm thấy). Hàm dò nhận `env` đã merge, không tự dựng.

Hiện repo có tiền lệ `--permission-mode plan` ở `frontend/src/shared/commit-message-agent-spec.ts`, nhưng KHÔNG ở chế độ `--print` một lần. CR nêu `claude --help` (phiên bản 2.1.289 trên máy soạn thảo) có `--permission-mode` (giá trị `acceptEdits, auto, bypassPermissions, manual, dontAsk, plan`), `--tools`, `--allowedTools`, `--disallowedTools`. Hành vi thật chưa kiểm chứng; tên công cụ `Read`, `Glob`, `Grep` chưa kiểm chứng. Task này chỉ dùng hai cờ CR đã nêu (`--permission-mode plan`, `--tools`); KHÔNG thêm `--disallowedTools`, `--allowedTools`, `--output-format`.

## Việc cần làm

1. Tạo `agent-readonly-tool-policy.ts` với:
   ```ts
   export const READONLY_DEFAULT_TOOLS = ['Read', 'Glob', 'Grep'] as const  // UNVERIFIED tool names
   export const CLAUDE_FLAGS_TTL_MS = 600_000
   export const CLAUDE_HELP_TIMEOUT_MS = 5_000
   export type ClaudeFlagSupport = { tools: boolean; permissionMode: boolean; disallowedTools: boolean }
   export type ClaudeHelpProbe = (env: NodeJS.ProcessEnv) => Promise<string>
   export type ReadonlyUnsupportedReason =
     'CLAUDE_HELP_UNAVAILABLE' | 'FLAG_TOOLS_MISSING' | 'FLAG_PERMISSION_MODE_MISSING'
   ```
2. Cài `defaultClaudeHelpProbe(env)`: `execFile('claude', ['--help'], { env, timeout: CLAUDE_HELP_TIMEOUT_MS, maxBuffer: 1024 * 1024, windowsHide: true })`, trả `stdout + '\n' + stderr` (một số CLI in help ra stderr). Lỗi hoặc hết giờ thì ném.
3. Cài `detectClaudeFlags(env, probe = defaultClaudeHelpProbe, now = Date.now): Promise<ClaudeFlagSupport | null>`:
   - Cache ở cấp module `Map<string, { at: number; flags: ClaudeFlagSupport | null }>` khoá `env.PATH ?? ''`.
   - Còn hạn (`now() - at < CLAUDE_FLAGS_TTL_MS`) thì dùng cache, KỂ CẢ khi kết quả là `null`? KHÔNG: chỉ cache kết quả thành công; lỗi dò được thử lại lần sau (tránh khoá chết 10 phút vì lỗi thoáng qua).
   - Dò cờ bằng regex trên văn bản help: `/(^|[\s,])--tools(?=[\s<=,]|$)/m`, `/(^|[\s,])--permission-mode(?=[\s<=,]|$)/m`, `/(^|[\s,])--(?:disallowedTools|disallowed-tools)(?=[\s<=,]|$)/m`. Không khớp theo chuỗi con lỏng (tránh `--tools-foo`).
   - Một lời gọi trùng khi đang dò dùng chung một Promise (single flight) cho cùng khoá.
4. Cài `readonlyUnsupportedReason(flags): ReadonlyUnsupportedReason | null`: `flags === null` thì `CLAUDE_HELP_UNAVAILABLE`; thiếu `tools` thì `FLAG_TOOLS_MISSING`; thiếu `permissionMode` thì `FLAG_PERMISSION_MODE_MISSING`; còn lại `null`.
5. Cài `buildReadonlyArgs(): string[]` trả `['--permission-mode', 'plan', '--tools', READONLY_DEFAULT_TOOLS.join(',')]`. Một phần tử gộp bằng dấu phẩy để cờ variadic `--tools <tools...>` không nuốt phần tử sau; CHỖ NÀY CHƯA KIỂM CHỨNG (Rủi ro). Không bao giờ trả cờ YOLO.
6. Cài `readonlyUnsupportedError(id, method, reason)` trả phản hồi lỗi `InvalidParams` với `message` chứa `READONLY_MODE_UNSUPPORTED (<reason>)` và `data: { reason: 'READONLY_MODE_UNSUPPORTED', detail: reason }`. Nhất quán với task 01: `data.reason` là mã lớp lỗi. Ghi chú: solution A ghi `data.reason` là mã chi tiết; chốt tại đây `data.reason = 'READONLY_MODE_UNSUPPORTED'` và `data.detail` là mã chi tiết, và cập nhật mục 2.3 của solution khi triển khai nếu khác.
7. Xuất thêm `resetClaudeFlagCacheForTests()` (chỉ dùng trong test, không dùng ở mã sản phẩm).

## Kiểm thử

Trong `agent-readonly-tool-policy.test.ts` (probe giả, `now` giả, không spawn thật):
- `detects tools and permission-mode and disallowedTools from help text` (văn bản help mẫu có ba cờ).
- `reports FLAG_TOOLS_MISSING when --tools absent` và `FLAG_PERMISSION_MODE_MISSING`.
- `does not match --tools-extra or --no-tools as --tools`.
- `returns null and reports CLAUDE_HELP_UNAVAILABLE when probe throws`.
- `caches a successful probe for 10 minutes keyed by PATH` (probe được gọi một lần; PATH khác gọi lần nữa; sau 10 phút gọi lại).
- `does not cache a failed probe` (lần thứ hai gọi lại probe).
- `single-flights concurrent detections for the same PATH`.
- `buildReadonlyArgs never contains --dangerously-skip-permissions and passes tools as one comma separated element`.
- `readonlyUnsupportedError carries data.reason READONLY_MODE_UNSUPPORTED`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-readonly-tool-policy.test.ts`; sau đó `pnpm test`.

Kiểm với `claude` thật (đánh dấu CHƯA CHẠY): trên dev server có `claude`, chạy `claude --help` bằng tay, dán đoạn chứa ba cờ làm fixture thứ hai trong test (`fixtures` nhỏ trong chính file test, không thêm file riêng) để regex được kiểm trên văn bản thật.

## Tiêu chí hoàn thành

- [ ] `detectClaudeFlags` trả đúng cờ cho help mẫu có và không có `--tools`/`--permission-mode`.
- [ ] Lỗi dò không bị cache; thành công cache đúng 10 phút theo PATH.
- [ ] `buildReadonlyArgs` đúng bằng `['--permission-mode','plan','--tools','Read,Glob,Grep']` và test khẳng định không có cờ YOLO.
- [ ] Không có tham chiếu cờ `claude` nào ngoài `--permission-mode`, `--tools` (và việc chỉ dò `--disallowedTools` để báo cáo).
- [ ] Không có import vòng; không đụng `agent-print-mode-exec.ts`.

## Rủi ro và lưu ý

- Dạng `--tools Read,Glob,Grep` (dấu phẩy) chưa kiểm chứng; nếu `claude` chỉ nhận danh sách cách nhau bằng dấu cách thì phải đổi thành ba phần tử đặt CUỐI argv (sau prompt). Task 04 có thử nghiệm đối kháng trên máy thật để chốt điều này; chỉ đổi hàm `buildReadonlyArgs` và test của nó.
- Regex dò cờ phụ thuộc cách `claude --help` trình bày (có thể thụt lề, có thể gạch nối phụ); fixture thật giảm rủi ro.
- Cache theo `PATH` nghĩa là sau khi nâng cấp `claude` có tối đa 10 phút dùng cờ cũ; chấp nhận.
- Probe là một tiến trình con thật (`execFile`); trong test của handler (task 04) PHẢI tiêm probe giả để không chạy `claude` thật và để khẳng định "không có `spawn` nào cho prompt".
