# Phase 4 — Sinh lại, hồ sơ vẽ, quyết định, duyệt (CR-OR-012, 013, 022 backend)

**Solutions:** BE-OR-SOL-003 §2.6–2.7, BE-OR-SOL-004 §2.1 · **Phụ thuộc:** Phase 3 · **Service:** `request-service`, CLI

## Sinh lại (CR-OR-012)

- [x] **BE-OR-T401** Chính sách: phạm vi `artifact|context|all`, hạn mức ngày và đồng thời, `client_request_id` idempotent, `expected_revision`.
  Files: `domain/outcome_review_regenerate_policy.go`. Verify: `REQUEST_REVIEW_BUDGET_EXCEEDED`, `REQUEST_REVIEW_RUN_IN_PROGRESS`, `REQUEST_REVIEW_STALE_REVISION`.
- [x] **BE-OR-T402** Usecase sinh lại: revision mới dù khoá trùng (`origin=regenerated`, `reason` bắt buộc); `context` vẽ lại chỉ mục có `inputDigest` đổi; draw lỗi giữ revision cũ `active`.
  Files: `usecase/outcome_review_regenerate.go`. Verify: `scope=artifact` tạo đúng 1 revision; `scope=context` KG đổi/code không đổi → đếm lượt vẽ.
- [x] **BE-OR-T403** `RestoreReviewArtifactRevision` (không tốn AI), so sánh mô hình sơ đồ hai revision (xác định).
  Files: `domain/outcome_review_model_diff.go`. Verify: `active` đổi đúng và chỉ một.
- [x] **BE-OR-T404** `EstimateOutcomeReview` nhận `scope/sections`, trả số lượt/sơ đồ, `lastActualCost`; "chưa có số liệu" khi chưa đo.
  Verify: Estimate khớp số lượt chạy thật; không tốn AI.
- [x] **BE-OR-T405** Xử lý quyết định mồ côi và review `approved` quay về `ready` khi xuất hiện mục `needsDecision` mới.
  Verify: tiêu chí §4 của CR-OR-012.
  Kết quả (2026-10-20): `domain/outcome_review_regenerate_policy.go`: phạm vi `artifact|context|all`, lý do bắt buộc (6 giá trị), ghi chú ≤ 200, `artifact` phải nêu mục; `RegenBudget` (5/ngày/Request qua `REQUEST_REVIEW_REGEN_PER_DAY`, 2 lượt đồng thời mỗi tenant) → `REQUEST_REVIEW_BUDGET_EXCEEDED`; `client_request_id` lưu ở cột `client_key` duy nhất của `outcome_review_runs` (migration 0109) nên lặp lại chỉ chạy một lần; `expected_version` lệch → `REQUEST_REVIEW_STALE_REVISION`; đang có lượt → `REQUEST_REVIEW_RUN_IN_PROGRESS`. Hạn mức thi hành ở server (test đủ ba mã lỗi). **Lệch spec:** `expected_revision` là `version` của review (hình không có "revision" chung); hạn mức chưa hoà vào `ai_budget.go` của dự án (chưa đọc cách tính) mà chạy song song, ghi nhận là rủi ro mở.
  Kết quả (2026-10-20): `usecase/outcome_review_regenerate.go` (`Regenerate`): `artifact` tạo đúng một revision mới của mục đó (`origin=regenerated`, `reason` ghi vào hình và vào lượt chạy), mục khác không đổi và không gọi agent (đếm lượt); `context` chạy lại context rồi chỉ vẽ lại sơ đồ có `inputDigest` đổi (test: đổi một luồng → lời nhắc vẽ chỉ có đúng luồng đó); `all` vẽ lại mọi hình dù mô hình không đổi; draw lỗi giữ revision cũ `active` và ghi bản lỗi `failed` (test). Audit `request.review.regenerate` (không có ghi chú).
  Kết quả (2026-10-20): `RestoreReviewArtifactRevision` (không gọi agent, đúng một `active` sau đó, kèm so sánh mô hình), `CompareReviewArtifactRevisions`, `domain/outcome_review_model_diff.go` (`DiffModels`: nút/cạnh thêm, bớt, đổi; xác định, không phụ thuộc thứ tự). Test đủ ca; chỉ so sánh được hai hình của cùng một sơ đồ.
  Kết quả (2026-10-20): `EstimateOutcomeReview` nhận `scope/sections`, trả `RegenerationEstimate` (số lượt agent, số sơ đồ sẽ vẽ, chi phí lượt cùng loại gần nhất, `confirmRequired` khi vượt `REQUEST_REVIEW_CONFIRM_COST`). Không gọi agent (test đếm). Khi chưa có lượt nào báo chi phí: `costKnown=false`, không có số và không đòi xác nhận (test). Chi phí = tổng chi phí thực của lượt gần nhất mỗi loại, không phải dự đoán; ngưỡng xác nhận chưa đo.
  Kết quả (2026-10-20): `ApplyContext`: review `approved` giữ nguyên khi sinh lại mà mọi mục `needsDecision` đã có quyết định; có mục mới → về `ready`, bỏ thông tin duyệt, cảnh báo `new_decisions_required` (test). Quyết định gắn `item_key` sống sót qua sinh lại (cùng `review_id`); quyết định của mục đã mất được đánh `orphaned` khi đọc, không xoá (test). **Giới hạn:** review `delivered` mới do commit đổi là một dòng review mới (khoá `head_key`), quyết định của dòng cũ ở lại dòng cũ, không tự chuyển sang.

## Hồ sơ vẽ (CR-OR-013, P1)

- [x] **BE-OR-T410** Lược đồ `review-drawing-profile/1`, hợp nhất `project → tenant → default`, `level` khoá `container`, `variants`, từ chối khoá lạ/hex/`variant=full`/`maxNodes>20`.
  Files: `domain/review_drawing_profile.go`. Verify: bảng ca kiểm hợp lệ với đường dẫn trường trong lỗi.
- [x] **BE-OR-T411** Repository + RPC `Get/SaveReviewDrawingProfile` (khoá lạc quan), audit `request.review.profile.save` (không ghi nội dung).
  Files: `adapter/{postgres,mysql}/review_drawing_profile.go`, `usecase/review_drawing_profile.go`. Verify: hai người lưu cùng lúc → người sau lỗi phiên bản.
- [x] **BE-OR-T412** `preset_digest` theo mục từ hồ sơ hiệu lực; lưu hồ sơ không vẽ lại.
  Verify: đổi `flow` chỉ đổi digest của `flow`.
- [x] **BE-OR-T413** CLI `orca review profile show|set|reset`.
  Files: `backend-go/cmd/orca-cli/internal/command/review_profile_cmd.go`. Verify: kết quả khớp RPC.
  Kết quả (2026-10-20): đã xong từ Phase 3 (`domain/review_drawing_profile.go`, 21 ca từ chối kèm đường dẫn, hợp nhất tenant→project, nguồn từng trường).
  Kết quả (2026-10-20): migration 0109 `review_drawing_profiles` (hai dialect), `adapter/{postgres,mysql}/review_drawing_profile_repository.go` + hợp đồng `RunReviewProfileContract` (khoá lạc quan, hai writer tạo đồng thời chỉ một thắng, tách lớp tenant/project, cô lập tenant) PASS trên Postgres 16 và MySQL 8 thật; `usecase/review_drawing_profile.go` (`Get/Save/Reset/Resolve`, lớp lưu không còn hợp lệ bị bỏ qua có log chứ không làm hỏng việc vẽ); RPC `Get/Save/ResetReviewDrawingProfile` (đọc: người đã xác thực; ghi: nhóm `admin`); audit `request.review.profile.save` chỉ chứa digest cũ/mới (test). Đã nối `Resolve` vào `Draw runner` và vào việc đánh dấu trạng thái hình khi đọc review.
  Kết quả (2026-10-20): `ResolvedProfile.PresetDigest` theo mục; test: đổi `flow` chỉ vẽ lại luồng, quay lại hồ sơ cũ miễn phí (hình `superseded` được đặt lại), tắt một mục thì không vẽ và không tính; lưu hồ sơ không gọi agent.
  Trạng thái (2026-10-20): chưa làm: CLI đi qua kênh gateway `request.review.profile.*` (xem Phase 5, BE-OR-T501), nên làm sau khi có kênh.
  Kết quả (2026-10-20): `backend-go/cmd/orca-cli/internal/command/review_profile_cmd.go` (`orca review profile show|set|reset`, `--project` để chọn lớp dự án, bỏ trống là lớp tenant; `set` nhận `--spec` hoặc `--file` ("-" là stdin), kiểm JSON trước khi gọi; phiên bản lớp được đọc trước như `engine set`, `--expected-version` thắng) qua đường REST mới của gateway `/v1/projects/{id}/review-drawing-profile` và `/v1/review-drawing-profile`. 3 test với gateway giả: đúng đường dẫn, phiên bản gửi đi, spec là đối tượng JSON, đầu vào sai bị từ chối trước mọi lời gọi. PASS: `go test ./cmd/orca-cli/...`. Kết quả khớp RPC được kiểm ở tầng gateway (test REST→kênh→RPC).
## Quyết định và duyệt (CR-OR-022 backend)

- [x] **BE-OR-T420** Migration `outcome_review_decisions`; repository hai dialect; một dòng hiệu lực mỗi `(review, item_key)`, lịch sử giữ.
  Verify: lên/xuống sạch; truy vấn dòng hiệu lực.
- [x] **BE-OR-T421** `DecideOutcomeReviewItem`: `note` bắt buộc với `accept_exception/dismiss`, `dismiss` không áp lên `origin=fact`, mục `advisory` không cần quyết định, khoá lạc quan, audit không ghi `note`.
  Files: `usecase/outcome_review_decide.go`, `domain/outcome_review_decision.go`. Verify: bảng quy tắc.
- [x] **BE-OR-T422** `ApproveOutcomeReview`: `REQUEST_REVIEW_DECISIONS_OPEN`, `STALE_REVISION`; pha `design` → `NOT_READY`; không đổi `RequestStatus`.
  Files: `usecase/outcome_review_approve.go`. Verify: gọi thẳng API bỏ qua UI vẫn bị chặn.
- [x] **BE-OR-T423** Liên kết `send_back` ↔ Request con (`child_request_id`), cách chốt sau khi đọc `request_child_rules.go`/`request_link.go`.
  Verify: tạo con từ Request `completed` thành công hoặc ghi phương án thay (liên kết `relates`).
  Kết quả (2026-10-20): migration 0109 `outcome_review_decisions` + cột `approved_by/approved_at/approval_note` của `outcome_reviews`, `adapter/{postgres,mysql}/outcome_review_decision_repository.go` + hợp đồng `RunReviewDecisionContract` (một dòng hiệu lực mỗi `(review, item)`, lịch sử mới nhất trước, liên kết con chỉ với `send_back`, sáu ghi đồng thời để lại đúng một, cô lập tenant) PASS hai DB thật; đã đăng ký cột xoá được (`note`, `approval_note`) và miễn trừ.
  Kết quả (2026-10-20): `domain/outcome_review_decision.go` (`CheckDecision`) + `usecase/outcome_review_decide.go`: 14 ca quy tắc (note bắt buộc với `accept_exception`/`dismiss`, ≤ 500 ký tự tính theo rune, `accept_exception` chỉ cho mục `needsDecision`, `dismiss` chỉ cho `origin=agent`, mục không có trong review, pha `design` → `REQUEST_REVIEW_NOT_READY`, review `stale/superseded` → `STALE_REVISION`); khoá lạc quan: hai người cùng phiên bản thì người sau thua và chỉ ghi một quyết định; quyết định sau khi đã duyệt đưa review về `ready`; audit `request.review.decide` không chứa ghi chú (test dò chuỗi). **Quyền:** reporter của Request hoặc admin/lead/owner của tenant (`REQUEST_REVIEW_FORBIDDEN` cho người khác; OPA nhóm `decide` chỉ mở cửa trước, đúng mẫu hiện có).
  Kết quả (2026-10-20): `ApproveOutcomeReview`: pha `design` → `NOT_READY`; còn mục chưa quyết định → `REQUEST_REVIEW_DECISIONS_OPEN` (n) từ server kể cả khi gọi thẳng API; `stale` → `STALE_REVISION`; ghi người/giờ/ghi chú; không đổi `RequestStatus`, không tạo Approval v6 (test). Chọn bảng riêng thay vì Approval v6 (Q1 của CR-OR-022).
  Kết quả (2026-10-20): đã đọc `request_child_rules.go`: lý do liên kết `escalation` cho phép tạo Request con từ mọi Request chưa huỷ, kể cả `completed` (test `SendBackCanSpawnAChildFromACompletedRequest`), nên **không cần nới quy tắc backend**: frontend tạo con bằng `SpawnChildRequest` với `escalation`. Thêm `LinkOutcomeReviewChild(review_id, item_keys, child_request_id)` ghi `child_request_id` lên các quyết định `send_back` đang hiệu lực, chỉ nhận Request là con của Request chứa review (test).

## Hoàn tất

- [~] **BE-OR-T499** Bộ kiểm chung + `-race` + hai dialect; ghi kết quả.
  Trạng thái (2026-10-20): `go vet` sạch; `go test -race` PASS toàn bộ request-service; tích hợp Postgres 16 và MySQL 8 thật PASS (hợp đồng review/run/quyết định/hồ sơ/hình/ngữ cảnh, RLS, ẩn danh, lược đồ cột). Riêng `TestTwoRelaysConcurrentlyNoLoss` (relay outbox) fail chập chờn ở cả hai dialect với lỗi `nats: no servers available for connection`, tức máy chủ NATS nhúng của test không lên kịp; nó không dùng code của phần này và đã PASS ở các lần chạy khác, nên ghi nhận là test hạ tầng chập chờn có sẵn. T413 (CLI) đã xong cùng Phase 5.
