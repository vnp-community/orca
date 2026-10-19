# FE-REQ-TASK-037-08: Sửa nhãn "Blocked by" và cảnh báo đỏ ở tab Result của task thuộc Request

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 1 (đính chính), CR-037 mục 2.10
**Priority:** P1
**Area:** frontend / task
**File:** `frontend/src/renderer/src/components/task/TaskDetail.tsx` (sửa, dòng ~389), `components/request/execution/ExecutionResultPanel.tsx` (sửa nếu cần), `components/task/*.test.tsx`, `components/request/execution/execution-components.test.tsx`, `i18n/locales/*.json`
**Depends on:** không (làm được ngay); xác nhận bước 1 trước khi sửa bước 3
**Status:** [x] DONE

---

## Context

- `TaskDetail.tsx` hiện "← Blocked by:" liệt kê mọi phụ thuộc dù đã xong (trên #1242: 3 task đã xong).
- Tab Result của task thuộc Request trên #1242 hiện chữ đỏ "The result is not in the expected format" rồi log thô của agent, trong khi `ExecutionResultPanel` có `AgentOutputView` trung tính ("This run did not report a structured result...") đúng cho "Tasks of a Request report no structured block".
- Chưa rõ nguyên nhân: bản đang chạy cũ hơn mã nguồn, hay `TaskDetail` không truyền `requestId`/`execution` cho panel nên rơi vào nhánh cũ.

## Việc cần làm

1. **Điều tra trước (không đoán):** xác định nhánh nào hiển thị cảnh báo đỏ (tìm chuỗi `not in the expected format` trong `renderer/src`) và vì sao task của Request không vào `AgentOutputView`. Kiểm tra bản build đang chạy (so ngày build với commit). Ghi kết luận vào "Ghi chú triển khai".
2. Nhãn phụ thuộc: đổi "← Blocked by:" thành "Depends on" kèm đếm "3 of 3 done" và biểu tượng xong cho từng mục; chỉ dùng "Blocked by" khi còn phụ thuộc chưa xong (và liệt kê đúng các mục chưa xong). Khoá i18n mới, đủ 5 locale.
3. Tab Result: với task có `requestId` mà không có khối kết quả có cấu trúc, hiển thị `AgentOutputView` trung tính; cảnh báo đỏ chỉ cho trường hợp hợp đồng có cấu trúc mà không phân tích được (đúng nghĩa "sai định dạng"). Nếu nguyên nhân là props thiếu, truyền `requestId` từ `TaskDetail`.
4. Không đổi hành vi task không thuộc Request.

## Kiểm thử

- `TaskDetail`: phụ thuộc đã xong hiện "Depends on 3 of 3 done"; còn mục chưa xong hiện "Blocked by" với đúng mục đó.
- `ExecutionResultPanel`/`TaskDetail`: task thuộc Request không có khối cấu trúc hiển thị `AgentOutputView`, không có chữ đỏ; kết quả hợp đồng sai schema vẫn hiện cảnh báo đỏ.
- Hồi quy các test `execution-components.test.tsx`, `execution-agent-output.test.tsx`.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/task src/renderer/src/components/request/execution` (đã chạy, xem ghi chú triển khai).

## Tiêu chí hoàn thành

- [x] Có ghi chú nguyên nhân cảnh báo đỏ (bản cũ hay thiếu props).
- [x] Không còn "Blocked by" khi mọi phụ thuộc đã xong.
- [x] Task thuộc Request không hiện cảnh báo "sai định dạng" khi vốn không có khối cấu trúc.
- [x] i18n năm locale (037-09 đã thêm khoá vào cả năm locale).

## Rủi ro và lưu ý

- Nếu nguyên nhân là bản build cũ thì bước 3 có thể không cần sửa mã; chỉ thêm test hồi quy.
- Đừng làm ẩn lỗi thật: kết quả hợp đồng có cấu trúc mà sai schema vẫn phải hiển thị cảnh báo.

## Ghi chú triển khai

- **Nguyên nhân (bước 1):** không phải bản build cũ hay thiếu props: `TaskDetail` đã truyền `requestId` và `execution` cho `ExecutionResultPanel`, và nhánh `source === 'last_execution_output'` đã trung tính. Cảnh báo đỏ đến từ nhánh khác: một lần chạy theo hợp đồng ghi `ExecutionRecord` với `parseStatus: 'missing'` (agent in văn bản tự do, không có khối `ORCA_RESULT_BEGIN/END`), gateway trả `source: 'execution_record'`; panel coi mọi `parseStatus !== 'ok'` là "sai định dạng" nên hiện chữ đỏ. `missing` (không có khối) khác `invalid` (có khối nhưng sai schema).
- `ExecutionResultPanel.tsx`: `parseStatus === 'missing'` cũng vào `AgentOutputView` trung tính; chỉ `invalid` còn `execution-malformed` đỏ.
- `TaskDependenciesSection.tsx` (mới, tách khỏi `TaskDetail.tsx`) + `task-dependency-progress.ts`: "Depends on  N of M done" kèm biểu tượng `CircleCheck` cho mục xong; "← Blocked by:" chỉ khi còn mục chưa xong và chỉ liệt kê các mục đó. Trạng thái không rõ được coi là chưa xong.
- Khoá i18n mới (mặc định tiếng Anh), tiền tố `auto.components.task.TaskDetail.`: `dependencies` "Dependencies", `blockedBy` "Blocked by:", `dependsOn` "Depends on", `dependsOnProgress` "{{done}} of {{total}} done", `noDependencies` "No dependencies", `dependenciesFailed` "Failed to load dependencies".
- Test: `execution-agent-output.test.tsx` (missing trung tính, invalid vẫn đỏ), `TaskDetail-request-artifacts.test.tsx` (record `missing` trong Result tab), `TaskDetail.test.tsx` (3 of 3 done; chỉ liệt kê mục chưa xong), `task-dependency-progress.test.ts`.
- Đã chạy: `npx vitest run --config config/vitest.config.ts src/renderer/src/components/task/ src/renderer/src/components/request/execution`: 33 file, 254 test xanh. `tsc` không có lỗi ở file đã sửa. Lỗi có sẵn, không liên quan: `task-page-*.test.ts` và `TaskSourceBadge.test.tsx` (thiếu gói `jsdom`).
