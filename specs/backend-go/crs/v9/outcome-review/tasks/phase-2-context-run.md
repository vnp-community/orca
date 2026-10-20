# Phase 2 — Context run: KG, thiết kế khoá, diff mức hệ thống (CR-OR-002 + AM)

**Solution:** BE-OR-SOL-002 · **Phụ thuộc:** Phase 1 (và spike BE-OR-S5) · **Service:** `request-service`

## Hợp đồng và kiểm

- [ ] **BE-OR-T201** Lược đồ `outcome-review-context/1` (Go struct + JSON Schema) gồm `kg`, `design.{containers,edges,flows,sourceBlocks,docSha256}`, `items[]`, `diagrams[]`, phần tử đọc có `reviewState`.
  Files: `internal/domain/outcome_review_context.go`, `schemas/outcome-review-context-1.json`. Verify: đối chiếu với kết quả spike S5.
- [ ] **BE-OR-T202** Kiểm bằng chứng: kinds `kg|source|file|rpc|channel|migration|check`; id KG theo regex chấp nhận `SVC-BUG-09`, `CT-02`, `ENT-02:attribute:email`; `kgId` ∈ phần tử agent khai đã đọc; `file` ∈ danh mục đổi hoặc tồn tại ở `baseRef`.
  Files: `domain/outcome_review_evidence.go`. Verify: bảng ca tốt/xấu; thiếu bằng chứng → `unverified`, không vào `withEvidence`.
- [ ] **BE-OR-T203** Mô hình sơ đồ `level=container`: id nút (`kgId` | `sad:<mã>` | `code:<kebab>`), `source`, `kind`/`tech` theo văn bản (AM-5), từ chối nút component và tin nhắn tự gọi, gộp > 12 nút.
  Files: `domain/outcome_review_diagram_model.go`. Verify: fixture nút `SVC-BUG-09` bị từ chối ở mức container hoặc gom thành huy hiệu của `ms-bug-management`.
- [ ] **BE-OR-T204** Suy trạng thái cạnh/tin nhắn (`added|removed|same|deviation|missing`) từ `inBase/inHead/inDesign`; `advisory` khi dựa trên phần tử `needs_review` (AM-3); `unmapped` chỉ khi `source=code` (AM-2).
  Files: `domain/outcome_review_edge_state.go`. Verify: agent không đặt `state` trực tiếp được; KG vắng → không có `deviation/missing`.
- [ ] **BE-OR-T205** Parser mermaid xác định cho `flowchart` và `sequenceDiagram` (AM-4).
  Files: `domain/outcome_review_sad_mermaid.go`. Verify: fixture SAD 5.5 (flowchart container), 8.1 (sequence), 8.4 (flowchart) của `vnp-qa-platform` → cạnh/tin nhắn viết tay; khối lạ → bỏ qua kèm cảnh báo, không panic.
- [ ] **BE-OR-T206** Hợp nhất: khi agent và parser lệch, ưu tiên parser, ghi `design_edge_mismatch`.
  Verify: test lệch hai chiều.

## Khoá thiết kế và cache

- [ ] **BE-OR-T210** Bảng/cột `design_digest`, lưu bản `design.*` đã chấp nhận theo `(kg.snapshot, docSha256, promptVersion, schemaVersion)`; tái dùng ở pha `delivered`.
  Files: migration bổ sung (hoặc cột trong `outcome_reviews.model`) + `usecase/outcome_review_design_lock.go`. Verify: `delivered` sau `design` còn hạn không gọi `kg_*` (đếm lời gọi của agent giả).
- [ ] **BE-OR-T211** `context_digest`, TTL `REQUEST_REVIEW_CONTEXT_TTL`, trúng cache không gọi agent.
  Verify: lần hai = 0 lượt `agent.execPrompt`.

## Lượt chạy

- [ ] **BE-OR-T220** Lời nhắc có `promptVersion`, rào `<untrusted-input>`, hai chế độ `design|delivered`, cấm công cụ ghi và `changeset_propose`.
  Files: `usecase/outcome_review_prompt.go`. Verify: test injection (AC/tên tệp/văn bản SAD chứa chỉ thị) với agent giả.
- [ ] **BE-OR-T221** Chạy qua đường `agent.execPrompt` sẵn có với hồ sơ quyền chỉ-đọc; giới hạn lượt/tệp/thời gian; thư mục làm việc trong `AGENT_WORK_DIR`; một lần thử lại kèm danh sách vi phạm; mã `REQUEST_REVIEW_RESULT_INVALID`.
  Files: `usecase/outcome_review_context_run.go`. Verify: các ca agent giả của SOL-002 §3.
- [ ] **BE-OR-T222** Khả năng dev server `reviewContext` (= `knowledgeHub` + `permissionProfile` + skill `tdd-knowledge`); hiển thị trong `orca engine show --check`; `REQUEST_REVIEW_SKILL_MISSING`.
  Files: `domain/dev_server_capability.go` (+ test), `cmd/orca-cli` (engine check). Verify: thiếu skill → mã lỗi + gợi ý cài.
- [ ] **BE-OR-T223** KG vắng: không chạy mặc định, `allow_code_only` có xác nhận chi phí, mọi mục `unverified`, `designChecked=false`.
  Verify: tiêu chí §4 của CR-OR-002.
- [ ] **BE-OR-T224** Số liệu `verdict.architecture|flow` (đếm xác định) kèm `designChecked`, `reviewedRatio`.
  Verify: golden JSON.

## Chạy thật (cần đồng ý chi phí)

- [ ] **BE-OR-T290** Chạy `workspace_probe.py` rồi một lượt context thật `mode=design` trên Request pilot của `vnp-qa-platform`; ghi chi phí, thời gian, tỉ lệ bằng chứng xác minh được, số cạnh thiết kế trích được.
  Verify: ghi vào `evidence/phase-2-real-run.md`; chốt trần lượt/tệp.
