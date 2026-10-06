# AG-REQ-TASK-033-09: Lõi báo cáo năng lực dev server (`agent-capability-report.ts`)

**From Solution:** [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) mục 2.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-capability-report.ts` (mới), `agent/src/relay/agent-capability-report.test.ts` (mới), `agent/src/relay/agent-build-version.ts` (mới)
**Depends on:** [02](./AG-REQ-TASK-033-02-readonly-tool-policy.md) (`detectClaudeFlags`)
**Status:** [ ] TODO

## Context

CR-033 mục 2.6 thêm RPC `agent.capabilities` để backend (infra-fleet, rồi `ReadinessGate` của CR-REQ-029 và CR-REQ-026) biết dev server có công cụ gì, `claude` đã đăng nhập chưa và còn bao nhiêu tài nguyên, thay cho các RPC dò rời rạc hiện có (`preflight.check`, `preflight.detectAgents` nhận lệnh do người gọi gửi, `host.capabilities` chỉ cho WSL/pwsh/git-bash, `agent.exec` chạy binary bất kỳ tối đa 5 phút). Task này viết lõi dò, chưa đăng ký RPC (task 10).

Đã đọc: `agent-config.ts` (`toolPath`, `toolEnv`, `workDir`); `agent-spawn-env.ts` (`execPrompt` chạy `claude` với `PATH = config.toolPath`); `agent-entry.ts` (`__AGENT_VERSION__` do esbuild `define`, khai báo ở `agent/src/types/build-constants.d.ts`); `agent-rpc-dispatch-misc.ts` (`host.capabilities` làm mẫu). Chênh với CR đã tìm ra: (1) `go` dùng `go version`, không có `--version`; (2) mọi dò `claude` phải dùng `PATH = config.toolPath`, đúng như lúc `execPrompt` chạy; (3) `ai.complete` đọc khoá chỉ từ `process.env`, nên `env.present` đọc `process.env`.

Nguyên tắc an toàn (CR): danh sách cho phép CỨNG `go node pnpm npm git openspec claude codegraph gitnexus rg make semgrep`; mỗi công cụ MỘT lệnh phiên bản cố định; KHÔNG BAO GIỜ chạy lệnh do người gọi gửi; chỉ trả `present` cho biến môi trường, không trả giá trị; `claude auth status --json` chỉ lấy một boolean.

## Việc cần làm

1. Tạo `agent-build-version.ts`: `export const AGENT_BUILD_VERSION: string = typeof __AGENT_VERSION__ !== 'undefined' ? __AGENT_VERSION__ : '0.0.0-dev'`. Không import `agent-entry.ts` (nó chạy `main` khi nạp, và import vòng).
2. Tạo `agent-capability-report.ts` với:
   ```ts
   export const CAPABILITY_TOOL_ALLOWLIST: ReadonlyArray<{ id: string; args: readonly string[] }> = [
     { id: 'go', args: ['version'] }, { id: 'node', args: ['--version'] }, { id: 'pnpm', args: ['--version'] },
     { id: 'npm', args: ['--version'] }, { id: 'git', args: ['--version'] }, { id: 'openspec', args: ['--version'] },
     { id: 'claude', args: ['--version'] }, { id: 'codegraph', args: ['--version'] },
     { id: 'gitnexus', args: ['--version'] }, { id: 'rg', args: ['--version'] },
     { id: 'make', args: ['--version'] }, { id: 'semgrep', args: ['--version'] }
   ]
   export const DEFAULT_ENV_NAMES = ['ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GOOGLE_API_KEY'] as const
   export const ENV_NAME_PATTERN = /^[A-Z][A-Z0-9_]{0,63}$/
   export type CapabilityReportParams = { tools?: string[]; envNames?: string[]; refresh?: boolean }
   export type CapabilityReportDeps = { run?: RunVersionCommand; now?: () => number; platform?: NodeJS.Platform;
     statfs?: typeof import('node:fs/promises').statfs; detectFlags?: typeof detectClaudeFlags; env?: NodeJS.ProcessEnv }
   export async function buildCapabilityReport(params, config: AgentConfig, deps?): Promise<CapabilityReport>
   export function validateCapabilityParams(raw: Record<string, unknown>):
     { ok: true; value: Required<...> } | { ok: false; code: 'INVALID_CAPABILITY_PARAMS' | 'TOO_MANY_ENV_NAMES'; message: string }
   ```
   (Chữ ký `run` mặc định bọc `execFile(bin, args, { timeout: 3000, env, windowsHide: true })`, không shell.)
3. `validateCapabilityParams`: `tools` phải là mảng chuỗi, `envNames` mảng chuỗi tối đa 64 phần tử, `refresh` boolean; sai kiểu thì `INVALID_CAPABILITY_PARAMS`; quá 64 tên thì `TOO_MANY_ENV_NAMES`. Phần tử `tools` ngoài danh sách đi vào `unknownTools` (không chạy); tên `envNames` không khớp `ENV_NAME_PATTERN` đi vào `rejectedEnvNames` (không đọc).
4. Dò công cụ: với mỗi công cụ được chọn chạy song song; `env = { ...process.env, PATH: config.toolPath }`. Thành công thì `{ id, installed: true, version }` với `version` là kết quả khớp đầu tiên của `/\d+(?:\.\d+){1,3}[\w.+-]*/` trong dòng đầu, tối đa 64 ký tự (không khớp thì bỏ `version`). `ENOENT` thì `{ id, installed: false }`. Hết giờ hoặc lỗi khác thì `{ id, installed: null }`. Trên `win32`, khi `ENOENT` thử thêm `<id>.cmd` rồi mới kết luận (chưa kiểm chứng).
5. `claude`: `installed`, `version` lấy từ mục công cụ `claude`; `flags` từ `detectClaudeFlags({ ...process.env, PATH: config.toolPath })` (hàm của task 02, dùng chung cache); `auth` từ `claude auth status --json`: parse JSON, nếu `typeof parsed.loggedIn === 'boolean'` thì `logged_in`/`logged_out`, còn lại `unknown`. Tên trường `loggedIn` CHƯA KIỂM CHỨNG; không bao giờ sao chép email hay tổ chức. `claude` chưa cài thì `auth: 'unknown'`, `flags` toàn `false`.
6. `host`: `platform`, `arch`, `nodeVersion: process.version`, `cpuCount: os.cpus().length`, `memTotalMb`, `memFreeMb` (làm tròn xuống, MiB), `diskFreeMb` từ `fs.statfs(config.workDir)` (`bavail * bsize`, làm tròn xuống, MiB; lỗi thì bỏ trường), `loadAvg1: os.loadavg()[0]` (0 trên Windows).
7. `env`: `[{ name, present }]` với `present = typeof process.env[name] === 'string' && process.env[name] !== ''`. KHÔNG bao giờ đưa giá trị vào đối tượng, log hay `span`.
8. Giới hạn thời gian: tổng 8 giây (`Promise.race` với bộ hẹn giờ); mục chưa xong có `installed: null` và đặt `partial: true`. Mỗi lệnh 3 giây.
9. Cache 60 giây trong tiến trình, khoá `JSON.stringify([sorted tools, sorted envNames])`; `refresh=true` bỏ cache; các lời gọi đồng thời cùng khoá dùng chung một Promise (single flight). `probedAt` là ISO của lúc dò thật.
10. Xuất `agent: { buildVersion: AGENT_BUILD_VERSION, protocolVersion: AGENT_PROTOCOL_VERSION }` (hằng số này thuộc task 12; trong task này khai báo `AGENT_PROTOCOL_VERSION = 2` ở `agent-protocol-features.ts` ở dạng tối thiểu hoặc truyền qua `deps` rồi nối ở task 12; chọn khai báo tối thiểu để không phụ thuộc vòng).

## Kiểm thử

`agent-capability-report.test.ts` (`run` giả, `statfs` giả, `now` giả; không chạy lệnh thật):
- `runs only allowlisted tools and puts everything else in unknownTools` (đưa `"rm -rf /"`, `"curl"`; khẳng định `run` không được gọi với chúng).
- `uses go version and --version for the rest`.
- `extracts a semantic version from the first output line and caps it at 64 chars`.
- `marks ENOENT as installed:false and timeout as installed:null with partial:true`.
- `rejects envNames that do not match the pattern and lists them in rejectedEnvNames` (`"x; rm -rf"`, `"lower"`, `"A".repeat(65)`).
- `never includes any env value in the report` (đặt `process.env.ANTHROPIC_API_KEY = 'sk-secret-123'` bằng `vi.stubEnv`, khẳng định `JSON.stringify(report)` không chứa `sk-secret-123`).
- `treats an empty string env var as not present`.
- `rejects more than 64 envNames with TOO_MANY_ENV_NAMES`.
- `reads claude auth as a single boolean and never copies other fields` (đầu ra giả có `email`; khẳng định không có trong báo cáo).
- `reports auth unknown when claude auth status output is not JSON or lacks loggedIn`.
- `caches for 60 seconds, bypasses with refresh, and single-flights concurrent calls` (đếm số lần `run`).
- `uses config.toolPath as PATH for probes`.
- `finishes within the 8s overall budget and sets partial when a probe hangs` (giờ giả).
- `omits diskFreeMb when statfs fails`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-capability-report.test.ts`; rồi `pnpm test`.
Kiểm với máy thật (CHƯA CHẠY): gọi `buildCapabilityReport` bằng một script `tsx`/vitest tạm trên dev server có `claude`, đối chiếu với `claude --version`, `claude auth status --json` chạy tay. Chỉ dùng lệnh CR-033 đã nêu.

## Tiêu chí hoàn thành

- [ ] Không có đường nào chạy lệnh ngoài `CAPABILITY_TOOL_ALLOWLIST` (test khẳng định bằng `run` giả).
- [ ] Không có giá trị biến môi trường nào trong báo cáo, log hay trace.
- [ ] `go` dùng `go version`; mọi dò dùng `PATH = config.toolPath`.
- [ ] `partial` và `installed: null` hoạt động khi hết giờ; tổng không quá 8 giây.
- [ ] Cache 60 giây và single flight đúng; `refresh` bỏ cache.

## Rủi ro và lưu ý

- `claude auth status --json`: tên trường và việc có cần mạng chưa kiểm chứng; nếu lệnh chậm sẽ làm `partial` thường xuyên. Có thể tách `claude.auth` thành lần dò riêng với hạn 3 giây (đã áp dụng).
- Quét 12 lệnh trên máy chậm; cache 60 giây và `partial` là biện pháp, chưa đo.
- Windows: `*.cmd` shim không chạy được qua `execFile` không shell nếu không thêm hậu tố; đã thử `.cmd`, chưa kiểm chứng.
- `process.env` của tiến trình agent có thể khác môi trường shell của người dùng (khi chạy dưới systemd); đó là lý do dùng `config.toolPath`; nhưng biến như `ANTHROPIC_API_KEY` đặt trong shell không có ở đây; `present` phản ánh đúng thứ `ai.complete` thấy.
- Lệch tên `GEMINI_API_KEY` (agent đặt) với `GOOGLE_API_KEY` (`ai.complete` đọc): câu hỏi mở 4 của solution; mặc định chỉ kiểm ba tên của CR.
