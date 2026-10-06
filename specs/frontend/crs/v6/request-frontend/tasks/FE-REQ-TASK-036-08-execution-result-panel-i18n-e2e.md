# FE-REQ-TASK-036-08: `ExecutionResultPanel`, i18n 5 locale, e2e và tài liệu

**From Solution:** [FE-REQ-SOL-036](../solutions/FE-REQ-SOL-036-clarification-decision-readiness-impact-ui.md) mục 2.8, 2.10, 5
**Priority:** P1
**Area:** frontend / request / execution / i18n / tests
**File:** `frontend/src/renderer/src/components/request/execution/{ExecutionResultPanel,ExecutionChecksTable}.tsx`, `execution-result-comparison.ts` (mới); `components/task/TaskDetail.tsx` (sửa: tab "Kết quả"); `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa); `frontend/src/renderer/src/i18n/request-artifact-locale-coverage.test.ts` (mới); `tests/e2e/request-clarification-risk.spec.ts` (mới); `docs/ui/pages/requests.md` (sửa); test cùng tên
**Depends on:** FE-REQ-TASK-036-02 (`useExecutionResult`), 036-03 đến 036-07 (nguồn khoá i18n), FE-REQ-TASK-019-07 (`request-locale-coverage.test.ts`, mẫu)
**Status:** [ ] TODO

## Context

- CR-029 2.5 và 2.6 (đã đọc): bản ghi `task_execution_records` (`parse_status ok|missing|invalid`, `result`, `changes`, `stdout_tail` 16 KB cuối); kết quả `{schema_version, status: done|blocked|failed|needs_info, summary, files_changed[], checks_run[{id, exit}], outputs, questions[], notes}`; `VerifyExecution` chạy lại Check bằng `agent.exec`, kiểm phạm vi bằng `git diff --name-only`, quét bí mật (CR-REQ-035, công cụ chưa chọn), ghi `ExecutionVerdict {status: passed|failed, findings[]}` với mã `CHECK_MISMATCH`, `SCOPE_VIOLATION`; `failure_class` ∈ `retryable|needs_info|spec_defect|env_defect|agent_defect`. "Orca không tin lời agent": hai cột "Agent báo" và "Orca chạy lại".
- **Chưa có kênh WS cho UI**: `ListExecutionRecords` là RPC nội bộ giữa service; kênh `execution.get {taskId, latestOnly}` là đề xuất (SOL-036 câu hỏi mở 1). Khi chưa có (`method_not_found`) panel hiện `stdout` cũ (như hiện nay) hoặc ẩn tab.
- `TaskDetail.tsx` (đã đọc ở SOL-021): không nhận props, đọc `activeTaskId` từ store; có `Sheet`. Task cũ trước CR-029 chỉ có `stdout` 8 KB cắt; không lỗi.
- `Failure.class` định tuyến bằng lời (CR-036 2.8): "Thử lại tối đa N lần", "Trả về bước Plan"; N nếu backend trả.
- i18n: khoá đọc theo tên, tiền tố `auto.components.request.{clarification,decision,readiness,impact,execution}.`, `RiskLevel.*` dùng chung `auto.components.graph.RiskLevel.*` (SOL-032). Mẫu test: `task-jira-link-locale-coverage.test.ts` (đã đọc: danh sách khoá + `lookup` trên `en, es, ja, ko, zh`).
- Lệnh có sẵn trong root `package.json`: `verify:localization-catalog`, `verify:localization-coverage` (nằm trong `lint`; chưa kiểm chứng chạy được trong môi trường này).

## Việc cần làm

1. `execution-result-comparison.ts` (thuần): `compareChecks(agentChecks: {id, exit}[], verdict, specChecks: {id, expect}[]): { id, agentExit: number | null, orcaPassed: boolean | null, mismatch: boolean }[]` (khớp theo `id`; `orcaPassed=null` khi `verdict` chưa có → hiển thị "Chưa chạy lại"); `classifyFiles(filesChanged, scopeViolations): { path, inScope: boolean }[]` (dựa `SCOPE_VIOLATION` trong `verdict.findings`, không tự suy scope); `describeFailureRoute(failureClass, attemptsLeft?): { messageKey, params }` ("retryable" → "Thử lại tối đa N lần", "spec_defect" → "Trả về bước Plan", "needs_info" → "Chuyển thành câu hỏi", "env_defect" → "Không tính vào số lần thử", "agent_defect" → "Thử lại một lần có nhắc định dạng"); `secretScanState(verdict): 'passed' | 'failed' | 'notRun'` (không có finding mã quét bí mật và công cụ chưa chọn → `notRun`).
2. `ExecutionChecksTable.tsx`: `Table` cột "Check", "Agent báo" (exit), "Orca chạy lại" (đạt/không/chưa chạy), "So với kỳ vọng"; hàng `mismatch` đánh dấu chữ "Không khớp" + icon (`TriangleAlert`), không chỉ màu (`risk-medium`); chú thích ngắn "Orca không dựa vào lời agent".
3. `ExecutionResultPanel.tsx`: props `{ taskId: string }`; `useExecutionResult(taskId)`; khối đầu `status` (Badge: `done` thành công token `status-success`, `blocked`/`needs_info` trung tính, `failed` `destructive` theo ý nghĩa lỗi) và `summary` (văn bản thuần); `filesChanged` có nhãn "Trong phạm vi" hoặc "Ngoài phạm vi" (nổi bật bằng icon `FileWarning` và chữ); `ExecutionChecksTable`; trạng thái quét bí mật ("Đạt", "Không đạt", "Chưa chạy", không ghi giá trị bí mật); `outputs` hiển thị tên và kiểu (không render nội dung lớn; nút "Xem" mở `Sheet` văn bản cắt 16 KB); `questions` kèm nút "Chuyển thành câu hỏi" khi `status==='needs_info'` (mở `ClarificationPanel`; Request đã ở `awaiting_information` do backend); `Failure.class` + `describeFailureRoute`.
4. Trạng thái: `status==='loading'` skeleton hoãn 200 ms; `legacy` hiện `stdout` cũ ("Kết quả chưa có cấu trúc") — lấy `stdout` từ trường task hiện có nếu có, không có thì "Không có kết quả"; `parseStatus !== 'ok'`: "Kết quả không đúng định dạng" kèm `stdoutTail` văn bản thuần rút gọn; `unsupported`: **ẩn** tab "Kết quả" (không lỗi đỏ); `forbidden`/`error` thông báo.
5. `TaskDetail` (sửa): thêm tab "Kết quả" (dùng `ui/tabs.tsx`) chỉ hiện cho task thuộc Request (`task.requestId`) và khi `useExecutionResult` không `unsupported`; giữ nguyên các tab hiện có.
6. i18n: gom mọi khoá của 036-03 đến 036-07 và panel này vào 5 locale (`en` trước; `es`, `ja`, `ko`, `zh` bản dịch thật, người dịch duyệt; không để trống, không sao chép en): `auto.components.request.clarification.*`, `.decision.*`, `.impact.*`, `.readiness.*`, `.execution.*`. Dùng nội suy (`{{count}}`, `{{name}}`, `{{stage}}`) thay vì ghép chuỗi. Không viết "an toàn", "đã xác minh" khi chưa có dữ liệu (STYLEGUIDE dòng 236).
7. `request-artifact-locale-coverage.test.ts`: hằng `REQUEST_ARTIFACT_LOCALE_KEYS` (xuất để các task khác thêm vào), kiểm mỗi khoá có chuỗi không rỗng ở cả 5 locale và các biến `{{...}}` trùng với en.
8. Chạy `pnpm run verify:localization-catalog` và `pnpm run verify:localization-coverage` (root); sửa theo yêu cầu bộ kiểm.
9. `tests/e2e/request-clarification-risk.spec.ts` (mới; mẫu `tasks-page.spec.ts`; dùng mock runtime): (a) Request `awaiting_information` hiện `ClarificationPanel`, điền và gửi một lần, trạng thái chuyển theo sự kiện; (b) chọn khác đề xuất cần lý do, phương án `high` đòi gõ tên; (c) task `env_defect` hiện badge và nút "Kết nối dev server"; (d) lệch kế hoạch hiện dải; (e) runtime không có kênh mới: không lỗi đỏ. E2E đầy đủ cần backend 028, 029, 030; chưa chạy.
10. `docs/ui/pages/requests.md`: thêm mục Clarification, Decision, Rủi ro, Readiness, Kết quả (điểm vào, trạng thái, phím tắt `Mod+Enter`). Không tạo file docs mới.
11. Kiểm tĩnh: `rg -n '#[0-9a-fA-F]{3,6}' frontend/src/renderer/src/components/request/{clarification,decision,readiness,impact,execution}` rỗng.

## Bảng tham chiếu nhanh

| Khối | Dữ liệu | Khi thiếu |
|---|---|---|
| Đầu | `status`, `summary` | "Kết quả không đúng định dạng" + `stdoutTail` |
| Files | `filesChanged` + `SCOPE_VIOLATION` | danh sách rỗng "Không có file đổi" |
| Checks | "Agent báo" so "Orca chạy lại" | "Chưa chạy lại" |
| Quét bí mật | `verdict` | "Chưa chạy" |
| Lỗi | `Failure.class` + định tuyến | không hiển thị |
| Task cũ | `stdout` cũ | "Kết quả chưa có cấu trúc" |

- Kênh WS và payload: `execution.get {taskId, latestOnly:true}` (đề xuất, chưa có trong CONTRACT).
- Phím tắt: không; `Esc` đóng `Sheet` của `outputs`.
- Khoá i18n: `auto.components.request.execution.{tab,status.done,status.blocked,status.failed,status.needs_info,files.inScope,files.outOfScope,checks.agent,checks.orca,checks.mismatch,checks.notRerun,checks.note,secret.passed,secret.failed,secret.notRun,failure.retryable,failure.needs_info,failure.spec_defect,failure.env_defect,failure.agent_defect,invalid,legacy,toQuestion}`.

## Kiểm thử

- `execution-result-comparison.test.ts`: `compareChecks` khớp/không khớp/chưa chạy lại; `classifyFiles` với `SCOPE_VIOLATION`; `describeFailureRoute` đủ 5 lớp; `secretScanState`.
- `ExecutionResultPanel.test.tsx` (mock `useExecutionResult`): tách "Agent báo" và "Orca chạy lại"; file ngoài phạm vi nổi bật; `Failure.class` có định tuyến bằng lời; kết quả sai schema hiển thị `stdoutTail` (không ẩn); task cũ (`legacy`) không lỗi; `unsupported` ẩn tab; `<script>` trong `summary` hiển thị như văn bản; "Chuyển thành câu hỏi" chỉ khi `needs_info`.
- `TaskDetail.test.tsx` (mở rộng): tab "Kết quả" hiện đúng điều kiện.
- `request-artifact-locale-coverage.test.ts`: 5 locale đủ khoá và biến nội suy.
- Phím tắt: `Mod+Enter` (ClarificationPanel, DecisionRationaleField, `RejectReasonDialog`) đúng `metaKey` Mac, `ctrlKey` Linux/Windows (đã test ở 036-03/036-04; task này chạy lại bộ test gộp).
- Chạy: `pnpm --filter orca-frontend test src/renderer/src/components/request/execution src/renderer/src/i18n/request-artifact-locale-coverage src/renderer/src/components/task/__tests__/TaskDetail`; rồi toàn bộ nhóm: `pnpm --filter orca-frontend test src/renderer/src/components/request src/renderer/src/hooks src/shared`.

## Tiêu chí hoàn thành

- [ ] `ExecutionResultPanel` tách "Agent báo" và "Orca chạy lại", đánh dấu file ngoài phạm vi, hiện `Failure.class` kèm định tuyến; kết quả sai schema hiển thị thay vì ẩn.
- [ ] Task cũ (trước CR-029) và runtime không có `execution.get` không lỗi đỏ.
- [ ] Mọi khoá `auto.components.request.{clarification,decision,readiness,impact,execution}.*` có 5 locale; test phủ khoá xanh.
- [ ] `verify:localization-catalog` và `verify:localization-coverage` chạy xanh (hoặc ghi rõ lỗi môi trường).
- [ ] Không hex trong năm thư mục `components/request/...`; không chữ "an toàn" khi chưa có kết quả.
- [ ] Phím tắt `Mod+Enter` đúng nền tảng; docs UI cập nhật.

## Rủi ro và lưu ý

- Kênh `execution.get` chưa tồn tại: nếu backend không thêm, tab "Kết quả" bị ẩn và chỉ còn `stdout` cũ; kết quả cuối vẫn đúng với CR (task cũ).
- Quét bí mật chưa chọn công cụ (CR-REQ-035): panel có thể luôn "Chưa chạy"; đừng viết "Không có bí mật".
- Bản dịch máy `es`, `ja`, `ko`, `zh` cho thuật ngữ rủi ro cần người duyệt.
- `stdoutTail` có thể chứa dữ liệu nhạy cảm: hiển thị văn bản thuần, không chuyển tiếp ra ngoài, không ghi vào `localStorage`.
- Bộ e2e cần mock runtime thận trọng; không hứa chạy được cho tới khi backend 028, 029, 030 có.
