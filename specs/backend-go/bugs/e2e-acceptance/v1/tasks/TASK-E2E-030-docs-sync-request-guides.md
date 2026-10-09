# TASK-E2E-030: Đồng bộ tài liệu với mã

**From Solution:** SOL-E2E-006 · **Resolves:** OBS-D
**Priority:** P2 · **Service/Area:** docs
**Files:**
- docs/guides/request/README.md
- docs/features/F36-multi-server-workflow-orchestration.md
- specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md
**Depends on:** TASK-E2E-001, 008
**Status:** `[x]` XONG cho `docs/guides/request/*` và F36 (cập nhật 2026-10-09, chưa commit)

---

## Context

`guides/request` vẫn ghi Solution/Plan/Phase "chưa có"; F36 mô tả đích `project:` chưa nói `server:` và `connection:`.

## Việc cần làm

- [x] Cập nhật bảng trạng thái giai đoạn theo kết quả `orca-flows`. — _`docs/guides/request/README.md`: bảng "Trạng thái hiện thực" có cột "Trong mã" và "E2E" (test id chứng minh), liệt kê đúng 24 RPC không có handler (đối chiếu proto với `func (s *Server)` trong `internal/adapter/grpc`), sửa bảng trạng thái, ba view backlog, bảng kiểm thử; thêm lỗi đã biết (`AST-05`)_
- [x] F36: ghi ba dạng `target` và điều kiện của `project:`. — _`docs/features/F36-multi-server-workflow-orchestration.md` mục "Chọn server cho step (`target`)": `connection:`, `server:`, `project:` (cùng `fleet:tag:`, rỗng, dạng trần cũ), điều kiện `project:` (dev server đã gắn; người chạy là thành viên vì `GetProject` kiểm thành viên và execution lưu `started_by`; dev server phải đang kết nối, không rơi về chạy cục bộ), đối chiếu `serverresolver/resolver.go`, e2e `WFL-12/13`_
- [x] Ghi cách đọc/ trả lời Clarification qua API.

## Tiêu chí hoàn thành

- [x] Không còn câu "chưa có" sai. — _đã rà `docs/guides/request/*` và F36; sửa thêm các câu sai ngoài README: `approving-requests.md` (cổng `solution`/`plan`/... đều có handler), `request-and-task-backlogs.md` (`ListBacklog` và hai view task có), `runbook-request-flow.md` (hai dòng), `creating-and-classifying-requests.md`, `admin-enable-request-flow.md`, `requests-from-agents-mcp.md` (tool `request_*` có, nằm ở api-gateway). Chưa rà tài liệu ngoài hai thư mục này_

## Kiểm thử

Đối chiếu từng câu với bộ e2e.

## Kết quả thực thi

**Làm xong** cho phạm vi `docs/guides/request/*` và F36 (2026-10-09, chưa commit). Mỗi dòng trạng thái đối chiếu với mã thật (wiring ở `cmd/server/main.go`, `wire_*.go`, danh sách handler) và với test id của bộ e2e; không dựa vào tài liệu cũ.

Điều tài liệu nói là "chưa" mà thực tế có: Solution, Plan, Phase/thực thi, `ListBacklog` và hai view task, cổng duyệt `solution`/`findings`/`answer`/`plan`/`task_list`/`phase`/`pre_deploy`, tool MCP `request_*`. Điều thực sự chưa có (giữ nguyên): Impact, rủi ro, kiểm tra readiness, nguồn ngữ cảnh, evidence, `GetProjectEngineSettings`, `ExportTenantRequests`, `AiBudgetAdminService`.

Chỗ còn ghi "mã có nhưng chưa được e2e chạy" (không phải "chưa có"): chẩn đoán/findings/answer, thực thi sau khi duyệt Plan (`StartPhase`, `ReportTaskOutcome`), nội dung từng view backlog, tool MCP, cổng `phase`/`pre_deploy`.

Phát hiện khi viết: README `T2` trỏ tới `tests/request/check_request_flow_types.py` đã bị xoá khỏi cây (commit `8beb6703d`) nên `run-request-e2e.sh` hiện không chạy được; đã ghi vào bảng kiểm thử, chưa sửa script (nằm ở `backend-go/ci`).
