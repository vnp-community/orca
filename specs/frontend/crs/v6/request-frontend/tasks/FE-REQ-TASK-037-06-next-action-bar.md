# FE-REQ-TASK-037-06: Dải "việc cần làm tiếp theo" ở đầu chi tiết Request

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.4, CR-037 mục 2.7
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/{NextActionBar,next-action-rules}.ts(x)` (mới), `RequestDetailPane.tsx` (sửa), `RequestBacklogBanner.tsx` (sửa nhỏ), test cùng tên, `i18n/locales/*.json` (khoá `NextAction.*`)
**Depends on:** FE-REQ-TASK-037-03, 037-05
**Status:** [x] DONE

---

## Context

- Hiện thông tin "cần làm gì" nằm rải: `RequestBacklogBanner` (một câu dài), `TypeConfirmationCard`, `ClarificationPanel`, thanh duyệt trong từng tab.
- Dữ liệu: `request.status`, `ExecutionSummary.needsYou`, `failed`, `running`, `Approval` pending (từ `useApprovals`), Clarification mở.
- Thứ tự ưu tiên (CR-037 2.7): task lỗi, chờ người, đang chạy, đang chờ, hoàn tất.

## Việc cần làm

1. `next-action-rules.ts` (thuần TS): `pickNextAction({request, summary, approvals, clarification, flow})` trả `{kind: 'failed'|'needs_you'|'running'|'waiting'|'done'|'none', title, detail, target: {tab, ref?}, primary?: {labelKey, action}}`. Quy tắc: `request_backlog` do task lỗi → `failed`; có `needsYou` hoặc Approval/Clarification chờ người xem → `needs_you` (mục đầu tiên theo hạn); `executing` có `running` → `running` ("Agent is working on `#TG-N`, 4 of 25 minutes. Nothing needs you right now."); Phase chờ phụ thuộc → `waiting`; `completed` → `done`.
2. `NextActionBar`: một dải dưới thang tiến độ ở mọi tab, icon + tiêu đề + chi tiết một dòng + nút chính dẫn tới đúng tab hoặc thực hiện ngay (ví dụ "Answer", "Rerun", "Approve"). Không hiện khi không có gì (hoặc "Nothing needs you right now" khi đang chạy).
3. Tab Execution có chấm trạng thái (và `aria-label`) khi có mục `needs_you` hoặc `failed`; tab Plan có chấm khi chờ duyệt.
4. `RequestBacklogBanner`: khi lý do là task lỗi thì không hiển thị câu dài, nhường cho `ExecutionFailureCard` và `NextActionBar`; các lý do khác giữ nguyên.
5. `aria-live="polite"` cho dải khi nội dung đổi; tiêu điểm không bị cướp.

## Kiểm thử

- `pickNextAction` table-driven: các tổ hợp ưu tiên (lỗi và chờ người cùng lúc → lỗi), `awaiting_information`, `hotfix` chờ `pre_deploy`, `completed`.
- #1242 (`request_backlog`, task lỗi 2/2): dải `failed` với nút "Reopen and rerun".
- Component: nút chính dẫn đúng tab; dải ẩn khi `none`.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/request/NextAction src/renderer/src/components/request/next-action` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Dải chọn đúng một mục ưu tiên cao nhất và dẫn tới đúng chỗ.
- [x] Tab có chấm khi có việc cần người hoặc lỗi.
- [x] `RequestBacklogBanner` không còn câu lỗi dài trùng với thẻ lỗi.
- [x] i18n năm locale; truy cập (`aria-live`) đúng.

## Rủi ro và lưu ý

- Quy tắc ưu tiên chưa qua người dùng thật; giữ trong một hàm thuần để đổi dễ.
- Dải có thể che mục thứ hai; "Needs you" ở tab Execution liệt kê đủ.

## Ghi chú triển khai

Hoàn thành 2026-10-16; đã xem trên giao diện web thật.

- `next-action-rules.ts` (`pickNextAction`, thuần TS, test bảng): thứ tự ưu tiên task lỗi, chờ người, đang chạy, đang chờ, hoàn tất; Approval chỉ tính là "chờ bạn" khi `needsYou` của backend liệt kê (đã lọc theo quyền), nếu không chỉ là "chờ người khác". `hotfix` chờ `pre_deploy` trỏ tới tab Execution (không có tab Plan).
- `NextActionBar.tsx`: một dải dưới thang tiến độ ở mọi tab, `aria-live="polite"`, không cướp tiêu điểm; nút chính "Reopen and rerun", "Answer" hoặc "Review". Trên #1242: "“…” failed 2/2 times" + "Reopen and rerun".
- Chấm trên tab Execution/Plan (`role="img"` có `aria-label`) khi có việc cần người hoặc lỗi.
- Lệch so với bản soạn: `RequestBacklogBanner` không sửa; `RequestDetailPane` ẩn nó khi Request về backlog do task lỗi (đã có `ExecutionFailureCard` và dải), các lý do khác giữ nguyên. Nút "Answer" xuất hiện ở cả header và dải, nên test pane dùng `findAllByRole`.
