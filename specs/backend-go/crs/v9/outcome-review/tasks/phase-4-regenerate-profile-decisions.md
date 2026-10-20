# Phase 4 — Sinh lại, hồ sơ vẽ, quyết định, duyệt (CR-OR-012, 013, 022 backend)

**Solutions:** BE-OR-SOL-003 §2.6–2.7, BE-OR-SOL-004 §2.1 · **Phụ thuộc:** Phase 3 · **Service:** `request-service`, CLI

## Sinh lại (CR-OR-012)

- [ ] **BE-OR-T401** Chính sách: phạm vi `artifact|context|all`, hạn mức ngày và đồng thời, `client_request_id` idempotent, `expected_revision`.
  Files: `domain/outcome_review_regenerate_policy.go`. Verify: `REQUEST_REVIEW_BUDGET_EXCEEDED`, `REQUEST_REVIEW_RUN_IN_PROGRESS`, `REQUEST_REVIEW_STALE_REVISION`.
- [ ] **BE-OR-T402** Usecase sinh lại: revision mới dù khoá trùng (`origin=regenerated`, `reason` bắt buộc); `context` vẽ lại chỉ mục có `inputDigest` đổi; draw lỗi giữ revision cũ `active`.
  Files: `usecase/outcome_review_regenerate.go`. Verify: `scope=artifact` tạo đúng 1 revision; `scope=context` KG đổi/code không đổi → đếm lượt vẽ.
- [ ] **BE-OR-T403** `RestoreReviewArtifactRevision` (không tốn AI), so sánh mô hình sơ đồ hai revision (xác định).
  Files: `domain/outcome_review_model_diff.go`. Verify: `active` đổi đúng và chỉ một.
- [ ] **BE-OR-T404** `EstimateOutcomeReview` nhận `scope/sections`, trả số lượt/sơ đồ, `lastActualCost`; "chưa có số liệu" khi chưa đo.
  Verify: Estimate khớp số lượt chạy thật; không tốn AI.
- [ ] **BE-OR-T405** Xử lý quyết định mồ côi và review `approved` quay về `ready` khi xuất hiện mục `needsDecision` mới.
  Verify: tiêu chí §4 của CR-OR-012.

## Hồ sơ vẽ (CR-OR-013, P1)

- [ ] **BE-OR-T410** Lược đồ `review-drawing-profile/1`, hợp nhất `project → tenant → default`, `level` khoá `container`, `variants`, từ chối khoá lạ/hex/`variant=full`/`maxNodes>20`.
  Files: `domain/review_drawing_profile.go`. Verify: bảng ca kiểm hợp lệ với đường dẫn trường trong lỗi.
- [ ] **BE-OR-T411** Repository + RPC `Get/SaveReviewDrawingProfile` (khoá lạc quan), audit `request.review.profile.save` (không ghi nội dung).
  Files: `adapter/{postgres,mysql}/review_drawing_profile.go`, `usecase/review_drawing_profile.go`. Verify: hai người lưu cùng lúc → người sau lỗi phiên bản.
- [ ] **BE-OR-T412** `preset_digest` theo mục từ hồ sơ hiệu lực; lưu hồ sơ không vẽ lại.
  Verify: đổi `flow` chỉ đổi digest của `flow`.
- [ ] **BE-OR-T413** CLI `orca review profile show|set|reset`.
  Files: `backend-go/cmd/orca-cli/internal/command/review_profile_cmd.go`. Verify: kết quả khớp RPC.

## Quyết định và duyệt (CR-OR-022 backend)

- [ ] **BE-OR-T420** Migration `outcome_review_decisions`; repository hai dialect; một dòng hiệu lực mỗi `(review, item_key)`, lịch sử giữ.
  Verify: lên/xuống sạch; truy vấn dòng hiệu lực.
- [ ] **BE-OR-T421** `DecideOutcomeReviewItem`: `note` bắt buộc với `accept_exception/dismiss`, `dismiss` không áp lên `origin=fact`, mục `advisory` không cần quyết định, khoá lạc quan, audit không ghi `note`.
  Files: `usecase/outcome_review_decide.go`, `domain/outcome_review_decision.go`. Verify: bảng quy tắc.
- [ ] **BE-OR-T422** `ApproveOutcomeReview`: `REQUEST_REVIEW_DECISIONS_OPEN`, `STALE_REVISION`; pha `design` → `NOT_READY`; không đổi `RequestStatus`.
  Files: `usecase/outcome_review_approve.go`. Verify: gọi thẳng API bỏ qua UI vẫn bị chặn.
- [ ] **BE-OR-T423** Liên kết `send_back` ↔ Request con (`child_request_id`), cách chốt sau khi đọc `request_child_rules.go`/`request_link.go`.
  Verify: tạo con từ Request `completed` thành công hoặc ghi phương án thay (liên kết `relates`).

## Hoàn tất

- [ ] **BE-OR-T499** Bộ kiểm chung + `-race` + hai dialect; ghi kết quả.
