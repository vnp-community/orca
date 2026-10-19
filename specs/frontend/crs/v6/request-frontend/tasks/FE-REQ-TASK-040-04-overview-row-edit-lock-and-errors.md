# FE-REQ-TASK-040-04: Dòng nhánh gốc ở Overview: xem, sửa, khoá, lỗi; hành động ở thẻ lỗi Execution

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/request/{RequestBaseRefRow.tsx,RequestOverviewTab.tsx}` (sửa), `components/request/execution/ExecutionFailureCard.tsx` + `execution-failure-cause.ts` (sửa), test
**Depends on:** FE-REQ-TASK-040-01, 040-02
**Status:** [ ] PARTIAL: mọi việc đã làm và kiểm bằng test đơn vị; chỉ còn tiêu chí "Cập nhật thời gian thực" chưa kiểm với sự kiện thật (xem Ghi chú triển khai)

---

## Context

- `request.setBaseRef` dùng `expectedVersion`; xung đột phải tải lại và hỏi (C12).
- Thẻ lỗi Execution hiển thị nguyên nhân từ `request.executionSummary` (`lastCause`).

## Việc cần làm

1. `RequestBaseRefRow` theo SOL-040 mục 2.4: giá trị, bút, ổ khoá + tooltip theo `baseRefLockReason`, cảnh báo `baseRefUnverified`.
2. Lưu bằng `callRequestRpc(SET_BASE_REF, {id, baseRef, expectedVersion})`; thành công cập nhật store và `version`.
3. Xử lý `REQUEST_VERSION_CONFLICT` (tải lại + hỏi), `REQUEST_BASE_REF_LOCKED` (toast trung tính, làm mới, bỏ nút sửa), `INVALID`/`NOT_FOUND` (dưới ô).
4. Quyền: ẩn bút nếu người xem không có quyền sửa (theo kiểu `ChangeTypeConfirmDialog`); nếu lỡ gọi, hiển thị lỗi quyền chuẩn.
5. Ẩn dòng khi `!supportsBaseRef` hoặc khi luồng của loại đó không có thực thi và không có Plan OpenSpec (`question`, `spike`) theo `request.flow`.
6. `execution-failure-cause.ts`: thêm nguyên nhân `base_ref_not_found` (nhãn, mô tả); `ExecutionFailureCard` hiện nút "Change starting branch" khi `!baseRefLocked`.
7. Sự kiện thời gian thực (`request.subscribe`) làm mới dòng này khi người khác đổi.

## Kiểm thử

- Component: các trạng thái sửa được / khoá (3 lý do) / chưa kiểm; từng mã lỗi; xung đột; ẩn theo loại và theo hỗ trợ.
- Thẻ lỗi: nút hiện đúng điều kiện.
- Test Overview hiện có xanh.

## Tiêu chí hoàn thành

- [x] Xem, sửa, khoá và mọi lỗi hoạt động.
- [x] Thẻ lỗi `base_ref_not_found` có hành động sửa khi chưa khoá.
- [ ] Cập nhật thời gian thực.

## Rủi ro và lưu ý

- Phối hợp backend: `base_ref_not_found` phải xuất hiện trong `lastCause` (TASK-REQ-039-03).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-19)

**File đổi/tạo** (dưới `frontend/src/renderer/src/`):
- `components/request/RequestBaseRefRow.tsx` (mới): "Starting branch / Worktrees start from <ref>" hoặc "...the default branch"; bút (`Change starting branch`) khi `!baseRefLocked` và chưa mất quyền; ổ khoá chữ "Locked" + tooltip + `aria-label` theo `baseRefLockReason` (proposal, task, execution, terminal; mã lạ thành lý do chung, vẫn là khoá); cảnh báo `baseRefUnverified`. Ẩn khi `!supportsBaseRef`, khi `request.baseRef` thiếu (gateway cũ) hoặc khi `request.flow.planKind === 'none'` (`question`, `spike`; cùng quy tắc `RequestStageTimeline`: bước Execution/Plan chỉ có khi `planKind !== 'none'`). Request chưa phân loại (`type unknown`, chưa có flow) vẫn hiện dòng để không giấu nhánh đã chọn lúc tạo.
- `components/request/use-request-base-ref-save.ts` (mới): `callRequestRpc(SET_BASE_REF, {id, baseRef, expectedVersion})` (bỏ `expectedVersion` và tắt retry khi không biết phiên bản). Thành công: `verifyBaseRefEcho` (gateway bỏ qua trường thì báo "This server didn't apply the starting branch.", không báo thành công), rồi gộp chỉ các trường nhánh/khoá/`version`/`updatedAt` vào store (câu trả lời mỏng hơn `request.get`, không được xoá `links`/`engine`) và toast "Starting branch updated". `REQUEST_BASE_REF_LOCKED`: toast trung tính + `request.get` làm mới (dòng chuyển sang ổ khoá, bỏ bút). `REQUEST_VERSION_CONFLICT`: tải lại bằng `request.get`, nếu khác thì hỏi "Someone else changed this request first. It now starts from X. Your change was to Y." với "Apply my change" (thử lại với phiên bản mới) / "Keep the current one"; nếu giống thì không hỏi. `INVALID`/`NOT_FOUND`: dưới dòng, kèm gợi ý bấm để lưu. `FORBIDDEN`/`REQUEST_ACTOR_NOT_ALLOWED`: lỗi quyền chuẩn và bỏ bút. `useFreshestRequest` lấy bản mới hơn giữa prop và store.
- `components/request/BaseRefChangeControl.tsx`, `BaseRefSaveFeedback.tsx` (mới): bộ chọn nhánh cho Request đang có (repo đầu của dự án, không hiện gì khi chưa biết repo) và khối thông báo dùng chung cho dòng Overview và thẻ lỗi.
- `components/request/RequestOverviewTab.tsx` (sửa): thêm `<RequestBaseRefRow key={request.id} .../>` dưới lưới thông tin (`key` để lỗi/quyền không lọt sang Request khác).
- `components/request/execution/execution-failure-cause.ts` (sửa): nguyên nhân `base_ref_not_found` (nhãn "Starting branch not found" + lời khuyên).
- `components/request/execution/ExecutionFailureCard.tsx` (sửa): nút "Change starting branch" chỉ khi `lastCause === 'base_ref_not_found'`, `supportsBaseRef`, có Request và `baseRefLocked !== true` (và chưa bị từ chối quyền). Prop mới `request?`; khi không truyền, lấy Request đang mở của trang từ store (`requestPage.requestId`).
- **Ngoài danh sách file được giao nhưng bắt buộc để nguyên nhân tới được thẻ** (2 dòng + 1 test): `shared/request-execution-types.ts` (`FailureCause` thêm `'base_ref_not_found'`) và `shared/request-execution-parsers.ts` (`FAILURE_CAUSES` thêm giá trị này; nếu không, parser đổi nó thành `unknown` và thẻ không bao giờ thấy nguyên nhân) cùng ca test trong `request-execution-parsers.test.ts`. Không file nào trong hai file này đang được người khác sửa (git status sạch).
- i18n: dùng chung 55 khoá của task 03 (xem 040-03).

**Test mới**: `RequestBaseRefRow.test.tsx` (29: hiện/ẩn theo hỗ trợ, thiếu `baseRef`, `question`/`spike` ẩn, 4 loại khác hiện; 5 lý do khoá; khoá thời gian thực qua rerender; gửi đúng tham số + cập nhật store/phiên bản; chọn mặc định gửi `''`; chọn lại nhánh hiện tại không gọi; unverified; echo thiếu; thiếu phiên bản; INVALID; NOT_FOUND + gợi ý; LOCKED; FORBIDDEN; xung đột áp dụng/giữ/trùng), `RequestOverviewTab.base-ref.test.tsx` (2), `ExecutionFailureCard.base-ref.test.tsx` (11: nhãn/lời khuyên, nút hiện khi chưa khoá, ẩn khi khoá/nguyên nhân khác/không hỗ trợ/không có Request, lấy Request từ store, lưu bằng phiên bản đúng, lỗi dưới thẻ), `request-rate-limit.test.ts` (+2 khẳng định), `request-execution-parsers.test.ts` (+1).

**Blast radius** (grep): `RequestOverviewTab` chỉ dùng ở `RequestDetailPane` (props không đổi); `ExecutionFailureCard` chỉ dùng ở `ExecutionTab` (prop mới tuỳ chọn); `failureCauseLabel/Advice` cũng dùng ở `ExecutionTaskRow` (thêm nhánh, mặc định giữ nguyên); `FailureCause` thêm một giá trị: `execution-task-state.ts` và `activity/activity-entry-text.ts` (`activityFailureCauseLabel` có nhánh `default`) không vỡ kiểu.

**Kiểm chứng**: cùng lệnh vitest/oxlint/tsc/i18n như 040-03 (kết quả ở đó): không lỗi kiểu mới (113), oxlint sạch, test thư mục `components/request` xanh.

**Chưa kiểm / còn lại**
- **Thời gian thực**: dòng đi theo `request` mà `RequestDetailPane` truyền (nó đã `refetch` khi nhận sự kiện của Request; có test rerender). Backend phát outbox `orca.request.request.base_ref_changed`, nhưng chưa kiểm gateway có chuyển sự kiện này tới `request.subscribe` hay không: để task 05 (live).
- `lastCause = base_ref_not_found` phải do backend trả (TASK-REQ-039-03 mục 4); gateway hiện chưa thấy chuỗi này ngoài tài liệu. Nếu backend dùng tên khác, thẻ sẽ hiện "Unknown cause" và không có nút.
- Gợi ý cho agent khác (ngoài quyền): `RequestDetailPane` có thể truyền `onChanged={refetch}` cho `RequestOverviewTab` và `ExecutionTab` có thể truyền `request` cho thẻ lỗi (hiện thẻ lấy từ store); `shared/request-errors.ts` chưa ánh xạ `REQUEST_BASE_REF_*` vào `kind` (UI dựa vào `backendCode` nên không ảnh hưởng); `activity-entry-text.ts` nên có nhãn cho `base_ref_not_found` (đang rơi vào `default`).
- "Ẩn bút nếu không có quyền": frontend không biết chủ Request nên không đoán; bút hiện, nếu bị từ chối thì hiện lỗi quyền chuẩn và bỏ bút (đúng cách `ChangeTypeConfirmDialog` để backend quyết định).
- Chưa xem trực quan trong ứng dụng thật (tooltip khoá, bố cục).
