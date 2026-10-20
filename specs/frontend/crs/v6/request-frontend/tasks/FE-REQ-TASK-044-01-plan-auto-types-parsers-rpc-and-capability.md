# FE-REQ-TASK-044-01: Kiểu, parser không bao giờ ném lỗi, kênh `request.retryPlan` và cờ tính năng `request.planAuto`

**From Solution:** [FE-REQ-SOL-044](../solutions/FE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/shared/request-plan-auto.ts` (mới), `frontend/src/shared/request-plan-auto.test.ts` (mới), `frontend/src/shared/request-rpc-methods.ts` (sửa), `frontend/src/renderer/src/store/slices/request.ts` (sửa), `frontend/src/renderer/src/store/slices/request.test.ts` (sửa)
**Depends on:** backend TASK-REQ-044-07, TASK-REQ-044-08 (hợp đồng `autoPlan`, `request.retryPlan`, `request.planAuto`)
**Status:** [ ] TODO

---

## Context

- `request.planProposal` hiện được đọc bằng kiểu đặt tại chỗ (`ProposeValue` trong `hooks/request-plan-generation.ts`, `RunView` trong `hooks/usePlanRunInFlight.ts`), không có parser chung.
- Cơ chế cờ tính năng đã có từ CR-REQ-040: `store/slices/request.ts:56` (`requestFlowFeatures: ReadonlySet<string>`), `:78-79` (`supportsBaseRef`), `:97` (`setRequestFlowFeatures`), nạp bởi `hooks/useRequestFlowSupport.ts` từ `request.flowStatus.features`; hằng `REQUEST_BASE_REF_FEATURE` ở `shared/request-base-ref.ts`.
- `shared/request-rpc-methods.ts` có `GENERATE_PLAN: 'request.generatePlan'` và `PLAN_PROPOSAL: 'request.planProposal'`, chưa có kênh thử lại. Bộ kiểm hợp đồng: `shared/request-rpc-contract.test.ts`.
- Quy ước parser: không bao giờ ném lỗi; gateway cũ thiếu trường vẫn hiển thị đúng (`shared/request-wire-parsers.ts`).

## Việc cần làm

1. `shared/request-plan-auto.ts`: `REQUEST_PLAN_AUTO_FEATURE = 'request.planAuto'`; kiểu `PlanAutoState` (`pending|running|committing|done|gave_up|skipped|superseded|''`), `PlanAutoOrigin` (`event|retry|''`), `PlanAutoView {enabled, state, origin, attempt, maxAttempts, reasonCode, nextAttemptAt?}`, `PlanRunStatus` (`running|succeeded|failed|''`), `PlanRunView {runId, status, errorCode, errorMessage, engine, autoPlan: PlanAutoView | null}`.
2. `parsePlanRunView(raw: unknown): PlanRunView`: không ném lỗi với đầu vào bất kỳ (`null`, mảng, chuỗi); khoá thiếu → giá trị rỗng/0/`false`; `autoPlan` thiếu hoặc không phải object → `null` (khác `enabled:false`); `state`, `origin`, `status` lạ → `''`; `attempt`, `maxAttempts` không phải số hữu hạn không âm → 0.
3. `REQUEST_RPC_METHODS.RETRY_PLAN = 'request.retryPlan'` (có chú thích ngắn nêu CR-REQ-044); cập nhật `request-rpc-contract.test.ts` nếu nó liệt kê kênh.
4. `store/slices/request.ts`: `supportsPlanAuto(state)` = `requestFlowFeatures.has(REQUEST_PLAN_AUTO_FEATURE)`.

## Kiểm thử

- Parser: bảng đầu vào hợp lệ đầy đủ (có `autoPlan`), không có `autoPlan` (gateway cũ → `null`), `autoPlan.enabled=false`, `state` lạ, số âm, kiểu sai, `null`, `undefined`, chuỗi, mảng.
- `supportsPlanAuto`: có và không có tên trong tập.
- `cd frontend && npx vitest run --config config/vitest.config.ts src/shared/request-plan-auto.test.ts src/shared/request-rpc-contract.test.ts src/renderer/src/store/slices/request.test.ts` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Parser không ném lỗi với mọi đầu vào thử.
- [ ] `autoPlan: null` phân biệt được với `enabled:false`.
- [ ] `supportsPlanAuto` đúng; kênh mới có trong danh sách RPC và test hợp đồng.

## Rủi ro và lưu ý

- Số `maxAttempts` backend gửi là tổng số lượt (1 + lượt thêm); giao diện hiện "Attempt n of max" nên đối chiếu với hợp đồng khi backend xong.
- Chưa chạy: kiểm kiểu bằng `npx tsc --noEmit -p tsconfig.json --composite false --incremental false` (mốc lỗi có sẵn của repo, so với mốc trước khi sửa).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
