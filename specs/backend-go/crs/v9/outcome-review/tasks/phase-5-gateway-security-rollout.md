# Phase 5 — Gateway, MCP, OPA, bảo mật, số liệu, e2e, tài liệu, rollout (CR-OR-030 và kênh)

**Solution:** BE-OR-SOL-004 §2.2–2.6 · **Phụ thuộc:** có thể làm kênh sớm theo từng RPC; phần còn lại sau Phase 4 · **Service:** `api-gateway`, `request-service`, `tests/`

## Gateway, MCP, OPA

- [ ] **BE-OR-T501** Kênh `request.review.estimate|generate|get|list|artifact|revisions|regenerate|restore|decide|approve|profile.get|profile.save` + push `request.review.changed|progress`.
  Files: `api-gateway/internal/adapter/wscompat/channels_request_review.go` (+ đăng ký ở Registry). Verify: `go test ./services/api-gateway/internal/adapter/wscompat/...`; phản hồi artifact ≤ 256 KiB.
- [ ] **BE-OR-T502** Tool MCP `request_review_*` (chỉ đọc cho agent) và OPA `agent_rpcs`; ghi/sinh lại/duyệt **không** cấp cho agent.
  Verify: thử gọi ghi bằng danh tính agent → bị từ chối.
- [ ] **BE-OR-T503** Sự kiện outbox `orca.request.review.generated|artifact_regenerated|decided` (tên chốt theo `outbox_subjects.go`).
  Verify: consumer chịu giao lặp.

## Bảo mật

- [ ] **BE-OR-T510** Bộ injection (AC, tên tệp, văn bản SAD/KG chứa chỉ thị, khối giả `</untrusted-input>`).
  Verify: kết quả vẫn qua lược đồ, không lộ biến môi trường, không id KG bịa, không công cụ ngoài danh sách.
- [ ] **BE-OR-T511** Canary bí mật (theo `code-intel-service/internal/usecase/secret_canary.go`): canary trong AC/tệp/diff/KG giả không xuất hiện ở DB, RPC, log, audit, hình.
- [ ] **BE-OR-T512** Cô lập tenant/Request: khoá cache, dedupe blob, `NOT_FOUND` không phân biệt tồn tại.
- [ ] **BE-OR-T513** Manifest skill lệch → `REQUEST_REVIEW_SKILL_MISSING`.
- [ ] **BE-OR-T514** Ma trận trạng thái × pha (trước `awaiting_plan_approval` → `NOT_READY`; `executing` → chỉ `design`; xong → `delivered` mới, `design` `superseded`, tái dùng `design.*`/hình).
  Verify: đếm lời gọi `kg_*` và lượt vẽ.

## Số liệu và log

- [ ] **BE-OR-T520** Chỉ số (SOL-004 §2.4) gồm `review_design_reviewed_ratio`; log `audit=true`.
  Verify: `review_cache_hit_ratio` tính được cho Request pilot. Chưa đặt ngưỡng số.

## e2e

- [ ] **BE-OR-T530** `tests/stages/e2e/orca-flows/suite_outcome_review.py` đăng ký ở `run_acceptance.py`: Estimate → Generate → lần hai trúng cache → sinh lại → quyết định → duyệt; cuối lượt "open Requests … 0 orphaned".
  Verify: `python3 tests/stages/e2e/orca-flows/run_acceptance.py outcome-review`.
- [ ] **BE-OR-T531** Lượt thật (tốn tiền): chỉ khi người dùng đồng ý; chạy `workspace_probe.py` trước; tuân thủ quiet-checkout 90 giây; đặt đúng engine dự án cho fixture; ghi chi phí và thời gian.

## Tài liệu và cấu hình

- [ ] **BE-OR-T540** `docs/guides/request/outcome-review-setup.md`: cài hai skill, Python cho `self_check.py`, biến môi trường, bật theo project, kiểm bằng `orca engine show --check`, lỗi `REQUEST_REVIEW_*`, xoá artifact, cập nhật skill ghim.
- [ ] **BE-OR-T541** `deploy/dev/.env.example`, `docker-compose.yml` (mặc định `REQUEST_DEFAULT_OUTCOME_REVIEW=off`), mục mới trong `deploy/dev/RELEASE-*.md`.

## Rollout

- [ ] **BE-OR-T550** Giai đoạn 0 (pilot `vnp-qa-platform`, `manual`) → 1 (chỉ chữ, `REQUEST_REVIEW_DRAW_ENABLED=false`) → 2 (có hình) → 3 (hồ sơ vẽ, thêm project). Mỗi bước ghi điều kiện qua.
- [ ] **BE-OR-T551** Kiểm công tắc `REQUEST_REVIEW_KILL_SWITCH`: ẩn tab, chặn RPC, không xoá dữ liệu; bật lại không mất review.

## Hoàn tất

- [ ] **BE-OR-T599** `go build` + `go test` + tích hợp hai dialect + `detect_changes({scope:"compare", base_ref:"main"})` (hoặc ghi rõ nếu công cụ không có); chỉ commit khi người dùng yêu cầu.
