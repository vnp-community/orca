# TASK-TDD-002-T2-05: Kiểm cấu trúc gói OpenSpec sau run và thử lại

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 5)
**Checklist:** [T2.5](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** request-service (Go, usecase thuần)
**Files:** `internal/usecase/openspec_bundle_check.go` (mới) + test, `internal/usecase/run_plan_generation.go`, `internal/usecase/analysis_settings.go`, `internal/usecase/ports_workspace.go` (thêm `List`/`Stat` nếu thiếu)
**Depends on:** T1.5, T2.3, T2.4
**Status:** `[ ]` TODO

---

## Context

Pipeline `tdd-agent change` cho thấy gói có thể qua luật của công cụ mà vẫn vô dụng (một task chung chung, kịch bản cũ còn lại). Hướng mới thay các cổng đó bằng kiểm tra cấu trúc của chính Orca, chạy sau mỗi lần agent ghi gói và dùng kết quả để thử lại một lần. Kiểm tra không truy vấn KG; chỉ đọc file trong worktree và chạy `openspec validate`.
Hợp đồng kiểm ở Phụ lục B của checklist.

## Việc cần làm

1. Định nghĩa `BundleFiles{ChangeID, Proposal, Design string; Specs map[string]string; TasksMD string}` và `BundleViolation{Code, Where, Message string}`; hàm thuần `CheckBundle(files, request, size, limits) []BundleViolation`.
2. Số task theo cỡ: mặc định S 1–5, M 3–12, L 6–30, cấu hình qua `AnalysisSettings` (mã `REQUEST_SPEC_TASK_COUNT`).
3. Bốn dòng mô tả của mỗi task (Serves, Code, Verify, Done when) bằng biểu thức chính quy cố định (mã `REQUEST_SPEC_TASK_INCOMPLETE`); dòng `Code:` đúng dạng `modify|create` + `path::symbol`.
4. Đích mã (`REQUEST_SPEC_CODE_TARGET_INVALID`): `modify` ⇒ tệp phải có thật; `create` ⇒ thư mục cha có thật và tệp chưa có; kiểm qua `ProposalWorkspace` (`ReadFile`/`Stat`/`List`). Kiểm tên symbol bằng tìm chuỗi trong tệp chỉ là cảnh báo.
5. AC ↔ scenario (`REQUEST_SPEC_AC_NOT_COVERED`): mỗi tiêu chí chấp nhận đang hoạt động của Request (`domain.ContentFromRequest`) phải có một `#### Scenario:` có tên **bắt đầu bằng id AC** (quy ước ghi vào skill); `ac=` trong `tasks.md` chỉ dùng id có trong Request (`REQUEST_SPEC_AC_UNKNOWN`).
6. `openspec validate <id> --strict --json` qua `DevServerExecutor.Exec` (`cwd` = worktree); lỗi ⇒ `REQUEST_SPEC_VALIDATION_FAILED` kèm tóm tắt (cắt gọn).
7. Nối vào `RunPlanGeneration`: tập hợp `Violation` của parser (`TASKSMD_*`) và `BundleViolation`; nếu còn lỗi ⇒ một lần thử lại với danh sách (tối đa 20 dòng) trong `RetryNote`; lần hai vẫn lỗi ⇒ `failed` với mã đầu tiên và tóm tắt, giữ worktree.
8. Khối kết thúc thiếu (`REQUEST_SPEC_REPORT_MISSING`) chỉ cảnh báo, không chặn.

## Tiêu chí hoàn thành

- [ ] Mỗi mã lỗi có test (một ví dụ gây lỗi, một ví dụ đạt).
- [ ] Gói mẫu tốt từ T1.5 đạt toàn bộ; gói mẫu xấu (một task, còn kịch bản 501, thiếu AC, đích mã bịa) bị từ chối đúng mã.
- [ ] Lần thử lại dùng danh sách lỗi; hai lần lỗi thì run `failed`.

## Kiểm thử

- Test bảng (table-driven) cho `CheckBundle` với fixture từ `evidence/` của T1.5.
- Test tích hợp `RunPlanGeneration` với agent giả: lần một lỗi, lần hai đạt.

## Rủi ro và ghi chú

- Giữ kiểm tra thuần để chạy nhanh và dùng chung với script spike.
- Khoảng số task theo cỡ chỉ là đề xuất khởi đầu; điều chỉnh theo số liệu T1.5 và T4.7.
