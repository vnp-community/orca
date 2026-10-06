# AG-CV-SOL-081-A: Lõi bộ chạy kiểm tra `quality.*` (spawn, huỷ cây, run manager, kết quả, RPC, thông báo)

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" là đọc code/CR, chưa chạy gì. "(mới)" = đề xuất của solution này.

**CR:** [CR-CV-081](../../../../../../docs/crs/v7/quality-signals/CR-CV-081-quality-runner-on-agent.md) mục 2.6 đến 2.9, 2.11 (phần cơ chế). Nhóm A trong hai solution của CR-081; nhóm B (catalog, preflight, planning, `listProfiles`): [AG-CV-SOL-081-quality-profile-catalog-and-preflight](./AG-CV-SOL-081-quality-profile-catalog-and-preflight.md).
**Khu vực:** `agent/` (Dev Server Agent), `agent/src/relay/`. **Feature:** `quality-signals`.
**TDD/Spec:** [TDD-AG-01](../../../../tdd/v5/01-architecture.md), [TDD-AG-04 mục 7-8](../../../../tdd/v5/04-handshake-session.md) (capabilities, `stop()`), [TDD-AG-05](../../../../tdd/v5/05-tool-registry.md) (`runToolCommand`), [TDD-AG-07 mục 5, 9](../../../../tdd/v5/07-jsonrpc-dispatch.md) (mã lỗi, `makeNotifier`), [TDD-AG-02](../../../../tdd/v5/02-wire-protocol.md), [api/agent-rpc-catalog-runtime.md](../../../../api/agent-rpc-catalog-runtime.md), [api/gaps-and-findings.md](../../../../api/gaps-and-findings.md) (#8 timeout không thực thi).
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/`.
**Đánh số task:** AG-CV-TASK-081-01 đến 09 (solution này); 10 đến 18 thuộc solution catalog/preflight.

## 1. Hợp đồng áp dụng

| Mục hợp đồng ([`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md)) | Áp vào |
|---|---|
| §2.1 tham số (`workspaceRoot` bắt buộc, tham số lạ bị từ chối, cấm `args|argv|command|cmd|cwd|env|repo|shell|timeout|tool`, `_trace` ngoại lệ) | `quality-method-table.ts` (task 08), test phản chiếu schema |
| §2.4 "Quality" (env con bắt đầu từ rỗng, allowlist, deny pattern, `CI=1 NO_COLOR=1 FORCE_COLOR=0 TERM=dumb`, Go/Node) | task 01 |
| §2.5 timeout: `quality.run|runStatus|cancel|results|coverage` 10 s ở agent; `listProfiles` 40 s | task 08 |
| §3.2 mã lỗi `PROFILE_UNKNOWN`, `ENV_NOT_READY`, `RUN_IN_PROGRESS` (`reason: worktree_busy|queue_full`), `RUN_NOT_FOUND`, `RUN_CANCELLED`, `INVALID_PARAMS`, `PATH_NOT_ALLOWED`, `TOOL_UNAVAILABLE` | task 08 |
| §5.2 `quality.run` (trả ngay, tuần tự, `runTimeoutMs` 45 phút, `maxOutputBytes` 32/64 MiB, `nice 10`, `SIGTERM → 5 s → SIGKILL`, cổng nặng `ORCA_HEAVY_JOBS=1`, chờ ≤ 10 phút → `skipped gate_timeout`) | tasks 03-06 |
| §5.3 `runStatus`; trạng thái bước/run; `interrupted`; nhật ký `~/.orca/quality/runs/<runId>.json` ≥ 50 bản, thư mục `0700` | task 06 |
| §5.4 `cancel` (idempotent, ≤ 15 s cả nhóm biến mất, giữ phát hiện đã parse đánh `partial`) | tasks 03, 06 |
| §5.5 `results` (`findings|steps|log`, trang ≤ 500 mục, ≤ 1 MiB; TTL 1 h, 20 run/worktree; ngân sách 5 000/bước, 20 000/run) | task 07 |
| §6.3, §6.4 `quality.progress`, `quality.finished` mang `workspaceRoot` (PQ-17) | task 09 |
| §1.3 capability `quality` | task 09 |
| PQ-17, PQ-21, PQ-27 | `workspaceRoot` ở mọi method; `runId` không thuộc worktree → `RUN_NOT_FOUND` |
| §8.3 (1), (5) | Trích PQ; không có `command|argv|args|env|cwd|timeout` |

## 2. Lệch giữa CR và hợp đồng

| # | CR-CV-081 | Hợp đồng | Solution theo |
|---|---|---|---|
| 1 | `quality.progress/finished` không có `workspaceRoot` (2.6) | PQ-17: **mọi** thông báo có `workspaceRoot` | Hợp đồng |
| 2 | `quality.runStatus/cancel/results` chỉ nhận `{runId}` (README 3.10) | PQ-21: **mọi** method nhận `workspaceRoot`; `runId` không thuộc worktree đó → `RUN_NOT_FOUND` | Hợp đồng |
| 3 | `quality.listProfiles` trả `missing[].searched[]` (2.4) | §5.1: `missing[]` = `{check, reason, hint, built?, required?}`; `display` không chứa đường dẫn ngoài `workspaceRoot` | Hợp đồng: **không** trả `searched[]` (chỉ log cục bộ) |
| 4 | `RUN_NOT_FOUND` là "mã mới" | §3.2 đã có | Hợp đồng |
| 5 | `percent = completedSteps/stepCount` | §6.3 ví dụ `stepIndex:1, stepCount:6, percent:16` khi bước 1 đang chạy: **không khớp** định nghĩa (đang chạy bước 1 thì đã xong 0 bước) | Định nghĩa (đã xong/tổng, làm tròn xuống); ví dụ coi là minh hoạ lỏng. Ghi ở câu hỏi mở 1 |
| 6 | Trạng thái bước `skipped` kèm lý do (`gate_timeout`, không có tệp) | §5.3 chỉ có `status`, `exitCode`, `durationMs`, `findings`, `truncated`, `envMissing` — **không có trường lý do skip** | Đề xuất thêm `skipReason` (tuỳ chọn) vào `runStatus.steps[]` và `QualityStepResult`; **cần sửa hợp đồng** (câu hỏi mở 2). Trong lúc chờ: cài ở agent, backend bỏ qua trường lạ |
| 7 | Run quá `runTimeoutMs`: CR không nói số phận các bước chưa chạy | §5.2 chỉ nêu giới hạn | Bước chưa chạy → `skipped` (`skipReason:"run_timeout"`), run `failed`, `errorCode:"CODEINTEL_TIMEOUT"` (quyết định của solution; cần xác nhận, câu hỏi mở 2) |
| 8 | `quality.results` của run `interrupted` | §5.5 không nói | Trả trang rỗng hợp lệ (`totalCount:0`), không lỗi (kết quả chỉ giữ trong bộ nhớ; câu hỏi mở 3) |
| 9 | Windows `taskkill` được thiết kế nhưng chưa kiểm chứng | §5 đầu: Windows → `TOOL_UNAVAILABLE reason="unsupported_platform"` | Hợp đồng; nhánh `taskkill` vẫn viết và test bằng mock |
| 10 | `quality` capability khi "có ≥ 1 profile `ready`" | §1.3 giống; nhưng `ready` phụ thuộc `workspaceRoot` (node_modules...), còn handshake không có `workspaceRoot` | Đề xuất mức máy chủ (task 09): có ≥ 1 binary catalog tìm thấy trên `qualityToolPath`; ghi câu hỏi mở 4 |
| 11 | `quality.*` không có ở `relay-ssh` (CR 2.11) | §5 đầu "relay-ssh/Part B không có quality.*" nhưng §1.1 nói Go chạy `--stdio` = Part A | `--stdio` dùng cùng `createSession` (đã đọc `agent-connection-stdio.ts:191`), nên method có sẵn qua `--stdio`; Part B (`desktop/src/relay`) không làm. Câu hỏi mở 5 |

## 3. Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `AG-CV-SOL-001-codeintel-agent-foundation` | **Điều kiện trước**: `agent-rpc-dispatch-codeintel.ts` (mẫu), `codeintel-errors.ts` (`CodeIntelError`, `toJsonRpcError`), `codeintel-limits.ts`, `codeintel-repo-resolution.ts`. Các file này **chưa tồn tại** (đã `ls`). Task 08 thêm 5 mã lỗi quality vào `codeintel-errors.ts` (CR-081 2.11) |
| `AG-CV-SOL-004-reindex-and-index-notifications` | `codeintel-notification-sink.ts` (notifier hiện hành, hợp đồng §6); task 05 sửa 1 dòng ở `codeintel-reindex-job.ts` để lấy cổng nặng; chữ ký `emit` do SOL-004 chốt |
| `AG-CV-SOL-082-quality-parsers-and-fingerprint` | Cung cấp `QualityParser` registry và `quality-finding-pipeline.ts` mà executor/manager gọi (nối bằng interface `StepParser` ở task 04; SOL-082 task 09 cắm vào) |
| `AG-CV-SOL-081-quality-profile-catalog-and-preflight` | Cung cấp `RunPlan`/`PlannedStep` (task 15, 18); lõi chỉ định nghĩa kiểu |
| `BE-CV-SOL-023-infra-fleet-codeintel-transport` | Timeout Go theo method (§2.5), chuyển `quality.progress|finished`, giữ `error.data` |
| `BE-CV-SOL-082-quality-run-storage-and-ingest`, `BE-CV-SOL-085-…` | Gọi `quality.run`, `runStatus`, `results` (phân trang 500), nhận `quality.finished`; ghi `quality_findings` **trước**, `Finish` sau |
| `AG-CV-SOL-073-agent-kill-switch` | `ORCA_QUALITY_RUN=off` và `ORCA_CODEINTEL_DISABLED=1` (§2.4) |
| `AG-CV-SOL-072-security-tests-agent` | Tái dùng test canary secret, test phản chiếu schema |
| Thứ tự hợp đồng §7.2: `001 → 081 → 082-AG → 083-AG, 084, 091-AG`; đợt 7 |

## 4. Re-verify (đã đọc, 2026-10-06)

Đã đọc: `agent/src/relay/agent-rpc-dispatch.ts` (toàn bộ, `route :297-382`, `makeNotifier :280`, `makeError :408`, `extractTraceFields :92`), `agent-rpc-dispatch-misc.ts` (mẫu dispatcher, dòng 1-90), `agent-session.ts` (toàn bộ, `stop() :226`), `agent-session-capabilities.ts`, `agent-session-handshake.ts`, `agent-config.ts` (`buildToolPath :44`, `toolEnv :81-88`), `agent-tool-registry.ts` (`runToolCommand :72-113`), `agent-exec-handler.ts` (`killProcessTree :72-88`), `pty-daemon-client.ts:84` (`detached: true`), `agent-connection-stdio.ts`, `agent/src/shared/agent-wire-protocol.ts` (`AgentErrorCode :35-52`, `AgentCapability :58`), `agent/vitest.config.ts`, `agent/package.json`, `.oxlintrc.json:89-101`.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| `runToolCommand` | `spawn(..., {shell:false})` pipe, timeout chỉ `child.kill('SIGTERM')`, không `detached`, không giới hạn đầu ra, stdout vào bộ nhớ | **Không dùng được** cho runner; viết executor riêng (task 04), không sửa `runToolCommand` (tránh đổi `tools/call`) |
| `killProcessTree` | Không xuất khẩu; POSIX chỉ `child.kill('SIGKILL')` một tiến trình, Windows `exec('taskkill /pid N /T /F')` (dạng chuỗi, qua shell) | Viết `quality-process-tree-kill.ts` mới: POSIX `process.kill(-pid)`; Windows dùng `execFile('taskkill', ['/pid', N, '/T', '/F'])` (không shell) |
| `toolEnv` | `{...process.env, PATH, HOME, ANTHROPIC_API_KEY, GITHUB_TOKEN, GH_TOKEN}` (toàn bộ env cộng khoá) | Env con xây từ rỗng (task 01); không bao giờ truyền `config.toolEnv` |
| `buildToolPath` | `~/.local/bin, ~/bin, /usr/local/bin, /usr/bin, /bin, /usr/sbin, /snap/bin` (không có `~/go/bin`; máy khảo sát: `golangci-lint`, `buf`, `opa` ở `/home/ubuntu/go/bin`) | `qualityToolPath` bổ sung (task 16, nhóm B); lõi chỉ nhận chuỗi PATH |
| Notifier | `makeNotifier(ws, state)` bỏ qua im lặng khi `ws.readyState !== 1`; mỗi dispatcher tự bind | Dùng `codeintel-notification-sink.ts` (SOL-004) để run nền còn gửi sau khi request kết thúc |
| `agent-session.stop()` | `clearInterval`, `scheduleAgentSpawnGracePeriod`, `notifyDaemonSessionClosed`, `cleanupAgentWatches` | **Không** huỷ run quality khi WS đóng (hợp đồng §5.2/§6.4 "run tiếp tục"); chỉ gỡ notifier (qua sink) |
| `AgentCapability` | Union `'pty'|'fs'|'git'|'preflight'` nhưng `buildCapabilities` trả mảng chuỗi tự do (đã đọc) | Task 09 nới kiểu thành `string` hoặc thêm `'quality'`; không đổi hành vi |
| `AGENT_PROTOCOL_VERSION` | `'1'`, thêm method là additive | Không đổi |
| Test hiện có | `agent-rpc-dispatch-misc.test.ts` dùng `MockWs`; `agent/src/relay/__tests__/` có `agent-rpc-dispatch.test.ts`, `agent-session.test.ts` | Mẫu test dispatcher |
| Tiến trình `nice` | `os.setPriority(pid, 10)` có sẵn trong Node (đa nền tảng) | Task 04 |

| Bảng "Correction relative to CR" | |
|---|---|
| CR-081 1.3: "`killProcessTree` ở `agent-exec-handler.ts` dùng `taskkill /pid /T /F`" | Đúng nhưng bằng `exec(\`taskkill …\`)` chuỗi (qua shell), không phải `execFile`; solution dùng `execFile` |
| CR-081 2.7 mô tả `process.kill(-pid, 0)` ném `ESRCH` để kiểm tra nhóm đã biến mất | Đúng trên Linux/macOS khi `detached:true` làm tiến trình thành nhóm trưởng (`pgid == pid`); không đúng nếu `setsid` bên trong; chưa kiểm chứng trên macOS |
| CR-081 viết `NODE_OPTIONS` "từ profile" | Chấp nhận; `NODE_OPTIONS` được kiểm không chứa `--require`/`--import` (tránh nạp mã tuỳ ý) — thêm ràng buộc, chưa có trong CR |

## 5. Giải pháp

### 5.1 Cây file (mới, phẳng, `agent/src/relay/`)

```
quality-child-env.ts / .test.ts                task 01
quality-output-redaction.ts / .test.ts         task 02
quality-process-tree-kill.ts / .test.ts        task 03
quality-run-types.ts                           task 04 (PlannedStep, StepExecResult, RunRecord...)
quality-run-step-executor.ts / .test.ts        task 04
agent-heavy-job-gate.ts / .test.ts             task 05
quality-run-manager.ts / .test.ts              task 06
quality-run-id.ts, quality-run-journal.ts / .test.ts   task 06
quality-results-store.ts / .test.ts            task 07
quality-limits.ts                              task 06 (hằng, đọc ORCA_QUALITY_*)
agent-rpc-dispatch-quality.ts / .test.ts       task 08
quality-method-table.ts                        task 08
quality-progress-emitter.ts / .test.ts         task 09
```
Sửa nhỏ: `agent-rpc-dispatch.ts` (một khối `dispatchQualityRpc` sau `dispatchCodeIntelRpc`, một nhánh `extractTraceFields`), `agent-session-capabilities.ts` (`quality`), `agent/src/shared/agent-wire-protocol.ts` (`AgentCapability`), `codeintel-errors.ts` (5 mã), `codeintel-reindex-job.ts` (1 dòng cổng nặng).

### 5.2 Kiểu dùng chung của lõi (`quality-run-types.ts`)

```ts
export type PlannedStep = {
  id: string                       // "ts-lint" | "go-test:services/project-service"
  profileId: string
  title: string
  cwd: string                      // tuyệt đối, realpath nằm trong worktree
  file: string                     // binary đã phân giải (tuyệt đối); argv KHÔNG chứa argv[0]
  argv: readonly string[]          // đã thay mẫu; không shell
  env: NodeJS.ProcessEnv           // đầu ra buildQualityChildEnv
  timeoutMs: number
  maxOutputBytes: number
  heavy: boolean
  exit: { ok: readonly number[]; findings: readonly number[] }
  parserKey: string | null         // "oxlint@json" (SOL-082)
  extraOutputPath: string | null   // {tmp:...}
  scopeFiles: ReadonlySet<string> | null
  skipReason: string | null        // bước đã bị loại ở lập kế hoạch (vd "no_files")
  envMissing: readonly QualityEnvMissing[]
}
export type StepExecResult = {
  kind: 'exited' | 'timeout' | 'cancelled' | 'output_too_large' | 'spawn_error' | 'gate_timeout'
  exitCode: number | null; signal: NodeJS.Signals | null; durationMs: number
  stdoutPath: string; stderrPath: string; stdoutBytes: number; stderrBytes: number
  orphanSuspected: boolean
}
export type StepParser = (input: StepParserInput) => Promise<StepParseOutcome>   // SOL-082 cung cấp bản thật
```

### 5.3 Executor (task 04), quy tắc cố định

`spawn(file, argv, { cwd, env, shell:false, detached: process.platform !== 'win32', stdio: ['ignore', fdOut, fdErr], windowsHide:true })`; stdout/stderr ghi ra tệp trong `<os.tmpdir()>/orca-quality-<uid>/<runId>/` (`mkdtemp`, thư mục `0700`, tệp `0600`, cờ `wx`), không qua pipe; `os.setPriority(pid, 10)` (nuốt lỗi, log); mỗi 500 ms `fs.stat` kiểm `maxOutputBytes` (vượt → diệt cây, `output_too_large`, **không** cắt tệp rồi đưa cho parser); quá `timeoutMs` → diệt cây, `timeout` (vẫn trả đường dẫn tệp để parser thử đọc phần đã ghi, đánh `partial`). `argv[0]` luôn là `file`; không có đường nào để tham số RPC chạm vào argv ngoài các mẫu đã kiểm ở planning.

### 5.4 Máy trạng thái run (task 06)

`queued → running → (succeeded|failed|cancelled)`; thêm `cancelling`, `interrupted` (chỉ từ nhật ký sau khởi động lại). Khoá theo `realpath(workspaceRoot)`; chạy tuần tự các bước; `maxConcurrentRuns = 1` toàn agent, hàng đợi ≤ `ORCA_QUALITY_QUEUE_MAX` (4). Run thứ hai cùng worktree → `RUN_IN_PROGRESS reason="worktree_busy"` kèm `runId`; hàng đợi đầy → `reason="queue_full"`. Bước `failed|timeout|env_not_ready` làm run `failed`; có phát hiện **không** làm run `failed`. `dirtyFingerprint` tính lúc bắt đầu (trả trong `quality.run`) và lúc kết thúc: khác nhau → `workTreeChangedDuringRun:true`.

### 5.5 Mã lỗi mới trong `codeintel-errors.ts` (task 08)

`CODEINTEL_PROFILE_UNKNOWN` (-32602), `CODEINTEL_ENV_NOT_READY` (-32000), `CODEINTEL_RUN_IN_PROGRESS` (-32000), `CODEINTEL_RUN_NOT_FOUND` (-32602), `CODEINTEL_RUN_CANCELLED` (-32000), đúng bảng §3.2. `error.code` số dùng lại `AgentErrorCode` (không thêm số mới); `error.data.code` mang chuỗi.

## 6. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | Executor riêng, không sửa `runToolCommand` | `tools/call` dùng chung; cần nhóm tiến trình, tệp đầu ra, diệt cây | Mở rộng `runToolCommand` |
| 2 | Env con từ rỗng + allowlist + deny pattern | `toolEnv` chứa toàn bộ `process.env` và khoá AI (đã đọc) | Lọc bớt `toolEnv` |
| 3 | Kết quả chỉ trong bộ nhớ (TTL 1 h) + nhật ký tóm tắt | Phát hiện đã che, nhưng không ghi đĩa để giảm bề mặt rò rỉ | JSONL trên đĩa |
| 4 | Run tiếp tục khi WS đóng | Hợp đồng §5.2/§6.4; backend nối lại gọi `runStatus` | Huỷ run khi mất kết nối |
| 5 | `percent` = bước xong/tổng hoặc `null` | Không bịa tiến độ | Ước lượng theo thời gian |
| 6 | Không tự diệt tiến trình mồ côi sau khởi động lại | Không xác minh được `pgid` còn là của ta (CR-081 2.7) | Diệt theo `pgid` trong nhật ký |
| 7 | `skipReason` thêm vào trạng thái bước (đề xuất) | Cần phân biệt `gate_timeout`/`run_timeout`/`no_files` | Nhồi vào `failureKind` |

## 7. Tiêu chí chấp nhận

- [ ] Không có tham số `command|argv|args|env|cwd|timeout|tool|shell` nào được chấp nhận ở bất kỳ `quality.*` (test phản chiếu schema); khoá lạ → `CODEINTEL_INVALID_PARAMS` (`data.field`).
- [ ] `quality.run` với `profile:"rm -rf /"` → `-32602 CODEINTEL_PROFILE_UNKNOWN` (`data.available[]`), **không spawn** tiến trình nào.
- [ ] Env của `spawn` không chứa biến nào khớp `/(TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|API_?KEY|PRIVATE|DSN|AUTH|COOKIE|SESSION)/i`, `ORCA_*`, `AWS_*`, `GOOGLE_*`, `SSH_AUTH_SOCK`; test với `process.env` chứa `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `AGENT_TOKEN`, `FOO_SECRET`.
- [ ] Huỷ giữa chừng một bước sinh 2 con cháu: sau ≤ 15 s cả nhóm biến mất (`process.kill(-pid, 0)` ném `ESRCH`), run `cancelled`, `quality.cancel` lặp lại không lỗi.
- [ ] Đầu ra vượt `maxOutputBytes` → diệt cây, bước `failed` (`failureKind:"output_too_large"`), tệp tạm xoá khi hết TTL, thư mục `0700`.
- [ ] Run thứ hai cùng worktree → `RUN_IN_PROGRESS` đúng `runId`; hàng đợi vượt 4 → `queue_full`.
- [ ] Mất WS giữa chừng: run tiếp tục; sau nối lại `runStatus` đúng; khởi động lại agent đánh dấu `interrupted` (không tự chạy lại).
- [ ] `quality.progress` ≤ 1/giây/run, `percent` số hoặc `null`, `message` đã che; `quality.finished` mang `workspaceRoot`.
- [ ] Bước `heavy` và `codeintel.reindex` không chạy đồng thời khi `ORCA_HEAVY_JOBS=1`.
- [ ] `ORCA_QUALITY_RUN=off` và `win32` → `CODEINTEL_TOOL_UNAVAILABLE` (`reason: quality_disabled|unsupported_platform`).
- [ ] Không file nào > 300 dòng; không `max-lines` disable; không tên `helpers/utils/common/misc`.

## 8. Kiểm thử

| File test (mới) | Nội dung |
|---|---|
| `quality-child-env.test.ts` | allowlist, deny pattern, `allowExtra` bị từ chối, `NODE_OPTIONS` hợp lệ |
| `quality-output-redaction.test.ts` | token, `user:pass@`, giá trị env, đường dẫn tuyệt đối |
| `quality-process-tree-kill.test.ts` | POSIX kiểm cây con/cháu thật; Windows `taskkill` mock |
| `quality-run-step-executor.test.ts` | script Node giả qua PATH tạm: đầu ra lớn, timeout, exit lạ, sinh con cháu, bỏ qua SIGTERM |
| `agent-heavy-job-gate.test.ts` | giới hạn, hàng đợi FIFO, thời gian chờ, giải phóng khi lỗi |
| `quality-run-manager.test.ts` | khoá worktree, hàng đợi, `interrupted`, mất WS, `workTreeChangedDuringRun`, run timeout |
| `quality-results-store.test.ts` | phân trang, TTL, `view=log` đã che |
| `agent-rpc-dispatch-quality.test.ts` | định tuyến, tham số lạ, hình dạng JSON-RPC, `MockWs` (mẫu `agent-rpc-dispatch-misc.test.ts`) |
| `quality-progress-emitter.test.ts` | throttle 1/giây, luôn phát khi đổi `stage`, WS đóng thì bỏ |

Lệnh gốc: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-child-env.test.ts src/relay/quality-output-redaction.test.ts src/relay/quality-process-tree-kill.test.ts src/relay/quality-run-step-executor.test.ts src/relay/agent-heavy-job-gate.test.ts src/relay/quality-run-manager.test.ts src/relay/quality-results-store.test.ts src/relay/agent-rpc-dispatch-quality.test.ts src/relay/quality-progress-emitter.test.ts`; sau cùng `pnpm test`. CI hiện không chạy test `agent/` (hợp đồng §10): job `code-intel-contract` của CR-CV-070.

## 9. Rủi ro và chưa kiểm chứng

- **Chưa chạy bất kỳ tiến trình công cụ nào** ở dev server; mọi giới hạn (45 phút, 32/64 MiB, `nice 10`, `ORCA_HEAVY_JOBS=1`) là giả định chưa đo (hợp đồng O-15).
- `kill(-pid)` không diệt tiến trình tự `setsid`/double-fork; test Docker/Playwright đã loại khỏi catalog.
- Chạy mã không tin cậy (`vitest`, `go test`) ở mức L0 không chặn đọc credential/mạng (CR-081 2.10; `ORCA_QUALITY_ISOLATION=bwrap` P1, chưa thử). **Ngoài phạm vi solution**: tham số `trust` chưa thêm (CR-081 Q2).
- Windows: `taskkill`, `PATHEXT`, shim `.cmd` chưa kiểm chứng; MVP trả `unsupported_platform`.
- SSH: `--stdio` dùng chung mã; thông báo trễ 50-200 ms; Part B không có.
- `os.setPriority` có thể bị từ chối (EPERM trong container): nuốt lỗi và log.
- Đặt `GOMAXPROCS`/`vitest --maxWorkers` theo `cores/4` giả định.

## 10. Câu hỏi mở

1. Sửa ví dụ §6.3 (percent 16) hay định nghĩa? Mặc định: theo định nghĩa.
2. Thêm `skipReason` và hành vi `run_timeout` vào hợp đồng §5.3/§5.5?
3. `results` của run `interrupted`: trang rỗng (mặc định) hay `RUN_NOT_FOUND`?
4. `quality` capability mức máy chủ (có binary) hay theo repo (không khả thi lúc handshake)?
5. `--stdio` (Part A qua SSH) có cho `quality.*` ở v7 không? Mặc định: có sẵn, chưa kiểm thử (CR riêng).
