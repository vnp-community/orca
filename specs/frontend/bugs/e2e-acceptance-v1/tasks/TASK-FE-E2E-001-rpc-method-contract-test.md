# TASK-FE-E2E-001: Test hợp đồng: mọi kênh frontend gọi đều có ở gateway

**From Solution:** SOL-FE-E2E-001 · **Resolves:** FE-E2E-C
**Priority:** P0 · **Area:** frontend
**Files:**
- frontend/src/shared/request-rpc-methods.ts
- frontend/src/shared/request-rpc-contract.test.ts (mới)
- backend-go/services/api-gateway/internal/adapter/wscompat/testdata/channel_inventory.json (đọc)
**Depends on:** TASK-E2E-003
**Status:** `[x]` DONE (2026-10-10)

---

## Context

Không có kiểm tra nào so tên kênh frontend với gateway; `clarification.*` thiếu mà mọi test xanh.

## Việc cần làm

- [x] Vitest đọc tệp vàng bằng `fs` (đường dẫn tương đối từ gốc repo), so với giá trị của `REQUEST_RPC_METHODS` và các bảng kênh Request khác.
- [x] Danh sách cho phép kênh chưa có ở gateway, mỗi mục có lý do (xem "Kết quả thực thi": không có kênh nào chỉ-Electron; 17 kênh là tên hợp đồng của CR chưa ship).
- [x] Thông báo lỗi nêu tên kênh thiếu và gợi ý cập nhật tệp vàng backend.

## Tiêu chí hoàn thành

- [x] Xoá một kênh khỏi tệp vàng làm test đỏ (đã chạy thật bằng bản sao tệp vàng thiếu `clarification.answer`, xem dưới).
- [x] Test xanh ở HEAD sau TASK-E2E-001/003.

## Kiểm thử

Vitest.

## Kết quả thực thi

**File đổi**
- `frontend/src/shared/request-rpc-contract.test.ts` (mới, 6 test).
- `frontend/src/shared/request-rpc-methods.ts`: thêm `PLAN_PROPOSAL: 'request.planProposal'` (kênh có ở gateway, UI cần cho TASK-FE-E2E-005; xem file đó).
- `frontend/src/renderer/src/runtime/request-rpc-client.ts`: `'request.subscribe'` viết cứng đổi sang `REQUEST_RPC_METHODS.SUBSCRIBE` để mọi tên kênh đi qua bảng được kiểm.

**Test làm gì**
1. Đọc `channel_inventory.json` (532 kênh), từ chối nếu tệp thiếu (lỗi nêu đường dẫn; biến `ORCA_GATEWAY_CHANNEL_INVENTORY` trỏ sang tệp khác, dùng cho CI tách repo và cho phép thử "xoá kênh").
2. Mọi giá trị của `REQUEST_RPC_METHODS` phải có trong danh mục, trừ `PENDING_GATEWAY_CHANNELS`. Lỗi nêu tên kênh thiếu và lệnh `UPDATE_GOLDEN=1 go test ./internal/adapter/wscompat -run TestChannelInventoryMatchesGolden`.
3. `clarification.list/get/answer/cancel` bị chốt riêng (đúng bốn kênh BUG-E2E-001 làm mất).
4. Danh sách cho phép chỉ co lại: mục nào gateway bắt đầu đăng ký thì test đỏ ("xoá khỏi allowlist"); mục không còn trong `REQUEST_RPC_METHODS` cũng đỏ.
5. Kênh còn lại phải là `unary`, riêng `request.subscribe` phải là `stream`.

**Kênh cho phép (17)**: `impact.{graph,heatmap,drift,get,request,compare,findings,evidence,accept}`, `risk.override`, `request.readiness`, `decision.{list,confirm}`, `readiness.{check,get,list}`, `execution.get`. Lý do thật: gateway chưa đăng ký các kênh này (đối chiếu tệp vàng và `grep` trong `backend-go`); UI đã coi `unsupported` là "chưa có tính năng" (`useDecisions`, `useImpactAssessment`, `useTaskReadiness`, `useExecutionResult`, `usePlanHeatmap`, `useGraphLens`). **Không có kênh nào chỉ-Electron** trong `REQUEST_RPC_METHODS`.

**Lệnh đã chạy** (từ `frontend/`, `NODE_OPTIONS=--no-experimental-webstorage`)
- `pnpm exec vitest run --config config/vitest.config.ts src/shared/request-rpc-contract.test.ts` → 6/6 xanh.
- Cùng lệnh với `ORCA_GATEWAY_CHANNEL_INVENTORY=<bản sao bỏ clarification.answer>` → 2 test đỏ, thông báo `Channels in REQUEST_RPC_METHODS missing from the gateway inventory: clarification.answer`.
- `oxlint` trên file mới: 0 lỗi.

**Blast radius (rà tay, không có gitnexus/codegraph)**: chỉ thêm một khoá vào `REQUEST_RPC_METHODS` và thay một chuỗi bằng hằng; `RequestRpcMethod` rộng thêm một giá trị, không `switch` nào theo union này. Hai target (Electron/web) dùng chung bảng.

**Chưa làm / chưa kiểm**: test chỉ bắt tên kênh, không bắt hình dạng tham số/kết quả (xem lỗi thật đã gặp ở TASK-FE-E2E-003: `options` gửi `{id,label}`, UI đọc `value`).
