# AG-CV-SOL-073: Công tắc tắt cứng tại máy (`ORCA_CODEINTEL_DISABLED`) cho `agent/`

> 📋 Proposed, chưa triển khai. Ngày soạn 2026-10-06. Mọi dòng "đã đọc" là đọc code/tài liệu thật; chưa chạy gì.

**CR:** [CR-CV-073](../../../../../../docs/crs/v7/quality-rollout/CR-CV-073-e2e-feature-flag-rollout-runbook.md) (P0) — chỉ dòng "Agent" của mục 2.1, bước 3 của mục 2.7 và runbook; phần còn lại thuộc BE/FE
**Service:** `agent/`
**Hợp đồng chuẩn tắc:** [`CONTRACT-codeintel-agent-rpc.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-agent-rpc.md) (§1.3, §2.4, §3.2, §4.1), [`CONTRACT-codeintel-proto-and-data-map.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) (PQ-01, PQ-23, PQ-24), [`CONTRACT-codeintel-ui-api.md`](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md)
**TDD tham chiếu:** [v5/04-handshake-session](../../../../tdd/v5/04-handshake-session.md), [v5/07-jsonrpc-dispatch](../../../../tdd/v5/07-jsonrpc-dispatch.md), [v5/08-deployment](../../../../tdd/v5/08-deployment.md)
**Mẫu định dạng:** `specs/agent/crs/v6/agent-capabilities/solutions/`

## 0. Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| Agent contract §2.4: `ORCA_CODEINTEL_DISABLED=1` (tắt cứng, CR-073), `ORCA_CODEINTEL_REINDEX=off`, `ORCA_QUALITY_RUN=off` | Ba công tắc; một bộ đọc duy nhất (task 01) |
| §3.2: `CODEINTEL_TOOL_UNAVAILABLE` (-32000), `reason ∈ {unsupported_version, unsupported_platform, reindex_disabled, quality_disabled, schema_version_unsupported}`, "tắt cứng" nằm ở cột "Khi nào" nhưng **không có `reason` riêng** | Dùng `reason:"codeintel_disabled"` (mới, đề nghị bổ sung) |
| §1.3: capability `codeintel*`, `quality` thêm có điều kiện; `-32601` ⇒ agent không hỗ trợ (infra-fleet đổi thành `CODEINTEL_AGENT_UNSUPPORTED`) | Công tắc **không** được trả `-32601` (sẽ bị hiểu là agent cũ) |
| §4.1: `codeintel.status` "luôn thành công khi agent chạy" | `status` là ngoại lệ duy nhất, trả trạng thái tắt |
| PQ-01/PQ-24: cờ backend `CODEINTEL_DISABLED` (gateway/service), cache cờ 5 s | Công tắc agent độc lập, ở tầng máy; không thay cờ tenant |
| PQ-23: tiền tố env phía agent là `ORCA_`, backend `CODEINTEL_` | Tên biến giữ nguyên |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `agent/src/relay/agent-config.ts` (`loadAgentConfig`: chỉ đọc `process.env`), `agent-session-capabilities.ts` (`buildCapabilities` trả chuỗi tự do; `STATIC_CAPABILITIES_FALLBACK` không có `codeintel`), `agent-session-handshake.ts` (gửi `capabilities`, `tools: tools.map(t=>t.name)`), `agent-rpc-dispatch.ts` (chuỗi `dispatchXxxRpc` rồi `MethodNotFound`), `agent-rpc-dispatch-misc.test.ts` (khuôn test), `deploy/agent/orca-agent.service` (không `EnvironmentFile`; `Environment=` chỉ HOME/PATH/NODE_ENV), `deploy/agent/agent-runtime.env.example`, `deploy/agent/scripts/start-agent-direct.sh` (đọc `../.env`).

| Điểm | Hiện trạng | Hệ quả |
|---|---|---|
| Mã `codeintel`/`quality`, `ORCA_CODEINTEL_*` | Chưa có (grep không thấy) | Mọi thứ "(mới)"; cắm vào dispatcher của AG-CV-SOL-001/081 |
| Nạp `.env` | Agent chỉ đọc `process.env`; `.env` do script khởi động nạp. `start.sh` mà unit gọi (`/home/ubuntu/orca-agent/start.sh`) **không có trong repo**, chưa kiểm chứng nó nạp `.env` hay không | Runbook phải ghi cách đặt biến thật sự có hiệu lực; kiểm trên dev server (task 05) |
| Tool cũ `gitnexus`/`codegraph` qua `tools/call` | Luôn có khi binary tồn tại, không phụ thuộc công tắc | Công tắc **không** chặn chúng (ngoài hợp đồng; câu hỏi 3) |

### 1.1 Lệch giữa CR và hợp đồng

| # | CR | Hợp đồng/thực tế | Xử lý |
|---|---|---|---|
| 1 | "Phương thức `codeintel.*` luôn có và không ai gọi khi cờ tắt" | Hợp đồng thêm `ORCA_CODEINTEL_DISABLED`, nhưng không nói hành vi | Định nghĩa ở mục 2 |
| 2 | "(nếu CR-CV-001 làm)" | Hợp đồng §2.4 đã **quyết** có biến này | Làm |
| 3 | Mã lỗi khi tắt cứng | Không có `reason` | `codeintel_disabled` (đề nghị bổ sung vào §3.2) |
| 4 | Runbook: "sau nâng cấp, `codeintel.status` phải hiện phương thức mới trong `tools[]`" | `tools[]` chỉ là tên công cụ (`gitnexus`, `codegraph`); hợp đồng §1.3 nói backend không dựa `tools[]` | Dùng `capabilities[]` và `codeintel.status`, không `tools[]` |

### 1.2 Phụ thuộc chéo khu vực

| Đối ứng | Quan hệ |
|---|---|
| `BE-CV-SOL-073-settings-flag-and-rollout` | Runbook bước 3 ("tắt tại máy"); backend thấy `capabilities` thiếu `codeintel` hoặc lỗi `TOOL_UNAVAILABLE reason=codeintel_disabled`; BE e2e (agent giả) có thể dùng tệp vàng `errors/tool-unavailable-*.json` — cần thêm ca `codeintel_disabled` (điều phối với BE-CV-SOL-070) |
| `FE-CV-SOL-073-flag-gating-and-web-e2e` | Không việc trực tiếp; UI nhận `CODEINTEL_TOOL_UNAVAILABLE` đã có hành vi riêng |
| `AG-CV-SOL-001, 004, 081` | Tiêu thụ `RuntimeSwitches` (reindex, quality) |

## 2. Giải pháp

### 2.1 Cây file

```
agent/src/relay/codeintel/
  runtime-switches.ts            (mới) đọc ORCA_CODEINTEL_DISABLED / ORCA_CODEINTEL_REINDEX / ORCA_QUALITY_RUN
  runtime-switches.test.ts       (mới)
  disabled-gate.ts               (mới) cổng đứng đầu dispatchCodeIntelRpc/dispatchQualityRpc
  disabled-gate.test.ts          (mới)
  disabled-capabilities.test.ts  (mới)
  disabled-startup.test.ts       (mới)
  disabled-dispatch.integration.test.ts (mới)
deploy/agent/agent-runtime.env.example  (sửa, task 05)  deploy/agent/README.md (sửa, task 05)
```

### 2.2 Bộ đọc

```ts
export type RuntimeSwitches = Readonly<{
  codeintelDisabled: boolean   // ORCA_CODEINTEL_DISABLED
  reindexDisabled: boolean     // ORCA_CODEINTEL_REINDEX=off, hoặc codeintelDisabled
  qualityDisabled: boolean     // ORCA_QUALITY_RUN=off, hoặc codeintelDisabled
  warnings: readonly string[]  // giá trị lạ (không chứa giá trị của env khác)
}>
export function readRuntimeSwitches(env: NodeJS.ProcessEnv): RuntimeSwitches
```
Quy tắc: `ORCA_CODEINTEL_DISABLED` ∈ {`1`,`true`,`yes`,`on`} (không phân biệt hoa thường, trim) → tắt; ∈ {``,`0`,`false`,`no`,`off`, vắng} → bật; **giá trị khác → tắt (fail closed)** và `warnings`. `ORCA_CODEINTEL_REINDEX` và `ORCA_QUALITY_RUN`: chỉ `off` (sau trim, hoa thường) là tắt; giá trị khác bị bỏ + cảnh báo (đúng quy tắc "giá trị sai bị bỏ" của hợp đồng §2.3 cho các biến cấu hình). Đọc **một lần** khi dựng dispatcher (cần khởi động lại để đổi; khớp runbook `systemctl restart`).

### 2.3 Hành vi khi `codeintelDisabled`

| Phương thức | Kết quả |
|---|---|
| `codeintel.status` | thành công; phong bì chuẩn nhưng **không spawn, không đọc fs/git**: `data.tools.<t>.available:false, supported:false`, `indexes.*.state:"unknown"`, `binding:null`, `limits.maxConcurrentTools:0`, `warnings:["codeintel_disabled"]` (mới) |
| mọi `codeintel.*` và `quality.*` khác | `-32000` `CODEINTEL_TOOL_UNAVAILABLE`, `data:{code, reason:"codeintel_disabled"}`; kiểm **trước** validate tham số, trước `workspaceRoot` (tránh oracle đường dẫn), trước cache; không spawn, không fs |
| thông báo (`indexChanged`, `reindexProgress`, `quality.*`) | không phát; không bộ theo dõi (`watch`), không quét nhật ký job khởi động |
| handshake | `capabilities` **không** có `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph`, `quality`; `tools[]` giữ nguyên |
| log | một dòng `warn` khi khởi động ("codeintel disabled by ORCA_CODEINTEL_DISABLED"); không in giá trị biến khác |

Công tắc này tắt cả `quality.*` (ba mã `quality_disabled` vẫn dành cho `ORCA_QUALITY_RUN=off`). `reindexDisabled` (env riêng): `codeintel.reindex` → `TOOL_UNAVAILABLE reason="reindex_disabled"`; đọc vẫn chạy. `qualityDisabled` riêng: `quality.run` → `reason="quality_disabled"`; `runStatus/results/cancel/coverage` vẫn đọc được kết quả cũ.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Fail closed với giá trị lạ của `ORCA_CODEINTEL_DISABLED` | Công tắc an toàn; gõ nhầm không được làm bật |
| D2 | Phương thức vẫn đăng ký, trả `TOOL_UNAVAILABLE` (không `-32601`) | `-32601` bị Go đổi thành `AGENT_UNSUPPORTED` (sai nghĩa) |
| D3 | `status` ngoại lệ | Hợp đồng §4.1 "luôn thành công" |
| D4 | Thiếu capability trong handshake | Backend biết sớm, không gọi |
| D5 | Đọc một lần khi khởi động | CR: khởi động lại agent; tránh trạng thái nửa vời |
| D6 | Không chặn tool cũ `gitnexus`/`codegraph` | Ngoài hợp đồng; đổi hành vi hiện hữu |
| D7 | Tắt cứng tắt luôn `quality.*` | Mọi mã đều tiền tố `CODEINTEL_`; đã có `ORCA_QUALITY_RUN` để tắt riêng quality |

## 4. Tiêu chí chấp nhận

- [ ] Bảng 2.3 đúng cho mọi method trong `CODEINTEL_METHODS` và `QUALITY_METHODS` (test lặp theo hằng; thêm method mà quên cổng → đỏ).
- [ ] Không có `spawn`/`fs`/`git` nào được gọi khi tắt (spy); `-32601` không bao giờ.
- [ ] Handshake không có capability `codeintel*`/`quality`; có lại sau khi bỏ biến và dựng lại dispatcher.
- [ ] Giá trị lạ → tắt + cảnh báo; không rò giá trị env khác.
- [ ] Runbook trong `deploy/agent/` hướng dẫn đặt biến và restart; đã thử trên một dev server (hoặc ghi "chưa thử").
- [ ] Không phụ thuộc mới; không `max-lines` disable.

## 5. Kiểm thử

Theo task. Lệnh: `pnpm exec vitest run src/relay/codeintel/<tệp>` trong `/opt/repos/orca/agent`; chạy trong job `code-intel-contract` của AG-CV-SOL-070 (`include` phải phủ `disabled-*.test.ts`, `runtime-switches.test.ts`). Chưa chạy.

## 6. Rủi ro và điểm chưa kiểm chứng

- Cách biến môi trường thật sự tới tiến trình agent trên dev server (`start.sh` không có trong repo; unit không có `EnvironmentFile`).
- Backend đang giữ kết nối cũ có capability cũ đến khi agent kết nối lại sau restart (hợp lý, vì restart đóng WS); chưa kiểm.
- Cache `capabilities` ở backend (CR-023, `GetAgentCapabilities`) có thể giữ giá trị cũ ngắn hạn.
- Tool cũ vẫn chạy được (D6): người vận hành tưởng đã tắt hết.
- SSH `--stdio` (Part A): biến phải có trong môi trường của tiến trình SSH exec, thường không nạp `.env`; chưa kiểm chứng cách đặt ở chế độ này.

## 7. Câu hỏi mở và điểm hợp đồng thiếu

1. **Hợp đồng thiếu:** `reason="codeintel_disabled"` (§3.2) và cảnh báo `codeintel_disabled` ở `status` (§4.1/§2.2).
2. Công tắc có tắt `quality.*` không (D7)? Cần xác nhận với BE-CV-SOL-073.
3. Có chặn `gitnexus`/`codegraph` ở `tools/call` khi tắt (xem AG-CV-SOL-072 câu hỏi 1)?
4. Cách đặt biến cho `--stdio`/SSH (Part A).
