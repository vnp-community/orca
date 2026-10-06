# AG-CV-SOL-006: `codeintel.*` trên Part B (`RelayDispatcher`) và sửa `error.data` ở hai cây

> 📋 Proposed, chưa triển khai. **P2, chỉ làm khi O-5 được chấp thuận** (task 02–06); task 07 (kiểm thử đường stdio/detach của Part A) làm độc lập. Ngày soạn 2026-10-06; chưa chạy gì.

**CR:** [CR-CV-006](../../../../../../docs/crs/v7/agent-codeintel/CR-CV-006-codeintel-relay-ssh-part-b.md)
**Service:** `agent/src/relay/` **và** `desktop/src/relay/` (bản `relay.js` thật cho Part B build từ `desktop/`)
**TDD tham chiếu:** [v5/03-connection-modes](../../../../tdd/v5/03-connection-modes.md) mục 3; [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md) mục 1, 5, 9.1; [v5/08-deployment](../../../../tdd/v5/08-deployment.md) mục 1–2; API: `specs/agent/api/gaps-and-findings.md` mục 4, `specs/agent/api/connection-modes.md` mục 0, 6
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md`

## 0. Hợp đồng áp dụng
| Nguồn | Mục |
|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §1.1 (relay-ssh do Go = Part A qua `--stdio`), §3.4 cuối (Part B làm rơi `err.data`; sửa ≤ 5 dòng ở **cả** hai cây; phương án `message` không khuyến nghị), §8 (Part B: cùng `CODEINTEL_METHODS`, `registerCodeIntelHandlers`, khác biệt wire, kiểm thử bắt buộc, không `quality.*`, không thêm vào `relay.status`), §10 |
| `CONTRACT-codeintel-proto-and-data-map.md` | §9 O-5; §7.2/7.3 (006 sau 001–005, đợt 6); §8.2 (gồm sửa `dispatcher.ts` ở hai cây) |

## 1. Trạng thái hiện tại (re-verify)
Đã đọc (2026-10-06): CR-CV-006; `agent/src/relay/dispatcher.ts:454-495` (`handleRequest`; catch `:485-491` chỉ gửi `{code,message}`; `rpc.cancel` `:497-503`); `desktop/src/relay/dispatcher.ts:455-495` (cùng logic, catch `:462-464`); cả hai đầu file có `/* eslint-disable max-lines */` **có sẵn**; `agent/src/relay/relay.ts:468-496` (`loadAgentConfig()` + logger `relayLogLine` + `registerAuthStatusHandlers` `:493`); `desktop/src/relay/relay.ts` (**không** có `registerAuthStatusHandlers`; có `dispatcher.onRequest('session.registerRoot'…)` `:447`, `session.resolveHome` `:459`, `orca.cli` `:494`, `relay.status` `:685`; `relayLogLine` import `:65`); `agent/src/relay/relay-auth-status-handlers.ts` (mẫu); `agent-entry.ts:80-108`; `agent-connection-stdio.ts:262-280`; `desktop/config/scripts/build-relay.mjs:19` (`RELAY_ENTRY = src/relay/relay.ts`); `desktop/src/relay/` có `agent-tool-registry.ts` (giống bản `agent/` ở 400 dòng đầu), `agent-config.ts` (`loadAgentConfig()` không tham số), `agent-logger.ts`, `git-handler-check-ignore.ts`, `git-handler-ops.ts`, `desktop/src/shared/git-capability-cache.ts`, `git-worktree-command-capabilities.ts`; không có `agent-git-handler-extended.ts`; `agent/config/` chỉ có `oxlint-react-doctor.json`, scripts `check-*` ở `desktop/config/scripts/`.

### Correction relative to CR
| # | CR nói | Thực tế / quyết định |
|---|---|---|
| 1 | gọi `registerCodeIntelHandlers` "ngay sau `registerAuthStatusHandlers`" ở cả hai cây | chỉ cây `agent/` có dòng đó; cây `desktop/` phải tự tạo `loadAgentConfig()` + logger `relayLogLine` và chọn điểm cắm (đề xuất ngay trước `dispatcher.onRequest('orca.cli'…)`, `desktop/src/relay/relay.ts:494`, chưa kiểm cấu trúc `main()` đầy đủ) |
| 2 | script parity ở `config/scripts/` | đặt ở `desktop/config/scripts/check-codeintel-relay-parity.mjs` (nơi các `check-*` đang ở) và nối vào `pnpm lint`/CI là việc sau khi chủ chốt (Q3) |
| 3 | sao `codeintel-*.ts` sang `desktop/` | phải sao thêm bản vá `runToolCommand` (AG-CV-TASK-001-04) vào `desktop/src/relay/agent-tool-registry.ts` và các phụ thuộc `shared/` |
| 4 | `git()` từ `agent-git-handler-extended.ts` | không có ở `desktop/`; `codeintel-git-exec.ts` (SOL-001) tự chứa nên sao được |

### Lệch giữa CR và hợp đồng
Không lệch ngữ nghĩa. Hợp đồng §8 chỉ nói "đăng ký sau `registerAuthStatusHandlers` `relay.ts:494`" (đúng cho `agent/`); đã bổ sung điểm cắm của `desktop/`. `perf`/`startedAt` được loại khỏi so sánh parity (thay `toolTimingsMs` của CR).

### Phụ thuộc chéo khu vực
BE: không đổi; Go phát hiện hỗ trợ bằng `codeintel.status` và `-32601` (`client.go:435-437`). AG: `AG-CV-SOL-001…005` (tất cả method), `AG-CV-SOL-070` (fixture/parity), `AG-CV-SOL-072-security-tests-agent`. FE: không (Electron là người dùng gián tiếp, O-5). `quality.*` không có ở Part B.

## 2. Giải pháp
### 2.1 Lõi trung lập và bộ chuyển
Quy tắc đã đặt ở SOL-001: `codeintel-*.ts` không import `ws`, `WireState`, `makeNotifier`, `makeError`, `dispatcher`. Task 01 thêm test quét import để giữ. Bộ chuyển Part B:
```ts
// agent/src/relay/codeintel-relay-handlers.ts (mới)
export function registerCodeIntelHandlers(dispatcher: RelayDispatcher, config: AgentConfig, log: AgentLogger): void
export function toRelayThrowable(err: unknown): Error & { code: number; data?: Record<string, unknown> }
```
Mỗi `CODEINTEL_METHODS[m]` -> `dispatcher.onRequest(m, (params, ctx) => runCodeIntelMethod(def, params, {config, log, signal: ctx.signal, notifier}))`; `setCodeIntelNotifier((method, params) => dispatcher.notify(method, params))`; lỗi qua `toErrorPayload` -> `Object.assign(new Error(message), {code, data})`. `rpc.cancel` -> `ctx.signal` -> `runCodeIntelTool({signal})` kill CLI. Part B không có `capabilities`: backend gọi `codeintel.status`, `-32601` = không hỗ trợ; **không** thêm vào `relay.status`.
### 2.2 Sửa `dispatcher.ts` (≤ 5 dòng, mỗi cây)
Trong catch của `handleRequest`: `const data = (err as {data?: unknown}).data; this.sendResponse(client, req.id, undefined, { code, message, ...(data !== undefined && { data }) })`. Không thêm `max-lines` disable (file đã có sẵn một cái; AGENTS.md cấm thêm). Test hồi quy: lỗi không `data` giữ nguyên khung.
### 2.3 Hai cây
Sao `codeintel-*.ts`, `gitnexus-*.ts`, `codegraph-*.ts`, `codeintel-symbol-ref*.ts`, `codeintel-relay-handlers.ts` và bản vá `runToolCommand` sang `desktop/src/relay/` trong **cùng một PR**; script parity so khớp nội dung (loại khác biệt import đã khai báo). Không sửa các khác biệt có sẵn giữa hai `relay.ts`/`dispatcher.ts`.
### 2.4 Đường Go (Part A qua `--stdio`/`--detach`)
Không cần mã; chỉ kiểm thử: `codeintel.status` qua `StdioWebSocketAdapter`; daemon `--detach` có nhiều phiên đồng thời (Map module sống qua kết nối; notifier chỉ tới phiên gọi gần nhất); trong `--stdio` trực tiếp `AgentLogger` ghi `console.log` ra stdout vốn là khung giao thức (`agent-logger.ts`): mã codeintel chỉ log qua logger được truyền, không `console.*`.

## 3. Quyết định thiết kế
| # | Quyết định | Lý do |
|---|---|---|
| 1 | Lõi trung lập + hai bộ chuyển | tiền lệ `relay-auth-status-handlers.ts` |
| 2 | Sửa `dispatcher.ts` thay vì mã trong `message` | `candidates/jobId` cần `data` |
| 3 | Không `capabilities` Part B | `-32601` đủ phân biệt |
| 4 | Điểm cắm `desktop/` tự dựng config/logger | thiếu `registerAuthStatusHandlers` |
| 5 | Task 07 độc lập O-5 | Part A qua stdio luôn cần |

## 4. Thứ tự task
```
07 (độc lập)
01 ─► 02 ─► 03 ─► 04 ─► 05 ─► 06
```
01 test lõi trung lập; 02 bộ chuyển (agent); 03 sửa `dispatcher.ts` (agent); 04 nối `relay.ts` (agent); 05 cây `desktop/`; 06 script parity + test tương đương; cần 001–005 xong.

## 5. Tiêu chí chấp nhận
- [ ] Qua `agent.js --stdio` và socket `--detach`, `codeintel.status` đúng như Part A; handshake có `codeintel*` khi binary tồn tại.
- [ ] `RelayDispatcher` thật: mọi `CODEINTEL_METHODS` đăng ký; `codeintel.nope` -> `-32601`.
- [ ] `JSON.stringify(result)` Part A bằng Part B (trừ `perf`, `startedAt`).
- [ ] `data.code`, `candidates`, `jobId` đến được qua Part B; lỗi không `data` giữ khung cũ.
- [ ] `rpc.cancel` kill CLI, không mồ côi, không gửi kết quả stale.
- [ ] Thông báo tới client B qua `dispatcher.notify`; không ghi stdout từ `codeintel-*`.
- [ ] Parity xanh; `agent/build.mjs` và `desktop/config/scripts/build-relay.mjs` build được; không `max-lines` disable mới.

## 6. Kiểm thử
`/opt/repos/orca/agent`: `pnpm exec vitest run src/relay/codeintel-relay-handlers.test.ts src/relay/codeintel-wire-parity.test.ts src/relay/agent-connection-stdio-codeintel.test.ts src/relay/dispatcher.test.ts` (`dispatcher.test.ts` **chưa kiểm** tên file thật; tìm bằng `ls src/relay | grep dispatcher`). `desktop`: `pnpm exec vitest run src/relay/codeintel-relay-handlers.test.ts` (cấu hình vitest của `desktop/` chưa kiểm). Script: `node desktop/config/scripts/check-codeintel-relay-parity.mjs`.

## 7. Rủi ro và điểm chưa kiểm chứng
Chưa kiểm chứng Go có dùng Part B ở đâu; hai cây lệch và không có script đồng bộ; `err.data` chưa rà hết người dùng; nhiều phiên đồng thời trong `--detach`; Node 18 ở relay từ xa (`node:sqlite` vắng -> CLI; API khác chưa kiểm); `loadAgentConfig()` ở chế độ socket của `relay.ts` chưa kiểm chứng; Part B tin renderer hơn nhưng giữ quy tắc nghiêm như Part A.

## 8. Câu hỏi mở
1. O-5/Q1: có cần đường Electron? 2. Q2: chấp nhận sửa `dispatcher.ts` hai cây? 3. Q3: vị trí script parity. 4. Q4: daemon `--detach` có nhiều phiên đồng thời thật không (cần danh sách notifier).

## 9. Tham chiếu
CR-CV-006; contract §3.4, §8; `dispatcher.ts` (hai cây), `relay.ts` (hai cây), `relay-auth-status-handlers.ts`, `build-relay.mjs`.
