# FE-REQ-TASK-044-02: Hook `usePlanRunState`, hàm thuần `derivePlanAutoDisplay` và hành động Retry

**From Solution:** [FE-REQ-SOL-044](../solutions/FE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/hooks/usePlanRunState.ts` (mới) và `usePlanRunState.test.ts` (mới), `frontend/src/renderer/src/hooks/usePlanRetry.ts` (mới) và `usePlanRetry.test.ts` (mới), `frontend/src/renderer/src/components/request/plan/plan-auto-display.ts` (mới) và `plan-auto-display.test.ts` (mới), `frontend/src/renderer/src/hooks/usePlanRunInFlight.ts` và `usePlanRunInFlight.test.ts` (xoá sau khi chuyển người gọi ở task 03)
**Depends on:** FE-REQ-TASK-044-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `hooks/usePlanRunInFlight.ts`: bật khi `status === 'planning' && !planTaskId`; hỏi `request.planProposal {id}` mỗi 4 giây (`PLAN_RUN_PRESENCE_POLL_MS`); chỉ `running` là bận; lỗi (kể cả "run not found") là "không chạy"; gọi `onFinished` khi bận chuyển sang không bận. Test hiện có: `usePlanRunInFlight.test.ts` (đồng hồ giả, mock `callRequestRpc`). Người gọi duy nhất: `RequestPlanTab.tsx:72`.
- Luồng thủ công: `usePlanDecision.regeneratePlan` (`hooks/usePlanDecision.ts:113`) → `generatePlanProposeCommit` (`hooks/request-plan-generation.ts:113`).
- Làm nguội: `useRequestCooldown('ai')` (`hooks/useRequestCooldown`), dùng trong `PlanStates.tsx`.
- Bảng quyết định hiển thị: solution mục 2.3; hành động Retry: mục 2.5.

## Việc cần làm

1. `plan-auto-display.ts`: `derivePlanAutoDisplay({request, run, supportsPlanAuto, graceElapsed}): PlanAutoDisplay` thuần, với `PlanAutoDisplay` = `{kind:'none'}` | `{kind:'writing', attempt, maxAttempts, waiting, replan, starting}` | `{kind:'failed', code, viaServer}` | `{kind:'skipped', reasonCode}` theo bảng ở solution mục 2.3 (gồm trường hợp gateway cũ: `failed` từ `run.status === 'failed'` dù `autoPlan === null`). Request không `planning` → `none`.
2. `usePlanRunState(request, onFinished)`: bật khi `request.status === 'planning'` (bất kể `planTaskId`); hỏi `request.planProposal {id}` ngay khi bật rồi mỗi `PLAN_RUN_STATE_POLL_MS` (4000); phân tích bằng `parsePlanRunView`; "run not found" (lỗi RPC) là `view=null`; trả `{view, busy, refresh}` với `busy` = `run.status === 'running'` hoặc `autoPlan.state` ∈ `pending|running|committing`; gọi `onFinished` khi `busy` chuyển từ true sang false; theo dõi thời điểm đầu tiên thấy trạng thái "chưa job, chưa run, `enabled`" để cung cấp `graceElapsed` sau `PLAN_AUTO_START_GRACE_MS` (20000); dọn timer khi tháo hoặc khi Request rời `planning`.
3. `usePlanRetry({request, run, regenerateManually, refresh})`: `retry()`: nếu `supportsPlanAuto` (đọc store) và `run?.autoPlan?.enabled` thì `callRequestRpc(RETRY_PLAN, {id}, {idempotent: true})` rồi `refresh()`; nếu lỗi có mã `REQUEST_PLAN_AUTO_DISABLED` hoặc `enabled` sai thì `regenerateManually()`. Trả `{retry, retrying, error}`; `error` qua `requestErrorMessageForError`; chặn bấm lặp khi đang bay.
4. Không xoá `usePlanRunInFlight` trong task này nếu còn người gọi; task 03 chuyển `RequestPlanTab` rồi xoá cả hai file cũ.

## Kiểm thử

- `derivePlanAutoDisplay`: bảng đầy đủ — status Request {`planning`, `awaiting_plan_approval`, `analyzing`} × `run` {null, `running`, `succeeded`, `failed`} × `autoPlan` {null, `enabled:false`, `pending` lượt 1, `pending` lượt 2 `waiting`, `running`, `committing`, `gave_up`, `skipped`, `done`} × `planTaskId` {có/không} × `graceElapsed`; kiểm riêng: gateway cũ + run `failed` → `failed`; job `gave_up` không run (hết hạn bận) → `failed` với `reasonCode`; `enabled` + chưa job + chưa run + chưa hết ân hạn → `writing` `starting`; sau ân hạn → `none`.
- `usePlanRunState`: bật cả khi có `planTaskId`; `busy` cho `running` và cho `pending|running|committing`; `onFinished` đúng một lần; dừng gọi khi Request rời `planning`; lỗi "run not found" không gây `busy`; ân hạn chạy bằng đồng hồ giả.
- `usePlanRetry`: đường server (gọi `RETRY_PLAN`, rồi `refresh`); rơi về thủ công khi `REQUEST_PLAN_AUTO_DISABLED`; rơi về thủ công khi `enabled=false` hoặc gateway cũ; bấm hai lần chỉ một lời gọi; lỗi khác hiện trong `error`.
- `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/hooks/usePlanRunState.test.ts src/renderer/src/hooks/usePlanRetry.test.ts src/renderer/src/components/request/plan/plan-auto-display.test.ts` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Trạng thái hiển thị suy ra thuần từ run và `autoPlan`, đúng bảng, kể cả gateway cũ. (`plan-auto-display.test.ts`, gồm bảng đầy đủ 3 status x 4 run x 7 job x planTaskId x grace)
- [x] `usePlanRunState` coi run `running` là bận cả khi `planTaskId` đã có. (`usePlanRunState.test.ts`: "polls although the Request already has a Plan")
- [x] Retry đi qua server khi được, rơi về luồng thủ công khi không; không bấm kép. (`usePlanRetry.test.ts`)
- [x] Không còn người gọi `usePlanRunInFlight` sau task 03. (đã xoá file và test; grep không còn tham chiếu trong `frontend/src`)

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `usePlanRunInFlight` và `usePlanDecision` trước khi sửa/xoá.
- Ân hạn "starting" đo ở client (20 giây): trang mở trên Request kẹt sẽ hiện "starting" tối đa 20 giây rồi trả về nút Generate; chấp nhận được.

## Ghi chú triển khai (2026-10-20)

- **File**: mới `components/request/plan/plan-auto-display.ts`, `hooks/usePlanRunState.ts`, `hooks/usePlanRetry.ts`, `hooks/usePlanAutoGeneration.ts` (hook ghép ba phần cho `RequestPlanTab`, để tab không phình) và các test cùng tên; xoá `hooks/usePlanRunInFlight.ts` + test.
- **Lệnh**: `npx vitest run --config config/vitest.config.ts src/renderer/src/hooks/usePlanRunState.test.ts src/renderer/src/hooks/usePlanRetry.test.ts src/renderer/src/components/request/plan/plan-auto-display.test.ts` xanh (65 test); `oxlint` và `oxlint --config frontend/config/oxlint-react-doctor.json` sạch.
- **Thiết kế**: `usePlanRunState` trả thêm `graceElapsed`; state theo `requestId` (snapshot) thay cho `setState` trong effect để qua `react-doctor/no-adjust-state-on-prop-change`; `refresh()` trong lúc một lần đọc đang bay sẽ đọc lại ngay sau đó. `usePlanRetry` bỏ qua lỗi `IN_FLIGHT` của `decision.regeneratePlan`. `derivePlanAutoDisplay` bỏ qua `autoPlan` khi `supportsPlanAuto` sai; mã mặc định `REQUEST_INTERNAL` khi run `failed` không có mã (cùng luồng thủ công).
- **Blast radius**: `usePlanRunInFlight` chỉ có `RequestPlanTab` gọi (đã chuyển). `usePlanDecision` không sửa.
- **Điểm cần backend làm rõ (quan trọng)**: BE task 07 nói "không run và không job thì vẫn `REQUEST_PLAN_RUN_NOT_FOUND`". Khi đó giao diện không biết `enabled` trong khoảng giữa duyệt Analysis và lúc job được tạo, nên "starting" không xuất hiện và nút Generate có thể hiện vài giây rồi mất. Để "starting" hoạt động, `request.planProposal` cần trả view rỗng kèm `autoPlan{enabled:true}` (không lỗi) khi tính năng bật; client cũ coi view rỗng giống "không có run". Tương tự cho công tắc dự án tắt: spec e2e (e) chờ `autoPlan.enabled=false` từ API.
- **Chưa kiểm**: hành vi với gateway thật.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
