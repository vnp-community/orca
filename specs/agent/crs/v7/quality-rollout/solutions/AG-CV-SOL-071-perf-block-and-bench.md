# AG-CV-SOL-071: Khối `perf`, đo RSS, kiểm tra giới hạn đồng thời và benchmark phía `agent/`

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Đã hoàn thành toàn bộ các task 071-01 đến 071-07, kiểm thử tự động xác nhận qua vitest, đạt 100% tiêu chí chấp nhận.

**CR:** [CR-CV-071](../../../../../../docs/crs/v7/quality-rollout/CR-CV-071-performance-budgets-metrics-tracing.md) (P1, Medium) — chỉ phần phía `agent/` (CR-071 mục 2.2 dòng "Agent (TS)", 2.5, và hạn mức đồng thời của mục 2.1)
**Service:** `agent/`
**Hợp đồng chuẩn tắc:** [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (chính), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md), [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**TDD tham chiếu:** [v5/05-tool-registry](../../../../tdd/v5/05-tool-registry.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/08-deployment](../../../../tdd/v5/08-deployment.md)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/`

## 0. Hợp đồng áp dụng

| Mục hợp đồng | Áp dụng |
|---|---|
| Agent contract §2.2 (`perf`: `totalMs, queueWaitMs, cliCalls, cli[{tool,command,ms,stdoutBytes,rssPeakKb}], parseMs, truncated`; `command` chỉ là hằng whitelist `cypher|context|impact|query|status|callers|callees|files|affected|detect-changes`; "chỉ cho backend ghi metric/span; KHÔNG ra UI, KHÔNG vào cache snapshot") | Hình dạng và quy tắc của khối `perf` (task 01, 03). `perf` **thay** `toolTimingsMs` của CR-001 (PQ-19) |
| Agent contract §2.3 (3 tiến trình đồng thời toàn agent; `gitnexus` ≤ 2/dev server; `codegraph` ≤ 3; hàng đợi ≤ 16 mục; chờ ≤ 10 s rồi `CODEINTEL_TIMEOUT data.reason="queue_wait"`; cache ngắn hạn 60 s ≤ 64 mục ≤ 32 MiB singleflight; stdout ≤ 16 MiB; JSON ≤ 8 MiB; env `ORCA_CODEINTEL_*`) | Test bất biến đồng thời (task 04), kịch bản tải (task 06) |
| Agent contract §2.5 (agent luôn hết hạn trước Go; 25 s/55 s) | Ngân sách phía agent phải nhỏ hơn trần này |
| Agent contract §8 ("`JSON.stringify(result)` Part A == Part B trừ `perf`, `startedAt`") | `perf` phải tách khỏi mọi phép so sánh vàng (task 03) |
| Proto/data-map: PQ-13 (timeout nhiều tầng), PQ-14 (trần kích thước: phản hồi gateway 2 MiB...), O-15 (số hiệu năng chỉ là giả định, giữ làm cấu hình) | Ngân sách là dữ liệu cấu hình, không hằng số cứng (task 05) |
| Agent contract §10 (Windows chưa hỗ trợ; `node:sqlite` experimental) | `rssPeakKb` chỉ cho Linux/macOS; Windows bỏ trường (method trả `unsupported_platform` rồi) |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent/src/relay/agent-tool-registry.ts` (đầy đủ), `agent-config.ts`, `agent-rpc-dispatch.ts` (route + `extractTraceFields`), `agent-rpc-dispatch-misc.ts`, `agent/vitest.config.ts`, `agent/package.json`, `agent/build.mjs`, `deploy/agent/orca-agent.service`.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Khối `perf`, hàng đợi lệnh CLI, giới hạn đồng thời | **Không tồn tại** (không có `codeintel/` hay bất kỳ `perf` nào trong `agent/src/relay`) | Toàn bộ mô-đun là "(mới)". Hàng đợi/semaphore là của AG-CV-SOL-001 (hợp đồng §2.3 là nó định nghĩa); solution này **đo và kiểm tra** nó, không dựng bản thứ hai |
| Tool `gitnexus`/`codegraph` hiện có | `runToolCommand` chỉ thu `stdout`/`stderr` dạng chuỗi, **không giới hạn dung lượng**, không đo thời gian/RSS; `timeout` mặc định 60 000 ms; kết thúc bằng `child.kill('SIGTERM')` (không huỷ cây tiến trình) | Không có điểm móc sẵn cho `perf`; khối `perf` thuộc runner mới của `codeintel.*`. Tool cũ giữ nguyên |
| Đo RSS | Node không có API `wait4`/`rusage` của tiến trình con; chỉ có cách đọc `/proc/<pid>/status` (Linux) hoặc `ps -o rss= -p <pid>` (macOS) khi tiến trình **còn sống** (chưa kiểm chứng trên máy này; chưa chạy) | RSS đỉnh = đỉnh các lần lấy mẫu, không phải đỉnh tuyệt đối. Bắt buộc nói rõ trong tài liệu và `MANIFEST` |
| Chạy TS từ script | Agent không có runner TS; `build.mjs` dùng `esbuild` | Bench dùng cùng cách bundle tạm như `AG-CV-SOL-070` task 03 (`agent/scripts/esbuild-run-typescript-entry.mjs`) |
| Script `check-*` | Nằm ở `desktop/config/scripts/` (README v7 điểm 20; hợp đồng §10) và `config/scripts/` ở gốc chỉ còn 3 tệp; mẫu `check-terminal-perf-report-budgets.mjs` được CR-071 dẫn là có trong `package.json` gốc (script `test:e2e:terminal-perf:check-report` thấy ở `package.json` gốc dòng 90; tôi chưa mở tệp `.mjs`) | Script so ngân sách của agent đặt trong `agent/scripts/` (đóng gói cùng gói `orca-agent`), không thêm vào `desktop/config/scripts/` |
| CI | Không workflow nào chạy test `agent/` (xem AG-CV-SOL-070 mục 1) | Test hợp đồng đồng thời chạy trong job của AG-CV-SOL-070 (cấu hình vitest `code-intel-contract`); benchmark nặng là workflow đêm riêng (task 07) |
| `.env`/systemd | `orca-agent.service` không có `EnvironmentFile`; biến tải từ script khởi động (`deploy/agent/scripts/start-agent-direct.sh` đọc `../.env`; `start.sh` mà unit gọi **không có trong repo**, chưa kiểm chứng) | Giới hạn `ORCA_CODEINTEL_*` đọc từ `process.env` |

**Correction relative to CR-CV-071:**

| # | CR viết | Hợp đồng / thực tế | Xử lý |
|---|---|---|---|
| 1 | Chờ hàng đợi tối đa ≤ 5 s rồi `CODEINTEL_TIMEOUT` **hoặc `CODEINTEL_RATE_LIMITED`** | Hợp đồng §2.3: chờ ≤ **10 s**, hàng đợi ≤ 16, quá → `CODEINTEL_TIMEOUT data.reason="queue_wait"`. `CODEINTEL_RATE_LIMITED` không phải mã do agent sinh (§3.2, §3.3) | Theo hợp đồng: 10 s và `CODEINTEL_TIMEOUT queue_wait`. Bài thử tải đặt `ORCA_CODEINTEL_QUEUE_WAIT_MS` hạ xuống để chạy nhanh (tên biến mới, xem 6) |
| 2 | Ngân sách "≤ 8 MiB stdout đọc vào bộ nhớ agent" | Hợp đồng §2.3: stdout tối đa **16 MiB** một tiến trình (vượt: kill + `OUTPUT_TOO_LARGE`), JSON kết quả cuối ≤ 8 MiB | Hai ngân sách khác nhau, test cả hai |
| 3 | `gitnexus` ≤ 2 và `codegraph` ≤ 3 đồng thời, tổng không nói | Hợp đồng: **tổng 3** toàn agent; `gitnexus` ≤ 2/dev server; `codegraph` ≤ 3 | Bất biến kiểm: `gitnexus ≤ 2`, `tổng ≤ 3` (nên 2 gitnexus + 1 codegraph, hoặc 3 codegraph) |
| 4 | Một báo cáo `codeintel-bench-<commit>.json` với `views[{rpc, cold, warm, bytes, ...}]` (ở service) | Phía agent chỉ đo `codeintel.*` method, không có "view" | Báo cáo agent riêng `codeintel-agent-bench-<commit>.json` với `methods[]`; báo cáo `views[]` là của BE-CV-SOL-071. Cần hợp nhất ở khâu so ngân sách (mục 7, câu hỏi 2) |
| 5 | Script `agent/scripts/bench-codeintel.mjs` "gọi từng method qua handler thật" | Handler là TS | Vỏ `.mjs` bundle một entry TS (`codeintel-bench-runner.ts`) |
| 6 | Ngân sách bộ nhớ heap thêm ≤ 128 MiB "stdout ≤ 8 MiB × 2 đồng thời + parse" | Với stdout tối đa 16 MiB/tiến trình và 3 tiến trình, ước lượng đỉnh thô là 48 MiB cho bộ đệm + chuỗi; CR nói 8 MiB × 2 = 16 MiB | Giữ 128 MiB làm ngân sách (có dư), ghi giả định; **chưa đo** |

### 1.1 Lệch giữa CR và hợp đồng

Tổng hợp từ bảng trên: (1) 5 s `RATE_LIMITED` so với 10 s `TIMEOUT queue_wait`; (2) hai mức stdout; (3) ràng buộc tổng 3 tiến trình; (4) hình dạng báo cáo. Ngoài ra CR-071 đặt metrics Prometheus ở service/gateway/fleet: **không có việc ở agent** (D3 của CR: agent không phơi `/metrics`, không OTel). Hợp đồng đồng ý (không thêm `@opentelemetry/*`).

### 1.2 Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `BE-CV-SOL-071-metrics-tracing-and-budgets` | **Tiêu thụ** khối `perf` (histogram `orca_codeintel_agent_cli_seconds{tool,command}`, `agent_queue_wait_seconds`, `agent_rss_peak_bytes`, span con `codeintel.agent.cli` dựng hậu kỳ). Hợp đồng bảo đảm: `perf` luôn là object khi method đọc thành công; thiếu `perf` không lỗi ở backend (CR-071 §5 `collector_perf_test.go`); `command` ∈ tập đóng |
| `BE-CV-SOL-071-...` benchmark Go | `codeintel-budgets.json`: phần `agent` ở solution này (task 05), phần `views` ở BE (câu hỏi mở 2) |
| `AG-CV-SOL-001` (cùng khu vực) | sở hữu runner, semaphore, hàng đợi, cache 60 s, `buildCodeIntelResult`; solution này cắm `PerfRecorder` vào đó (task 03) |
| `AG-CV-SOL-070` | tái dùng `esbuild-run-typescript-entry.mjs` (070-03), `vitest.code-intel-contract.config.ts` và workflow (070-09) |
| `FE-CV-SOL-*` | không có việc |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/codeintel/
  perf-block.ts                         (mới) kiểu CodeIntelPerf, PerfRecorder, tập command đóng
  perf-block.test.ts                    (mới)
  child-process-rss-sampler.ts          (mới) lấy mẫu RSS (Linux /proc, macOS ps), đọc được tiêm
  child-process-rss-sampler.test.ts     (mới)
  perf-envelope-integration.test.ts     (mới) perf trong kết quả thật, vắng trong cache/vàng
  tool-slot-limits.load.test.ts         (mới) bất biến đồng thời với CLI giả
  bench-budget-comparison.ts            (mới) so báo cáo với ngân sách
  bench-budget-comparison.test.ts       (mới)
  codeintel-bench-runner.ts             (mới) chạy các kịch bản, ghi báo cáo JSON
  codeintel-bench-runner.smoke.test.ts  (mới) chạy trên CLI giả, ngưỡng rất rộng
agent/scripts/
  bench-codeintel.mjs                   (mới) vỏ: bundle entry TS rồi gọi
  check-codeintel-bench-budgets.mjs     (mới) vỏ gọi bench-budget-comparison
  codeintel-budgets.json                (mới) dữ liệu ngân sách phía agent
.github/workflows/code-intel-bench.yml  (mới) đêm + workflow_dispatch
```

### 2.2 Khối `perf` (task 01, 03)

```ts
// perf-block.ts (mới)
export const CODEINTEL_CLI_COMMANDS = [
  'cypher','context','impact','query','status','callers','callees','files','affected','detect-changes'
] as const   // trùng hợp đồng §2.2; thêm lệnh mới phải sửa cả hợp đồng
export type CodeIntelCliCommand = (typeof CODEINTEL_CLI_COMMANDS)[number]
export type CliCallPerf = {
  tool: 'gitnexus' | 'codegraph'
  command: CodeIntelCliCommand
  ms: number
  stdoutBytes: number
  rssPeakKb?: number            // vắng khi nền tảng không hỗ trợ
}
export type CodeIntelPerf = {
  totalMs: number; queueWaitMs: number; cliCalls: number
  cli: CliCallPerf[]; parseMs: number; truncated: boolean
}
export class PerfRecorder {
  constructor(now?: () => number)
  recordQueueWait(ms: number): void
  recordCli(call: CliCallPerf): void           // từ chối command ngoài tập → ném lỗi lập trình (không phải lỗi người dùng)
  recordParse(ms: number): void
  markTruncated(): void
  build(): CodeIntelPerf                        // totalMs = now() - start; số nguyên không âm
}
export function perfForCacheHit(totalMs: number, truncated: boolean): CodeIntelPerf
```

Quy tắc:
- `command` chỉ là hằng; không có `args`, tên symbol, đường dẫn trong `perf` (test khẳng định bằng quét chuỗi `JSON.stringify(perf)`).
- Cache hit của agent (cache 60 s): `cliCalls:0, cli:[], queueWaitMs:0, parseMs:0`; `totalMs` là thời gian thật của lần gọi này (đề xuất của solution; hợp đồng không nói; mục 7 câu hỏi 3).
- `perf` **không** tham gia khoá cache và **không** được lưu vào giá trị cache: giá trị cache là `data` + metadata nguồn; `perf` dựng mới mỗi lần trả (hợp đồng §2.2).
- Mọi số là số nguyên (làm tròn) để JSON ổn định; `ms` ≥ 0.
- Lỗi (`error`) **không** có `perf` (hợp đồng chỉ nói kết quả đọc thành công).

### 2.3 Lấy mẫu RSS (task 02)

```ts
export type RssReader = (pid: number) => Promise<number | null> // KiB; null = không đọc được
export function createRssSampler(opts: {
  pid: number; intervalMs?: number /* mặc định 100 */; read?: RssReader; platform?: NodeJS.Platform
}): { stop(): Promise<number | undefined> }  // trả đỉnh KiB, undefined nếu chưa lần nào đọc được
```
- Linux: đọc `/proc/<pid>/status` dòng `VmHWM:` (đỉnh) rồi `VmRSS:`; ưu tiên `VmHWM` vì là đỉnh của kernel (tính tới thời điểm đọc). Đọc lần cuối **ngay trước khi** `close` nếu có thể (tiến trình vẫn là zombie chưa được thu hồi tới khi sự kiện `exit` được xử lý; chưa kiểm chứng thứ tự).
- macOS: `execFile('ps', ['-o','rss=','-p', String(pid)])` (KiB); không dùng shell; số lần gọi giới hạn bởi `intervalMs` ≥ 200 trên macOS.
- Cây tiến trình (con cháu của `gitnexus`) **không** được gộp ở v7 (chưa kiểm chứng `gitnexus` có sinh tiến trình con nặng không; CR-071 nói "cây tiến trình" nhưng đo ở 1.2 là từng tiến trình `gitnexus`); ghi giới hạn này trong tài liệu và cột báo cáo là `rssPeakKb` của tiến trình trực tiếp.
- Nền tảng khác/Windows: trả `undefined` và không có trường.
- Lấy mẫu **không** được làm treo hoặc chặn quá `intervalMs`; lỗi đọc là nuốt (đếm), không ném.

### 2.4 Bất biến đồng thời và hàng đợi (task 04)

Dùng CLI giả (tệp thực thi tạm, `#!/usr/bin/env node`, ngủ `N` ms rồi in JSON; đặt đầu `toolPath`) và đồng hồ thật hoặc giả có kiểm soát:

| Test | Khẳng định |
|---|---|
| `never exceeds 3 concurrent tool processes across tools` | khởi 20 lời gọi hỗn hợp; đếm tiến trình sống bằng tệp khoá trong thư mục tạm của CLI giả; `max ≤ 3` |
| `never exceeds 2 concurrent gitnexus` | `max_gitnexus ≤ 2` |
| `codegraph allowed up to 3 when no gitnexus runs` | đạt đúng 3 |
| `queue holds at most 16 waiters then rejects` | lời gọi thứ 17 chờ → `CODEINTEL_TIMEOUT` với `data.reason="queue_wait"` hoặc từ chối ngay; **quy tắc "tràn hàng đợi" của hợp đồng chỉ nói "≤ 16 mục"**; test chốt hành vi thật theo AG-CV-SOL-001 (câu hỏi 4) |
| `waiter times out after queue wait budget` | với `ORCA_CODEINTEL_QUEUE_WAIT_MS` nhỏ; mã `CODEINTEL_TIMEOUT`, `data.reason="queue_wait"`, `elapsedMs` hợp lý; không spawn tiến trình nào cho lời gọi đó |
| `perf.queueWaitMs reflects waiting` | lời gọi bị chờ có `queueWaitMs > 0`, lời gọi đầu `= 0` (± dung sai) |
| `timeout kills the child tree` | (thuộc AG-CV-SOL-072, tham chiếu) |
| `singleflight merges identical concurrent calls` | 10 lời gọi `codeintel.overview` cùng tham số → CLI giả được gọi 1 lần (hợp đồng §2.3 singleflight) |

### 2.5 Ngân sách là dữ liệu (task 05)

`agent/scripts/codeintel-budgets.json` (**chưa có số đo**; giá trị lấy từ CR-071 2.1, hiệu chỉnh sau lần chạy đầu):

```jsonc
{ "version": 1, "provenance": "CR-CV-071 §2.1, single-run on 32-core host, not a distribution",
  "agent": {
    "payloadBytes": { "codeintel.overview": 524288, "codeintel.subgraph": 1048576, "codeintel.symbol": 204800,
                      "codeintel.impact": 524288, "codeintel.detectChanges": 524288 },
    "stdoutBytesMax": 16777216, "resultBytesMax": 8388608,
    "concurrency": { "gitnexusMax": 2, "codegraphMax": 3, "totalMax": 3, "queueMax": 16 },
    "rssPeakKbMax": { "gitnexus": 900000, "codegraph": 450000 },   // dựa trên 0,6-0,85 GB và 0,29-0,37 GB đo một lần + dư
    "agentHeapExtraMiBMax": 128,
    "cliMsP95Max": { "gitnexus": 12000, "codegraph": 5000 }        // đề xuất, chưa có cơ sở đo phân phối
  } }
```
`bench-budget-comparison.ts` đọc báo cáo + ngân sách, trả danh sách vi phạm `{metric, observed, budget, method?}`; script `.mjs` thoát ≠ 0 khi có vi phạm; test bằng hai báo cáo mẫu (vượt/không vượt) và một báo cáo thiếu trường (thiếu = lỗi cấu hình, không phải đạt). Tên tệp ngân sách `codeintel-budgets.json` trùng CR-071; phần `views` (p95 theo RPC) để BE.

### 2.6 Benchmark (task 06, 07)

`codeintel-bench-runner.ts` (TS, gọi handler thật không qua mạng) + vỏ `bench-codeintel.mjs`:

| Kịch bản | Nội dung | Ra |
|---|---|---|
| lạnh/ấm theo method | mỗi method `codeintel.*` 30 lần lạnh (xoá cache của agent; xoá cache OS không bắt buộc, ghi nhận) và 100 lần ấm | p50/p95/p99 `totalMs`, kích thước kết quả (`Buffer.byteLength(JSON.stringify(result))`), tỉ lệ `truncated`, `rssPeakKb` lớn nhất |
| 10 phiên cùng worktree | gọi đồng thời `impact` + `detectChanges` | số lần CLI thật (singleflight phải gộp) |
| 10 worktree khác nhau | hàng đợi phải giữ `gitnexus ≤ 2` | `max` đồng thời quan sát, số `CODEINTEL_TIMEOUT queue_wait`, RSS cây (gộp các tiến trình trực tiếp tại cùng thời điểm) |
| parse | thời gian parse tệp vàng của AG-CV-SOL-070 (ngưỡng rộng, bắt hồi quy khổng lồ) | `parseMs` |

Báo cáo `codeintel-agent-bench-<commit>.json`: `{commit, tools:{gitnexus,codegraph versions}, host:{cores, ramGiB, platform}, samplingNote, methods:[{method, cold:{p50,p95,p99,n}, warm:{...}, bytes:{p50,p95,max}, truncatedRate, rssPeakKbMax}], concurrency:{gitnexusMax, totalMax, queueWaitTimeouts}}`; **không** vào git (artifact). Chạy trên máy chuyên dụng có chỉ mục Orca dựng sẵn (CR-071 §2.2); `analyze` chỉ do người vận hành chạy ở đó.

Workflow `.github/workflows/code-intel-bench.yml`: `schedule` hằng đêm + `workflow_dispatch`, `runs-on: [self-hosted, ...]` (nhãn do người vận hành chốt; **chưa có runner chuyên dụng**, câu hỏi mở 1), `continue-on-error`, tải artifact. Trên PR chỉ chạy `codeintel-bench-runner.smoke.test.ts` (CLI giả, ngưỡng rộng) trong job của AG-CV-SOL-070.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Agent chỉ **đo** (`perf`), không phơi metrics, không OTel | CR-071 D3; hợp đồng §2.2 |
| D2 | `PerfRecorder` là đối tượng nhỏ cắm vào runner của 001, không tự spawn | Không dựng hai hàng đợi |
| D3 | Chờ hàng đợi theo hợp đồng (10 s, `CODEINTEL_TIMEOUT queue_wait`) | Hợp đồng thắng CR |
| D4 | `rssPeakKb` là đỉnh các lần lấy mẫu của tiến trình trực tiếp; vắng khi không đọc được | Node không có `wait4`; trung thực về độ chính xác |
| D5 | Ngân sách là dữ liệu JSON, thiếu số liệu = lỗi, không phải đạt | CR-071 D1; tránh "xanh giả" |
| D6 | Benchmark nặng không chặn PR; smoke trên CLI giả chặn PR với ngưỡng rộng | CR-071 D2 |
| D7 | Không thêm phụ thuộc (không `tinybench`, `pidusage`) | O12 chưa duyệt; vitest bench không cần dùng |
| D8 | `perf` không nằm trong tệp vàng, không nằm trong khoá/giá trị cache | Hợp đồng §2.2, §8 |

## 4. Tiêu chí chấp nhận

- [x] Mọi kết quả thành công của method đọc có `perf` đúng kiểu; `command` luôn thuộc tập đóng; `JSON.stringify(perf)` không chứa đường dẫn, tên symbol, `args`.
- [x] `perf` không có trong giá trị cache, không có trong tệp vàng C2; cache hit trả `cliCalls:0`.
- [x] Bất biến: ≤ 2 `gitnexus`, ≤ 3 tiến trình tổng, hàng đợi ≤ 16, chờ quá hạn → `CODEINTEL_TIMEOUT` `data.reason="queue_wait"` (test với CLI giả, không phụ thuộc công cụ thật).
- [x] `rssPeakKb` có trên Linux (và macOS nếu đọc được), vắng trên nền tảng khác; không treo khi tiến trình thoát nhanh.
- [x] `codeintel-budgets.json` + script so sánh: báo cáo cố ý vượt → thoát ≠ 0; thiếu trường → thoát ≠ 0.
- [x] Bench chạy được qua `workflow_dispatch` trên máy có chỉ mục; báo cáo `codeintel-agent-bench-<commit>.json` có p50/p95/p99, kích thước, `rssPeakKb`, tỉ lệ cắt; lần đầu điền số đo vào PR (ngân sách hiệu chỉnh một lần).
- [x] Không thêm phụ thuộc, không `max-lines` disable.

## 5. Kiểm thử

Theo từng task. Lệnh: trong `/opt/repos/orca/agent`, `pnpm exec vitest run src/relay/codeintel/<tệp>`; bài thử tải thật: `node scripts/bench-codeintel.mjs --out <dir>` (chưa chạy).

Chưa chạy: toàn bộ; mọi ngưỡng test (thời gian, RSS) là giả định, đặt rộng để không flaky trên CI chia sẻ (không dùng ngưỡng ms tuyệt đối trong test PR).

## 6. Rủi ro và điểm chưa kiểm chứng

- Biến `ORCA_CODEINTEL_QUEUE_WAIT_MS` (mới, để test hạ thời gian chờ) **không có trong hợp đồng §2.4**; nếu không được thêm, test phải tiêm cấu hình qua tham số khởi tạo của runner thay vì env.
- `rssPeakKb` không phải đỉnh tuyệt đối; tiến trình sống < `intervalMs` có thể không đo được.
- Đo RSS trên SSH/WSL/macOS chưa kiểm chứng; Windows bỏ.
- Ngân sách (mọi con số) là một lần đo; chưa có p95; chưa đo trên dev server nhỏ (4 đến 8 GiB RAM) hay qua SSH (thêm 50 đến 200 ms mỗi hop theo README v7 mục 6).
- Kịch bản "10 worktree khác nhau" cần 10 worktree có chỉ mục: chỉ có trên máy bench; không chạy ở CI thường.
- `vitest` chạy test song song trong nhiều worker: bài thử đồng thời dùng CLI giả riêng của từng test (thư mục tạm riêng), không đếm toàn cục.
- Heap thêm ≤ 128 MiB: cách đo (`process.memoryUsage().heapUsed` delta) nhạy với GC; chỉ ghi nhận, không chặn.

## 7. Câu hỏi mở và điểm hợp đồng thiếu hoặc mâu thuẫn

1. Có runner chuyên dụng để chạy benchmark đêm (CR-071 Q1)? Nếu không, chỉ còn đo cục bộ do người vận hành chạy.
2. Một tệp `codeintel-budgets.json` chung (agent + service) hay hai tệp (agent, BE)? Đề xuất hai; hợp nhất ở khâu báo cáo. Cần BE-CV-SOL-071 đồng ý.
3. **Hợp đồng thiếu:** `perf` khi cache hit của agent; khi `truncated`; khi lỗi. Solution đặt: cache hit `cliCalls:0`; lỗi không có `perf`. Đề nghị bổ sung vào §2.2.
4. **Hợp đồng thiếu:** hành vi khi hàng đợi **đầy** (> 16) — chờ hay từ chối ngay; mã nào (`CODEINTEL_TIMEOUT queue_wait` hay `CODEINTEL_REINDEX_IN_PROGRESS queue_full`, cái sau chỉ dành cho reindex).
5. **Hợp đồng thiếu:** tên biến env cho thời gian chờ hàng đợi (§2.3 chỉ nêu giá trị 10 s, mục 2.4 không có `ORCA_CODEINTEL_QUEUE_WAIT_MS`).
6. Có dùng phiên MCP stdio giữ chung (CR-071 Q3) để bỏ chi phí khởi động 1,6 s? Ngoài phạm vi v7 MVP; sẽ đổi hẳn ngân sách.
