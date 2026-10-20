---
cr_id: CR-OR-022 (backend), CR-OR-030
solution_id: BE-OR-SOL-004
product: orca
service: request-service, api-gateway
layer: backend
language: go
status: draft
priority: P0
effort: 6–8 ngày
depends_on: BE-OR-SOL-001 đến 003
created: 2026-10-20
---

# BE-OR-SOL-004 — Quyết định, gateway/MCP/OPA, bảo mật, số liệu, rollout

## 1. Phân tích

- Quyết định chỉ có ở pha `delivered`. Request `completed` là cuối nên "gửi lại" = Request con (cơ chế `SpawnChildRequest`; ĐỌC TRƯỚC `request_child_rules.go`, `request_link.go` xem có cho tạo con từ Request `completed`).
- Kênh gateway mới cần đăng ký ở ba nơi theo mẫu `request.activity`/`request.executionSummary` (handoff 2026-10-17 mục 4.1): kênh `wscompat`, tool MCP, `agent_rpcs` của OPA.
- code-intel-service của v7 có mẫu canary bí mật (`internal/usecase/secret_canary.go`) và `secret_redactor.go` để theo cách làm.

## 2. Thiết kế

### 2.1 Quyết định và duyệt

Bảng `outcome_review_decisions(id, tenant_id, review_id, item_key, decision(accept_exception|send_back|dismiss), note, child_request_id?, decided_by, decided_at)`; một dòng hiệu lực mỗi `(review, item_key)`, lịch sử giữ. `DecideOutcomeReviewItem(review_id, item_key, decision, note?, expected_version)`:
- `accept_exception`, `dismiss` bắt buộc `note` (≤ 500); `dismiss` không áp lên mục `origin=fact`; mục `advisory` (AM-3) không có `needsDecision` nên không cần quyết định.
- Khoá lạc quan `outcome_reviews.version`; audit `request.review.decide` (không ghi `note`); sự kiện `orca.request.review.decided`.
`ApproveOutcomeReview(review_id, note?, expected_version)`: `REQUEST_REVIEW_DECISIONS_OPEN` khi còn mục `needsDecision` chưa quyết định; `REQUEST_REVIEW_STALE_REVISION` khi `stale`; không đổi `RequestStatus`, không tạo `Approval` v6. Pha `design`: cả hai RPC trả `REQUEST_REVIEW_NOT_READY`.
Liên kết `send_back` ↔ Request con: lưu `child_request_id` khi frontend tạo con xong (RPC nhỏ `LinkOutcomeReviewChild`, hoặc dùng `request_link` hiện có — chốt sau khi đọc).

### 2.2 Gateway, MCP, OPA

`api-gateway/internal/adapter/wscompat/channels_request_review.go`: kênh `request.review.estimate|generate|get|list|artifact|revisions|regenerate|restore|decide|approve|profile.get|profile.save`; push `request.review.changed|progress` (gắn `request_id`, `phase`). MCP: `request_review_estimate`, `request_review_get`, … theo mẫu `request_activity`. OPA `agent_rpcs`: thêm các RPC chỉ-đọc cho agent; ghi/sinh lại/duyệt **không** thêm cho agent. Giới hạn kích thước phản hồi (artifact ≤ 256 KiB).

### 2.3 Bảo mật

- Thứ tự phòng thủ: lược đồ kết quả → bộ kiểm artifact → CSP/sandbox ở frontend.
- Bộ test: injection (AC, tên tệp, văn bản SAD/KG chứa chỉ thị, khối giả `</untrusted-input>`), canary bí mật (chuỗi canary trong AC/tệp/diff/KG giả; kỳ vọng không xuất hiện ở DB, RPC, log, audit, hình), cô lập tenant/Request (khoá cache, dedupe blob, `NOT_FOUND` không phân biệt tồn tại), manifest skill lệch.
- Hồ sơ quyền chỉ-đọc cho context run; draw run chỉ ghi tệp tạm trong thư mục làm việc.

### 2.4 Số liệu và log

`review_cache_hit_ratio`, `review_tokens{kind}` (nếu agent báo; không thì thời gian + số lượt), `review_cost_estimate_error`, `review_draw_failure_ratio{error_code}`, `review_regenerate_rate{reason}`, `review_time_to_first_facts`, `review_unverified_item_ratio`, `review_design_reviewed_ratio` (AM-3). Log có `audit=true` cho sinh, sinh lại, quyết định, duyệt (kèm `inputsDigest`, không nội dung). Ngưỡng chỉ đặt sau khi đo trên 10 Request pilot.

### 2.5 Cờ và rollout

0. Pilot `vnp-qa-platform` (`outcome_review=manual`, có `.tdd-agent/config.toml`), chạy spike. 1. Chỉ chữ (`REQUEST_REVIEW_DRAW_ENABLED=false`). 2. Có hình (preset mặc định). 3. Hồ sơ vẽ + mở thêm project. Công tắc `REQUEST_REVIEW_KILL_SWITCH=true` ẩn tab và chặn RPC (`REQUEST_REVIEW_DISABLED`), không xoá dữ liệu. Biến: `REQUEST_DEFAULT_OUTCOME_REVIEW`, `REQUEST_REVIEW_CONTEXT_TTL`, `REQUEST_REVIEW_ARTIFACT_RETENTION`, `REQUEST_REVIEW_ARTIFACT_QUOTA_MB`, `REQUEST_REVIEW_REGEN_PER_DAY`, `REQUEST_REVIEW_CONFIRM_COST`, `REQUEST_REVIEW_DRAW_ENABLED`.

### 2.6 e2e và tài liệu

`tests/stages/e2e/orca-flows/suite_outcome_review.py` (đăng ký trong `run_acceptance.py`, nhóm dùng AI): Estimate → Generate (agent giả) → lần hai trúng cache → sinh lại một hình → quyết định → duyệt; kiểm "open Requests … 0 orphaned". Phần thật: tuân thủ quiet-checkout 90 giây, chạy `workspace_probe.py` trước, chỉ khi người dùng đồng ý chi phí. Tài liệu `docs/guides/request/outcome-review-setup.md`, cập nhật `deploy/dev/.env.example`, `docker-compose.yml` (mặc định `off`), mục mới trong hướng dẫn phát hành.

## 3. Rủi ro

`request_child_rules.go` có thể chặn tạo con từ Request `completed`; e2e thật tốn tiền; agent có thể không báo token; ngưỡng chưa có số.
