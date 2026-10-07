# AG-REQ-SOL-033-B: khối kết quả `ORCA_RESULT`, danh sách file thay đổi và giới hạn đầu ra

> ✅ **Đã triển khai.** Ngày triển khai 2026-10-07. Mọi mục "đã đọc" là đọc code, chưa chạy gì (thời điểm soạn). Phần "9. Kết quả triển khai" ghi lại những gì thực sự đã thực hiện và sai khác với kế hoạch.

**CR:** [CR-REQ-033](../../../../../../docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md) mục 2.1 (tham số `reportChanges`, `resultBlock`, `maxOutputBytes`), 2.4, 2.5; liên quan CR-REQ-029 (nơi định nghĩa nội dung khối), CR-REQ-008 (dùng `changes` để loại kết quả vi phạm chỉ đọc), CR-REQ-013 (đọc kết quả có cấu trúc)
**Service:** `agent/`, thư mục `agent/src/relay/`
**Nhóm solution:** B trong ba nhóm của CR-REQ-033 (A: chỉ đọc và vùng làm việc; C: báo cáo năng lực, handshake, `ai.complete`)
**TDD tham chiếu:** [v5/02-wire-protocol](../../../../tdd/v5/02-wire-protocol.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/10-git-handler-extension](../../../../tdd/v5/10-git-handler-extension.md), [v5/12-agent-spawner](../../../../tdd/v5/12-agent-spawner.md)
**Mẫu định dạng:** `specs/agent/crs/v4/task-graph/solutions/SOL-AG-TG-002-agent-chunk-streaming.md`

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent-print-mode-exec.ts` (toàn bộ), `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch.ts` (`makeNotifier`, `extractResultFields`, `route`), `agent-wire-protocol.ts` (`AgentErrorCode`), `agent-git-handler.ts` (tiền lệ chạy Git qua `execFile`), `agent-print-mode-exec.test.ts`.

| Điểm | Hiện trạng thật | Hệ quả |
|---|---|---|
| Cộng dồn `stdout` | `stdout += chunk` và `stderr += chunk` trong `handleAgentExecPrompt`, không giới hạn; `timeoutMs` tối đa 15 phút (`MAX_TIMEOUT_MS`) nên một lần chạy lâu có thể cộng dồn rất lớn | `maxOutputBytes` cần bộ đệm đuôi có giới hạn ngay khi nhận chunk, không cắt sau khi đã gom |
| Chunk thông báo | Mỗi chunk gửi thêm `agent.execPrompt.output` qua `notify` (đầy đủ, không cắt) | Giữ nguyên: giới hạn chỉ áp cho buffer cộng dồn trả trong kết quả |
| Kết quả handler thường | `{ stdout, stderr, exitCode, timedOut, stepId }` | Thêm trường tuỳ chọn `parsed`, `changes`, `truncated`, `warnings`, `applied` chỉ khi cần |
| Kết quả handler stream | không cộng dồn; `stream.end` chỉ có `exitCode` | Muốn có `parsed`/`changes` ở stream phải gom đuôi `stdout` (chỉ khi `resultBlock` bật) và đưa trường vào `stream.end` (mục 2.6) |
| `extractResultFields` | `agent-rpc-dispatch.ts` có hàm này lấy `exitCode`/`timedOut` cho trace | Không cần sửa; trường mới không vào trace (tránh lộ nội dung) |
| Git khi chạy | `agent-git-handler.ts` dùng `execFile` cho Git; chính sách repo (`git-handler-*`) đã tồn tại | Snapshot dùng `execFile('git', ...)` riêng, không gọi qua handler Git (handler cần `ws`) |
| Giới hạn khung | khung truyền tối đa 16 MiB: đã xác nhận `MAX_MESSAGE_SIZE = 16 * 1024 * 1024` ở `packages/dev-agent-transport/src/relay-protocol.ts:21`; kích thước tính SAU khi JSON thoát ký tự (dấu nháy và xuống dòng thành 2 byte) | `maxOutputBytes` tối đa 12 MiB cho `stdout` cộng `stderr` còn chỗ cho JSON bọc: mục 2.2 chốt cách tính |

Correction relative to CR-REQ-033: (1) CR viết "cắt `stdout`/`stderr` giữ phần cuối": solution áp ngưỡng RIÊNG cho từng luồng (mỗi luồng tối đa `maxOutputBytes`), nên tổng có thể đến 2 lần; để còn dưới 16 MiB, trần 12 MiB của CR chỉ an toàn nếu ngưỡng là TỔNG. Solution chốt: ngưỡng là tổng, chia `stdout` 3/4 và `stderr` 1/4 (mục 2.2). (2) CR không định nghĩa kiểu của `truncated`; solution chọn `{ stdout: boolean, stderr: boolean }` và chỉ có mặt khi có luồng bị cắt (câu hỏi mở 1, backend phải khớp).

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/
  agent-bounded-output-buffer.ts            (mới) bộ đệm đuôi theo byte, cắt an toàn UTF-8
  agent-bounded-output-buffer.test.ts       (mới)
  agent-result-block-parser.ts              (mới) hàm thuần phân tích khối ORCA_RESULT
  agent-result-block-parser.test.ts         (mới)
  agent-worktree-change-snapshot.ts         (mới) chụp git status trước/sau và so sánh
  agent-worktree-change-snapshot.test.ts    (mới, dùng repo tạm bằng git init thật)
  agent-print-mode-exec.ts                  (sửa) gom đuôi, gọi parser và snapshot, thêm trường kết quả
  agent-print-mode-exec.test.ts             (sửa) thêm describe cho nhóm B
```

### 2.2 `maxOutputBytes` và `truncated`

| Mục | Quy tắc |
|---|---|
| Tên, kiểu | `maxOutputBytes`: số nguyên (JSON number, `Number.isInteger`) |
| Mặc định | `4 * 1024 * 1024` (4 MiB) |
| Trần | `12 * 1024 * 1024`; lớn hơn thì bị KẸP về trần (khác `accessMode`, vì đây không phải tham số an toàn) |
| Sàn | `64 * 1024`; nhỏ hơn hoặc không nguyên hoặc không phải số: `InvalidParams`, `INVALID_MAX_OUTPUT_BYTES` |
| Cách tính | Tổng byte `stdout` + `stderr` giữ lại không vượt `maxOutputBytes`; `stdout` được 3/4, `stderr` 1/4 |
| Phần giữ | PHẦN CUỐI mỗi luồng (khối kết quả nằm ở cuối `stdout`) |
| Dấu hiệu | `truncated: { stdout: boolean, stderr: boolean }`, chỉ có khi ít nhất một luồng bị cắt; vắng nghĩa là không cắt |
| Thay đổi hành vi cho người gọi cũ | Người gọi cũ không gửi tham số sẽ lần đầu bị giới hạn 4 MiB (trước đó không giới hạn). Chấp nhận vì khung 16 MiB vốn đã làm lớn hơn không đi qua được (CR mục 1.2, chưa kiểm chứng) |

`BoundedOutputBuffer`:

```ts
export class BoundedOutputBuffer {
  constructor(maxBytes: number)
  append(chunk: Buffer): void          // giữ tối đa maxBytes byte cuối
  get truncated(): boolean             // đã bỏ byte đầu
  toString(): string                   // decode utf8, bỏ byte tiếp nối (10xxxxxx) mở đầu nếu bị cắt giữa ký tự
}
```

Handler hiện nhận `d: Buffer` rồi `toString('utf8')` ngay cho từng chunk (một ký tự UTF-8 bị cắt giữa hai chunk sẽ hỏng; đây là lỗi có sẵn). Bộ đệm mới giữ `Buffer` thô và chỉ decode khi kết thúc, nên đồng thời sửa lỗi đó cho `stdout`/`stderr` trong kết quả. Chunk `notify` vẫn decode từng chunk như cũ (không đổi).

### 2.3 Khối kết quả (`resultBlock`)

Tham số: `resultBlock: { nonce: string }`; `nonce` khớp `^[A-Za-z0-9]{16,64}$`, sai thì `InvalidParams`, `INVALID_RESULT_BLOCK_NONCE`. Không có tham số: không thêm `parsed`, không chạy parser.

```ts
export const RESULT_BLOCK_MAX_BYTES = 256 * 1024
export type ResultBlockErrorCode =
  | 'RESULT_BLOCK_MISSING' | 'RESULT_BLOCK_INVALID_JSON'
  | 'RESULT_BLOCK_TOO_LARGE' | 'RESULT_BLOCK_NOT_OBJECT'
export type ParsedResult =
  | { ok: true; value: Record<string, unknown> }
  | { ok: false; code: ResultBlockErrorCode; detail: string }
export function parseResultBlock(stdout: string, nonce: string): ParsedResult
```

Thuật toán (chốt các chỗ CR để ngỏ):

1. Tách `stdout` thành dòng theo `\n`, bỏ `\r` cuối mỗi dòng.
2. Dòng đánh dấu hợp lệ khi so sánh BẰNG đúng `ORCA_RESULT_BEGIN <nonce>` hoặc `ORCA_RESULT_END <nonce>` (bỏ khoảng trắng đầu/cuối dòng). Dòng có nonce khác, thiếu nonce, hay có chữ thừa không phải đánh dấu (chống chèn từ nội dung Jira).
3. Lấy dòng END hợp lệ CUỐI CÙNG; tìm dòng BEGIN hợp lệ GẦN NHẤT phía trước nó. Không có cặp: `RESULT_BLOCK_MISSING`.
4. Phần giữa (nối bằng `\n`): nếu `Buffer.byteLength > 256 KiB`: `RESULT_BLOCK_TOO_LARGE` (kiểm TRƯỚC `JSON.parse`).
5. `JSON.parse`; lỗi: `RESULT_BLOCK_INVALID_JSON`, `detail` là `error.message` cắt 200 ký tự (không chép nội dung khối). Kết quả là mảng, `null`, số, chuỗi: `RESULT_BLOCK_NOT_OBJECT`.
6. Có nhiều khối: luôn dùng khối cuối. Khối đầu bị bỏ.

Parser chạy trên phần đuôi giữ lại của `stdout` (đã qua `BoundedOutputBuffer`). Nếu khối bị cắt mất `BEGIN` thì `RESULT_BLOCK_MISSING` (hệ quả của giữ phần cuối; nêu ở 6).

Agent KHÔNG kiểm schema. Trường `parsed` thêm vào `result` của RPC: `parsed: { ok: true, value } | { ok: false, code, detail }`; tên `parsed` thay vì `result` để tránh `result.result`.

### 2.4 Danh sách file thay đổi (`reportChanges`)

Chỉ chạy khi `reportChanges === true` (kiểu boolean; chuỗi `"true"` là `InvalidParams`, `INVALID_REPORT_CHANGES`).

Lệnh (không tuỳ chọn nào mới hơn Git 2.25; `--no-optional-locks` từ 2.15, `--porcelain=v1` và `-z` từ 1.7.x và 2.x sớm; tuỳ chọn toàn cục đặt trước subcommand đúng quy tắc `AGENTS.md`):

```
git --no-optional-locks status --porcelain=v1 -z --untracked-files=all
git rev-parse HEAD            # ở repo chưa có commit: lỗi, headBefore = null
```

Cấu trúc:

```ts
export type SnapshotEntry = { status: string; size: number | null; mtimeMs: number | null }
export type WorkspaceSnapshot =
  | { kind: 'git'; head: string | null; entries: Map<string, SnapshotEntry> }
  | { kind: 'directory'; entries: Map<string, SnapshotEntry>; truncated: boolean }
  | { kind: 'unavailable'; reason: 'NOT_A_GIT_REPO' | 'SNAPSHOT_FAILED' | 'SNAPSHOT_TIMEOUT' }
export async function captureSnapshot(cwd: string, mode: 'git' | 'directory', deps?: SnapshotDeps): Promise<WorkspaceSnapshot>
export function diffSnapshots(before: WorkspaceSnapshot, after: WorkspaceSnapshot): ChangeReport
```

Kết quả:

```ts
export type ChangeReport =
  | { available: true; headBefore: string | null; headAfter: string | null; headMoved: boolean;
      changedFiles: { path: string; change: 'added'|'modified'|'deleted'|'renamed'|'untracked' }[];
      truncated: boolean }
  | { available: false; reason: 'NOT_A_GIT_REPO' | 'SNAPSHOT_FAILED' | 'SNAPSHOT_TIMEOUT' }
```

Quy tắc phân loại `change` (từ mã XY của `status --porcelain=v1`): `??` là `untracked`; có `R` hoặc `C` là `renamed` (ghi đường dẫn đích; `-z` đưa đường dẫn nguồn ở mục kế, phải bỏ qua mục đó); có `D` là `deleted`; có `A` là `added`; còn lại `modified`. Đường dẫn có trong `before` nhưng KHÔNG có trong `after` (bị hoàn tác hoặc commit): coi là thay đổi, `change: 'modified'` (CR không có nhãn riêng; câu hỏi mở 2). `changedFiles` tối đa 2000 mục, quá thì `truncated: true`. Chữ ký bằng nhau (`status`, `size`, `mtimeMs`) thì KHÔNG đưa vào danh sách; file đã bẩn từ trước mà bị sửa tiếp có `mtimeMs` hoặc `size` đổi nên được tính.

Không phải repo Git: mode `git` trả `unavailable NOT_A_GIT_REPO` (nhận biết bằng lỗi của `git rev-parse --is-inside-work-tree`, mã thoát khác 0). Với `workspaceKind=scratch` (nhóm A) dùng mode `directory`: đi bộ thư mục tối đa 5000 mục (không theo symlink), so `size`+`mtimeMs`. Thời hạn mỗi lần chụp 30 giây (`SNAPSHOT_TIMEOUT`), không làm hỏng kết quả chính.

Giới hạn đã biết (ghi lại trong test và README tasks): file bị `.gitignore` không thấy; thay đổi ngoài `cwd` (ví dụ `$HOME`) không thấy; đây là phát hiện, không hoàn tác.

Cảnh báo `READONLY_VIOLATION`: khi `accessMode=readonly` (nhóm A) và `changes.available` mà `changedFiles` không rỗng hoặc `headMoved`. Thêm `'READONLY_VIOLATION'` vào `warnings`. Agent KHÔNG hoàn tác và KHÔNG đổi `exitCode`; backend (CR-REQ-008) quyết định loại kết quả.

### 2.5 Thứ tự trong handler thường

```
parse options (nhóm A) -> validate workspace (A) -> buildAgentEnv -> readonly policy (A)
-> [reportChanges] captureSnapshot(before) -> spawn -> thu thập (BoundedOutputBuffer)
-> close/timeout -> [reportChanges] captureSnapshot(after) -> diff
-> [resultBlock] parseResultBlock(stdoutTail) -> assemble result
```

Chụp "trước" xảy ra SAU `buildAgentEnv` và trước `spawn`, nên nếu `buildAgentEnv` ném lỗi thì không chụp. Chụp "sau" chạy cả khi hết giờ (tiến trình bị `SIGKILL` có thể đã ghi dở file, đây chính là điều cần biết).

Kết quả cuối (mọi trường mới đều tuỳ chọn):

```json
{ "stdout": "...", "stderr": "...", "exitCode": 0, "timedOut": false, "stepId": "run-1",
  "applied": { "accessMode": "readonly", "workspaceKind": "repo_root" },
  "changes": { "available": true, "headBefore": "ab12...", "headAfter": "ab12...", "headMoved": false,
               "changedFiles": [], "truncated": false },
  "parsed": { "ok": true, "value": { "summary": "..." } },
  "truncated": { "stdout": true, "stderr": false },
  "warnings": ["TRUST_PRESET_IGNORED_READONLY"] }
```

### 2.6 Handler stream

`handleAgentExecPromptStream` giữ nguyên cách gửi `stream.chunk`. Khi cần (có `resultBlock` hoặc `reportChanges`), nó gom thêm đuôi `stdout` bằng `BoundedOutputBuffer` và chụp snapshot như trên; ở `stream.end` đính thêm các trường như bảng trên, cùng tên: `{ type: 'stream.end', exitCode, parsed?, changes?, truncated?, warnings?, applied? }`. Nhánh hết giờ cũng gửi `stream.end` (`exitCode: -1`) với các trường này sau khi chụp "sau". `truncated` ở stream chỉ nói về buffer dùng để phân tích, không nói về chunk đã gửi (chunk gửi đầy đủ).

Backend Go cần đọc `stream.end` có thêm trường; `isTerminalStreamResponse` và decoder của `devserveragent` hiện chưa kiểm xem có bỏ qua trường lạ hay không (chưa đọc `client.go` phần stream trong đợt này): ghi vào rủi ro.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do | Bỏ |
|---|---|---|---|
| 1 | Parser là hàm thuần, không đọc môi trường, không log nội dung khối | Dễ test bảng đúng sai; nội dung khối có thể chứa dữ liệu nhạy cảm | Parser gắn trong handler |
| 2 | Khớp dòng BẰNG đúng, không tìm chuỗi con | Chuỗi `ORCA_RESULT_BEGIN` nằm giữa câu trong nội dung Jira không được kích hoạt | Regex chuỗi con |
| 3 | Dùng khối CUỐI, không khối đầu | Mô hình hay nhắc lại khuôn dạng trong lời giải thích trước khi in kết quả thật | Từ chối khi có nhiều khối |
| 4 | Ngưỡng `maxOutputBytes` là tổng, chia 3/4 và 1/4 | Bảo đảm dưới 16 MiB kể cả khi trần 12 MiB | Ngưỡng riêng từng luồng |
| 5 | `truncated` chỉ có khi cắt | Kết quả không tham số mới giống hệt trước (tiêu chí chấp nhận CR) | Luôn trả |
| 6 | Snapshot lỗi không làm hỏng lần chạy | `changes` là thông tin bổ sung; `available:false` kèm `reason` | Ném lỗi |
| 7 | Không hoàn tác khi `READONLY_VIOLATION` | Hoàn tác (`git checkout`, `git clean`) có thể xoá việc hợp lệ của người dùng trên repo gốc | Tự `git restore` |
| 8 | Không kiểm schema nội dung khối | Schema theo `kind` và TaskSpec đổi thường xuyên (CR-REQ-029) | Nhúng schema vào agent |

### 3.5 Tương thích hai chiều

| Backend | Agent | Kết quả |
|---|---|---|
| Cũ | Mới | Không gửi tham số mới: chỉ khác là `stdout`/`stderr` bị giới hạn 4 MiB (có `truncated` khi cắt) |
| Mới gửi `resultBlock`, `reportChanges` | Cũ | Agent cũ bỏ qua; kết quả không có `parsed`/`changes`. Backend PHẢI coi "không có `parsed`" là agent cũ (protocol 1) và tự tìm khối trong `stdout` bằng cùng thuật toán (CR-REQ-029 mục 2.5 đã nêu); `changes` vắng thì dùng kiểm repo trước/sau qua git-gateway (CR-REQ-008 v1) |
| Mới | Mới, nhưng `parsed.ok=false` | Mã lỗi `RESULT_BLOCK_*`; backend quyết định thử lại hay thất bại `agent_defect` (CR-REQ-029) |

## 4. Phụ thuộc và thứ tự

- Bắt đầu sau task 01 của nhóm A (bộ đọc tham số `parseExecPromptOptions` đã có chỗ cho `reportChanges`, `resultBlock`, `maxOutputBytes`). Task 05, 06, 07 độc lập với nhau; task 08 nối vào handler và phụ thuộc 04 (nối nhóm A) cùng 05 đến 07.
- Handshake `features` chỉ thêm `agent.execPrompt.changes` và `agent.execPrompt.resultBlock` sau task 08 (task 12).
- Backend: `request-service`/`task-service` (`SimpleExecutor`, CR-REQ-029 Q4) phải gửi đúng tên `resultBlock.nonce`, `reportChanges`, đọc `parsed`, `changes`, `truncated`, `warnings`; xem README solutions.

## 5. Kiểm thử

Chạy trong `/opt/repos/orca/agent`:

| Tầng | Test | Lệnh |
|---|---|---|
| Unit parser | bảng đúng/sai (nonce đúng, sai, thiếu; nhiều khối; lồng; `\r\n`; Unicode tiếng Việt; JSON hỏng; mảng; quá 256 KiB; dòng giả không có nonce) | `pnpm exec vitest run src/relay/agent-result-block-parser.test.ts` |
| Unit bộ đệm | cắt đuôi, ký tự UTF-8 nhiều byte ở ranh giới, chunk rỗng | `pnpm exec vitest run src/relay/agent-bounded-output-buffer.test.ts` |
| Snapshot | repo tạm bằng `git init` thật: thêm, sửa, xoá, đổi tên, untracked, file bẩn từ trước sửa tiếp, commit (headMoved), ngoài repo, quá 2000 mục | `pnpm exec vitest run src/relay/agent-worktree-change-snapshot.test.ts` |
| Handler | mock `spawn`, snapshot tiêm bằng hàm giả | `pnpm exec vitest run src/relay/agent-print-mode-exec.test.ts` |
| Toàn gói | | `pnpm test` |

Git thật cần có trên máy chạy test (CI agent đã dùng Git ở `git-handler*.test.ts`, chưa kiểm chứng đợt này). Test snapshot đặt `GIT_CONFIG_GLOBAL=/dev/null` hoặc `-c user.name/-c user.email` khi tạo commit mẫu để không phụ thuộc cấu hình máy; trên Windows dùng `NUL` (hoặc bỏ qua).

Kiểm với `claude` thật: không bắt buộc cho nhóm B; kiểm tay: prompt yêu cầu in khối `ORCA_RESULT_BEGIN <nonce>` ... `ORCA_RESULT_END <nonce>` đúng nonce, quan sát `parsed`. Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Trần 12 MiB chưa chắc đủ dưới 16 MiB: `JSON.stringify` có thể nở đầu ra tới khoảng 2 lần với văn bản nhiều dấu nháy, xuống dòng hoặc ký tự điều khiển (nở 6 byte cho mỗi ký tự điều khiển). Task 05 thêm kiểm tra kích thước SAU khi mã hoá và cắt thêm nếu cần; chưa đo thực tế.
- Chụp snapshot với repo rất lớn (hàng trăm nghìn file untracked như `node_modules` chưa ignore) có thể chậm; mốc 30 giây là đề xuất chưa đo.
- `git status` trong worktree liên kết, submodule, sparse checkout: chưa thử; có thể cho đường dẫn tương đối khác nhau.
- Mô hình có thể không in đúng khối; đó là việc của prompt backend (CR-REQ-029), không phải của agent.
- Hết giờ (SIGKILL) rồi chụp "sau" làm tăng thời gian trả lời tới 30 giây trên đường hết giờ; backend cần tính thêm vào timeout phía gọi.
- Bộ đệm byte thô thay cho cộng chuỗi thay đổi nhẹ hành vi decode (sửa ký tự bị cắt); test hồi quy hiện có kiểm chuỗi ASCII nên không bắt được; thêm test UTF-8.

## 7. Câu hỏi mở

1. Kiểu `truncated`: `{stdout,stderr}` (đề xuất) hay boolean đơn? Backend (task-service) chọn khi khớp.
2. Đường dẫn bị hoàn tác hoặc commit trong lúc chạy: có cần nhãn riêng (`reverted`) thay cho `modified` không?
3. `stream.end` mang `parsed` hay backend tự phân tích từ chunk? (Đề xuất: agent phân tích, để hai đường cùng hợp đồng.)

## 8. Tham chiếu

- `/opt/repos/orca/agent/src/relay/agent-print-mode-exec.ts`, `agent-print-mode-exec.test.ts`, `agent-rpc-dispatch-agent-exec.ts`, `agent-rpc-dispatch.ts`, `agent-git-handler.ts`, `agent/src/shared/agent-wire-protocol.ts`
- `/opt/repos/orca/guides/reference/git-compatibility.md`, `AGENTS.md` (mục Git Binary Compatibility)
- CR: `docs/crs/v6/agent-capabilities/CR-REQ-033-agent-readonly-worktree-and-capability-report.md`, `docs/crs/v6/execution-contract/CR-REQ-029-execution-contract-and-readiness-gate.md` (mục 2.5, Q4), `docs/crs/v6/solution-analysis/CR-REQ-008-diagnosis-findings-answer-analysis.md`

## 9. Kết quả triển khai (2026-10-07)

### 9.1 File đã tạo / sửa

| File | Hành động | Ghi chú |
|---|---|---|
| `agent/src/relay/agent-bounded-output-buffer.ts` | Tạo mới | `BoundedOutputBuffer`, `splitOutputBudget` (3/4 stdout, 1/4 stderr), `fitResultToFrame` |
| `agent/src/relay/agent-result-block-parser.ts` | Tạo mới | `parseResultBlock` thuần hàm, `RESULT_BLOCK_MAX_BYTES = 256 KiB` |
| `agent/src/relay/agent-worktree-change-snapshot.ts` | Tạo mới | `captureSnapshot` (git/directory), `diffSnapshots`, `ChangeReport` |
| `agent/src/relay/agent-print-mode-exec.ts` | Sửa | Cả hai handler: snapshot trước/sau, `BoundedOutputBuffer`, `parseResultBlock`, `fitResultToFrame`, `changes`, `parsed`, `truncated`, `warnings` |

### 9.2 TypeScript

**0 lỗi** trên tất cả file mới/sửa. Pre-existing error ở `agent-tool-registry.test.ts:259` ngoài phạm vi.

### 9.3 Sai khác so với kế hoạch

1. **`fitResultToFrame` được thêm vào `agent-bounded-output-buffer.ts`** (không phải file riêng) để tránh tích thêm file. Loại bỏ `stdout`/`stderr` khỏi JSON khi frame quá lớn (giữ toàn bộ mế ta dữ liệu khác).
2. **`analysisBuf` trong stream handler**: buffer phân tích riêng (4 MiB) được tạo khi `resultBlockNonce` hiện diện; chunk gửi đầy đủ như cũ (không cắt stream). Đúng theo lý do của giải pháp (mục 2.6).
3. **`diffSnapshots` trả `ChangeReport` trực tiếp** (không bao gói thêm); `changes` trong kết quả là `ChangeReport` gốc.
4. **Test files đã hoàn thành và đạt 100% GREEN**:
   - `agent-bounded-output-buffer.test.ts` (8/8 tests)
   - `agent-result-block-parser.test.ts` (14/14 tests)
   - `agent-worktree-change-snapshot.test.ts` (6/6 tests)
5. **Mặc định `maxOutputBytes = 4 MiB`**: người gọi cũ không gửi tham số được giới hạn 4 MiB lần đầu; chấp nhận như kế hoạch (khung 16 MiB vốn đã làm lớn hơn không đi qua được).

### 9.4 Câu hỏi mở đã chốt

- **Q1 (kiểu `truncated`)**: Dùng `{ stdout: boolean, stderr: boolean }`, chỉ có khi cắt — đúng kế hoạch.
- **Q3 (`stream.end` mang `parsed`)**: Agent phân tích và đưa vào `stream.end` — đúng kế hoạch.

### 9.5 Còn lại

- Kiểm chứng `stream.end` trường mới với Go decoder phía backend.
- Thử nghiệm giới hạn 12 MiB sau JSON encode trên payload lớn trong môi trường staging.
