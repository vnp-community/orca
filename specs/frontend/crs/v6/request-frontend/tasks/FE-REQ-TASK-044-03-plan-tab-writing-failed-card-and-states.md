# FE-REQ-TASK-044-03: Tab Plan: "Orca is writing the plan", thẻ lỗi bền vững có Retry, ghi chú không tự chạy được, dải viết lại

**From Solution:** [FE-REQ-SOL-044](../solutions/FE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/plan/PlanAutoWriting.tsx`, `PlanRunFailedCard.tsx`, `PlanAutoSkippedNote.tsx`, `PlanRewriteStrip.tsx` (mới), `PlanStates.tsx`, `RequestPlanTab.tsx` (sửa), các file test cùng thư mục (`PlanStates.auto.test.tsx`, `PlanRunFailedCard.test.tsx`, `RequestPlanTab.auto.test.tsx` mới; `RequestPlanTab.test.tsx` sửa nếu cần), xoá `hooks/usePlanRunInFlight.ts` và `usePlanRunInFlight.test.ts`
**Depends on:** FE-REQ-TASK-044-02
**Status:** [ ] TODO

---

## Context

- `RequestPlanTab.tsx` (249 dòng; trần `max-lines` 300 cho vùng này): `usePlanRunInFlight` ở `:72`; `PlanStates` nhận `canGenerate`, `generating`, `generateError`, `generateAiCode`, `onGenerate`, `onRetry`, `loadErrorKind` ở `:123-135`; `generateFailure` từ `decision.failure` ở `:122`.
- `PlanStates.tsx` (173 dòng): `generateButton` (`:56-69`, `plan-generate`), `generateFailure` (`:70-80`), nhánh `generating` (`:95-111`), nhánh `empty`/`not_found` cũng dùng `generateButton` (`:169`).
- `RequestAiSetupBanner` (`components/request/RequestAiSetupBanner.tsx`) và `firstAiSetupCode` (`components/request/request-ai-setup.ts`) đã dùng trong `PlanStates`.
- Style: `guides/STYLEGUIDE.md`; token `assets/main.css`; primitive `components/ui` (`Button`, `Skeleton`); icon `lucide-react` như `PlanStates`.

## Việc cần làm

1. `PlanAutoWriting`: `data-testid="plan-auto-writing"`, `aria-busy`, skeleton như nhánh `generating` cũ, dòng chính "Orca is writing the plan..." (hoặc "Orca is starting the plan..." khi `starting`), dòng phụ "Attempt {{attempt}} of {{max}}" khi `attempt > 1`, và "Trying again shortly" khi `waiting`. Không có nút.
2. `PlanRunFailedCard`: `role="alert"`, `data-testid="plan-run-failed"`, biểu tượng `AlertTriangle`; tiêu đề "Plan generation failed ({{code}})"; một dòng gợi ý theo nhóm mã: timeout (`REQUEST_AI_COMPLETE_TIMEOUT`, `REQUEST_OPENSPEC_TIMEOUT`), bị gián đoạn (`REQUEST_SOLUTION_RUN_INTERRUPTED`), đầu ra không hợp lệ (`REQUEST_PLAN_AI_INVALID_JSON`, `REQUEST_OPENSPEC_INVALID_OUTPUT`), bận (`REQUEST_ANALYSIS_BUSY`, `REQUEST_AI_BUSY`), lỗi lưu Plan (`REQUEST_PLAN_COMMIT_FAILED`), khác; nếu `firstAiSetupCode(code)` có thì hiện `RequestAiSetupBanner` (có `onRetry` là Retry) thay cho dòng gợi ý; nút **Retry** (`data-testid="plan-run-retry"`, khoá khi `retrying` hoặc `useRequestCooldown('ai')`); lỗi của Retry hiện dưới nút (`role="alert"`). Mã được hiển thị nguyên văn trong dấu ngoặc.
3. `PlanAutoSkippedNote`: ghi chú trung tính "Orca could not start the plan on its own ({{reason}}). Generate it when you are ready." (`data-testid="plan-auto-skipped"`); lý do dịch được cho `REQUEST_PLAN_AI_UNAVAILABLE` ("no dev server is connected"), `REQUEST_ANALYSIS_NO_REPO_PATH`, `REQUEST_ENGINE_UNAVAILABLE`, còn lại hiện mã.
4. `PlanRewriteStrip`: dải mỏng (`data-testid="plan-rewrite-strip"`, `role="status"`) "Orca is rewriting the plan..." đặt trên cây Plan.
5. `PlanStates`: thêm prop `autoDisplay` và `onRetry` của thẻ (đặt tên `onRetryRun` để không trùng `onRetry` tải lại); nhánh `generating` hiển thị `PlanAutoWriting` / `PlanRunFailedCard` / `PlanAutoSkippedNote` theo `autoDisplay.kind`, còn `none` giữ nguyên "AI is drafting the plan..."; `generateButton` **ẩn** khi `kind === 'writing'`, và khi `failed` hiển thị dạng phụ ("Generate manually", `variant="ghost"`, vẫn `plan-generate`); `generateError` inline chỉ hiện khi `kind !== 'failed'`. Hai nhánh `empty`/`not_found` giữ nguyên.
6. `RequestPlanTab`: thay `usePlanRunInFlight` bằng `usePlanRunState` + `derivePlanAutoDisplay` + `usePlanRetry` (truyền `regenerateManually = decision.regeneratePlan`); `generating = decision.isBusy('generatePlan') || busy`; ở trạng thái `ready`, khi `autoDisplay.kind === 'writing' && replan` thì hiện `PlanRewriteStrip` ngay dưới `PlanSummaryHeader`. Nếu file chạm trần 300 dòng thì chuyển phần dựng `PlanStates` props vào một hàm riêng trong `plan-auto-display.ts` hoặc component con, không thêm `eslint-disable`.
7. Xoá `hooks/usePlanRunInFlight.ts` và `usePlanRunInFlight.test.ts`; chuyển ca của nó sang `usePlanRunState.test.ts` nếu chưa có.

## Kiểm thử

- `PlanStates`: `writing` ẩn nút Generate và hiện tiến độ (lượt 2/2 khi `attempt=2`); `failed` hiện thẻ có mã, Retry, và nút phụ; `skipped` hiện ghi chú và nút Generate; `none` y như trước (test hiện có xanh); lỗi inline không trùng thẻ; mã AI-setup hiện `RequestAiSetupBanner`.
- `PlanRunFailedCard`: từng nhóm mã cho đúng gợi ý; Retry gọi `onRetryRun` một lần, khoá khi `retrying`; lỗi Retry hiện.
- `RequestPlanTab`: sau "tải lại" (mount mới) với run `failed` thẻ vẫn hiện; replan (`planning` + `planTaskId` + run `running`) hiện dải; `planning` không có `planTaskId` và `autoPlan.state=running` hiện `writing`; gateway cũ (không `autoPlan`) với run `failed` vẫn hiện thẻ; Request `awaiting_plan_approval` không có gì thêm.
- `plan-locale-coverage.test.ts` xanh (task 04 thêm khoá).
- `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/components/request/plan src/renderer/src/hooks` (chưa chạy; bỏ qua 17 test đỏ có sẵn không liên quan nếu còn như ở CR-REQ-040).
- `npx oxlint -c .oxlintrc.json` cho thư mục đã đổi và `oxlint -c frontend/config/oxlint-react-doctor.json`; `tsc --noEmit` không tăng so với mốc.

## Tiêu chí hoàn thành

- [ ] Duyệt Analysis xong, tab Plan không còn nút Generate trong lúc Orca đang viết; hiện "Orca is writing the plan...".
- [ ] Thẻ "Plan generation failed (mã)" còn sau khi tải lại trang, có Retry; nút Generate thủ công vẫn dùng được (dạng phụ).
- [ ] Trường hợp không tự chạy được hiển thị ghi chú và nút Generate như cũ.
- [ ] Replan có dải "đang viết lại" sau khi tải lại.
- [ ] Gateway cũ hiển thị đúng; test hiện có của tab Plan xanh; không file nào vượt 300 dòng.

## Rủi ro và lưu ý

- `PlanStates` có thể dùng ở nơi khác ngoài `RequestPlanTab` (chưa kiểm); chạy `gitnexus_impact` cho `PlanStates` và `RequestPlanTab` trước khi sửa.
- Thẻ lỗi và banner AI-setup cùng có nút thử lại: chỉ một nút Retry được hiện cho mỗi trường hợp (banner dùng `onRetry` của thẻ khi có mã AI-setup).
- Chưa dựng giao diện thử: kiểm bằng ảnh chụp ở task 04.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
