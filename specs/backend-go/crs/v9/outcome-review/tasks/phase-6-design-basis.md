# Phase 6 — Cơ sở thiết kế đơn điệu, nguồn flow tất định, lát cắt kiến trúc (CR-OR-040)

**Solution:** BE-OR-SOL-005 §2 · **Phụ thuộc:** Phase 2 (context run) đã có · **Service:** `request-service`, `api-gateway`, `cmd/orca-cli` · **Trạng thái:** 📝 chưa bắt đầu (2026-10-21)

Quy ước: mỗi task có `Files` và `Verify`. Không thêm `max-lines` disable; file mới đặt tên theo khái niệm (không `helpers`/`utils`). Mọi thay đổi `_contract.tmpl` phải cập nhật golden bằng `go test ./services/request-service/internal/usecase -run TestPromptGolden -update` và commit cùng.

## Spike

- [ ] **BE-OR-T601** Đọc tài liệu thiết kế qua `ReadFile`: lấy đường dẫn từ khoá `docSha256` mà agent khai; thử với `vnp-qa-platform/docs/SAD.md` (80.214 byte, 10 mermaid, 3 sequence); thử SAD > 256 KiB và SAD không ở `docs/`.
  Files: ghi kết luận vào `solutions/BE-OR-SOL-005-…md` §8. Verify: bảng kết quả (đường dẫn, kích thước, số khối) cho ít nhất 2 project; quyết O5.
- [ ] **BE-OR-T602** Chọn cách lưu **phiên bản** thiết kế: bảng `outcome_review_design_versions` hay thêm cột vào `outcome_review_contexts`. Đã biết `Put` ghi đè khi trùng `digest`; còn kiểm `docSha256` agent khai có ổn định giữa các lần trích không.
  Files: `adapter/postgres/outcome_review_context_repository.go` (đọc), kết luận trong SOL-005 §8. Verify: ghi quyết định kèm lý do; nếu chọn bảng mới, phác migration.

## Cơ sở thiết kế đơn điệu (R1–R3)

- [ ] **BE-OR-T603** `DesignShrank(old, new)` và quy tắc giữ phiên bản cũ khi cùng `(snapshot, docSha256)`.
  Files: `domain/outcome_review_design_basis.go` (mới), `usecase/outcome_review_design_lock.go`. Verify: bảng ca (nhỏ hơn mỗi chiều: container/cạnh/flow; bằng; lớn hơn; snapshot đổi; docSha256 đổi); ba fixture #2318 (05:41, 06:43, 07:14): 06:43 → 07:14 phải bị giữ lại, cảnh báo `design_shrank`.
- [ ] **BE-OR-T604** Tách `RegenScope` thành `artifact | code | design | all`; giữ `context` là bí danh `all` một phiên bản; tách `Force` thành `IgnoreCache` và `ReextractDesign`.
  Files: `domain/outcome_review_regenerate_policy.go`, `usecase/outcome_review_context_run.go`, `usecase/outcome_review_regenerate.go`, `proto/orca/request/v1/*.proto` + code sinh, `api-gateway/.../channels_request_review.go`, `cmd/orca-cli`. Verify: `code` ⇒ agent giả không nhận lời gọi `kg_*` và `design_digest` không đổi; `design` ⇒ trích lại; test parity kênh CLI/MCP; `go test ./services/api-gateway/...`.
- [ ] **BE-OR-T605** Con trỏ "thiết kế đang dùng" và RPC `request.review.designBasis.get|adopt` (adopt ghi audit, cần vai trò duyệt).
  Files: `usecase/outcome_review_design_lock.go`, `usecase/outcome_review_design_adopt.go` (mới), gateway, OPA `agent_rpcs`, `excluded_channels.yaml`. Verify: người thường 403; agent không gọi được `adopt`; audit không chứa nội dung thiết kế.
- [ ] **BE-OR-T606** `model.designBasis{version, digest, snapshot, counts, extractedAt, warnings}` trong mô hình review và proto.
  Files: `domain/outcome_review_model.go`, `usecase/outcome_review_context_apply.go`, proto + gateway. Verify: review mới có `designBasis`; model cũ không có vẫn đọc được (test tương thích).

## Nguồn flow tất định (R4–R5)

- [ ] **BE-OR-T607** Mở rộng cổng `RepoFileStat` thành `RepoFiles{Stat, ReadFile}`; truyền adapter `NewProposalWorkspace` (đã có `ReadFile`).
  Files: `usecase/ports_workspace.go`, `cmd/server/wire_outcome_review.go`. Verify: test adapter giả; runner không đọc quá 256 KiB.
- [ ] **BE-OR-T608** `DiscoverDesignBlocks` liệt kê khối mermaid của tài liệu theo mục bằng `ParseMermaid`.
  Files: `domain/outcome_review_design_blocks.go` (mới). Verify: SAD thật của `vnp-qa-platform` ⇒ 3 `sequenceDiagram` (6.1, 8.1, 8.2) và 10 khối tổng; khối lạ (C4Context) bị bỏ kèm cảnh báo, không panic.
- [ ] **BE-OR-T609** Bổ sung khối agent không chép: ánh xạ participant qua alias container; cảnh báo `design_blocks_added:<mục>`, `design_flow_participant_unmapped:<mục>`; tài liệu không đọc được ⇒ `design_docs_unreadable`.
  Files: `domain/outcome_review_design.go` (`mergeFlows`, `readBlocks`). Verify: agent chép 1 trong 3 ⇒ đủ 3; participant lạ ⇒ cảnh báo, không mất flow khác.
- [ ] **BE-OR-T610** `DesignFlow.FlowRef` và `Origin (mermaid|text)`; flow `text` bắt buộc bằng chứng `source` (doc, section, line); flow mermaid dùng `FlowRef = mục SAD`, giữ `Name` đọc được.
  Files: `domain/outcome_review_design.go`, `domain/outcome_review_context.go` (schema), `prompts/outcome_review_context/_contract.tmpl`. Verify: flow `text` không bằng chứng ⇒ vi phạm; có bằng chứng ⇒ vào thiết kế `origin=text`; golden prompt cập nhật.
- [ ] **BE-OR-T611** Đọc được khoá cũ (`schema 1`, không `FlowRef`): suy `FlowRef` từ `Evidence.Section`; chỉ ghi lock mới ở định dạng mới (O4).
  Files: `usecase/outcome_review_design_lock.go`. Verify: lock cũ nạp được; digest cũ không bị coi là "khác prompt".

## Lát cắt kiến trúc (R6)

- [ ] **BE-OR-T612** `ScopeSlice(design, facts, requirements)` cho điểm container (3 chạm code, 2 nối yêu cầu, 1 hàng xóm một bước, 0 còn lại).
  Files: `domain/outcome_review_scope_slice.go` (mới). Verify: #2318 ⇒ `ms-integration`, `ms-bug-management`, MySQL, Kafka, Google Workspace có điểm ≥ 3 hoặc hàng xóm; container không liên quan điểm 0.
- [ ] **BE-OR-T613** `capDiagram` và `BuildDesignDiagrams` dùng điểm lát cắt thay vì thứ tự ID; phần dư gom "+N" kèm danh sách id cho drilldown.
  Files: `domain/outcome_review_diagram_model.go` (`capDiagram`, `BuildDesignDiagrams`). Verify: hồi quy test hiện có; ca mới: 17 container, hình thiết kế giữ đúng các container có điểm cao; "+N" mang danh sách đủ.

## Quan sát và tài liệu

- [ ] **BE-OR-T614** Chỉ số và log: `request_review_design_shrank_total`, `request_review_design_blocks_added_total`, `request_review_design_docs_unreadable_total`; log `audit=true` cho `adopt`.
  Files: `usecase/review_observer.go` (hoặc file observer hiện có), `domain/outcome_review_*`. Verify: nhãn thuộc tập đóng (test: giá trị lạ → `other`).
- [ ] **BE-OR-T615** Cập nhật tài liệu: `docs/crs/v9/outcome-review-fidelity/README.md` (đánh dấu E1–E5 đã sửa), ghi kết quả vào checklist này theo mẫu "Kết quả (ngày): …".
  Verify: mỗi task đã xong có dòng "Kết quả" nêu file, test, số liệu.

## Cổng của pha

- [ ] `go test ./services/request-service/... ./services/api-gateway/...` xanh; golden prompt cập nhật.
- [ ] Chạy lại thật trên BL-E05: regenerate `code` giữ `design_digest`, 0 lời gọi `kg_*` (AC2 của CR-OR-040).
