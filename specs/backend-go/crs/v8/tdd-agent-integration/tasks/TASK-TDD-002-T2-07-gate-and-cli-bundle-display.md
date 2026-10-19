# TASK-TDD-002-T2-07: Gate Plan và CLI hiển thị gói OpenSpec, kiểm cấu trúc, tham chiếu KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.1, 3.4 (bước 7)
**Checklist:** [T2.7](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service proto + api-gateway + orca-cli (Go)
**Files:** `proto/orca/request/v1` (GetPlanProposal, Approval), `api-gateway/internal/adapter/wscompat/channels_request_plan.go` (`planProposalView`), `httpgateway/request_routes_flow.go`, `backend-go/cmd/orca-cli/internal/command/plan_cmd.go`, `flow_render.go`, test đi kèm; frontend (ngoài phạm vi, ghi chú)
**Depends on:** T2.6
**Status:** `[ ]` TODO

---

## Context

Người duyệt gate Plan phải đọc được cái họ duyệt: gói OpenSpec, kết quả kiểm cấu trúc, và những gì agent báo về KG (snapshot, id đã đọc, chỗ thiếu, xung đột Decision/Constraint). Hiện `planProposalView` trả proposal và trạng thái run, chưa có thông tin engine.

## Việc cần làm

1. Proto: thêm vào phản hồi đề xuất Plan: `engine`, `bundle{change_id, branch, commit, files[{path,size}], tasks_md_digest}`, `checks[{code, where, message, severity}]`, `knowledge{snapshot, read[], gaps[], conflicts[], available}`; tạo lại mã proto.
2. Gateway: `planProposalView` thêm các trường; thêm route `GET /v1/requests/{id}/plan/bundle` (danh sách tệp) và `GET /v1/requests/{id}/plan/bundle/file?path=` (nội dung từ `bundle_json`, giới hạn đường dẫn dưới `openspec/changes/<id>/`, chống `..`); kênh WS tương ứng cho UI nếu có.
3. CLI: `orca plan proposal <R>` in engine, nhánh, commit, kết quả kiểm, tham chiếu KG; thêm `orca plan bundle <R> [--file <path>]` để xem danh sách hoặc nội dung tệp; `--json` cho cả hai.
4. Gate hiển thị cảnh báo khi `knowledge.available=false` hoặc có `conflicts`.
5. README orca-cli cập nhật các lệnh mới.

## Tiêu chí hoàn thành

- [ ] Có thể đọc `proposal.md`, `design.md`, `spec.md`, `tasks.md` của gói từ CLI trước khi duyệt Plan.
- [ ] Route bundle không đọc được ngoài vùng gói; đường dẫn chứa `..` hoặc tuyệt đối bị từ chối.
- [ ] Hiển thị đúng cho engine `native` (không có gói): không lỗi, không trường rỗng gây nhầm.

## Kiểm thử

- Test gateway (bảng route, quyền, đường dẫn xấu), test CLI (`httptest`), test render.

## Rủi ro và ghi chú

- Giao diện web: ngoài phạm vi task này; ghi vào nợ (T5.4).
