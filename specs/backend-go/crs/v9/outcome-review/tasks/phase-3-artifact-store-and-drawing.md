# Phase 3 — Kho artifact, skill ghim, vẽ (CR-OR-011, CR-OR-010)

**Solution:** BE-OR-SOL-003 §2.1 đến 2.5, 2.8 · **Phụ thuộc:** Phase 1, 2 (và spike S1 đến S4) · **Service:** `request-service`, `skills/`

## Skill ghim

- [ ] **BE-OR-T301** Vendor `skills/diagram-design/` tại commit cố định (đã duyệt), thêm `LICENSE`, `NOTICE`, `MANIFEST.sha256`; vô hiệu/xoá `export_png.py` (Playwright) và các bộ trích xuất draw.io/Mermaid/Excalidraw.
  Files: `skills/diagram-design/**`. Verify: `shasum -a 256 -c MANIFEST.sha256`; sửa 1 byte → lệch.
- [ ] **BE-OR-T302** Hồ sơ không tương tác (kết quả S1): tệp/hồ sơ style guide `orca-default` đặt cho skill; phông hệ thống (không Google Fonts).
  Verify: lượt chạy thật không hỏi lại; HTML không có URL ngoài.
- [ ] **BE-OR-T303** Khả năng dev server `reviewDraw` (skill có mặt, manifest khớp, Python cho `self_check.py`); `REQUEST_REVIEW_SKILL_MISSING` chỉ chặn draw run.
  Files: `domain/dev_server_capability.go`. Verify: manifest lệch → báo thiếu skill, context run vẫn chạy.
- [ ] **BE-OR-T304** Hướng dẫn cài `npx skills add https://github.com/vnp-community/orca --skill diagram-design --global` (xác nhận chạy được trên dev server thật).
  Files: `docs/guides/request/outcome-review-setup.md` (phần cài skill).

## Kho

- [ ] **BE-OR-T310** Migration `review_artifacts`, `review_artifact_blobs` (hai dialect, có `down`, chỉ mục duy nhất một phần/`active_key` MySQL).
  Verify: lên/xuống sạch.
- [ ] **BE-OR-T311** Domain: khoá cache, digest blob, quy tắc revision.
  Files: `domain/review_artifact.go`, `review_artifact_key.go`. Verify: bảng ca trúng/trượt/failed; `preset_digest` theo mục.
- [ ] **BE-OR-T312** Repository hai dialect: ghi/đọc, dedupe blob `(tenant, digest)`, một `active` mỗi `(tenant, request, section, variant)`, kiểm sha256 khi đọc.
  Files: `adapter/{postgres,mysql}/review_artifact.go`. Verify: `go test -tags=integration ... -run ReviewArtifact`; nội dung bị sửa → coi hỏng, không phục vụ.
- [ ] **BE-OR-T313** `Resolve(review)` + `GetReviewArtifact` (`if_digest` → `not_modified`, meta `view_box/title/desc`) + `ListReviewArtifactRevisions`.
  Files: `usecase/review_artifact_store.go`, `adapter/grpc/review_artifact_server.go`. Verify: mở lần hai = 0 lượt agent.
- [ ] **BE-OR-T314** Hạn mức (256 KiB/blob, 2 MiB/review → nâng theo ≤ 8 artifact, quota tenant), dọn dẹp định kỳ, ân hạn xoá blob, đăng ký `erasable_columns.go` nếu cần.
  Files: `usecase/review_artifact_cleanup.go`. Verify: không xoá blob mà draw run đang ghi (test đồng thời).

## Vẽ

- [ ] **BE-OR-T320** Preset `orca-default` bất biến trong mã + `preset_digest` theo mục (bốn mục × `design|delta`).
  Files: `domain/outcome_review_preset.go`. Verify: ổn định giữa tiến trình.
- [ ] **BE-OR-T321** Bộ kiểm artifact (an toàn, hình dạng, kích thước, "chữ phải thuộc mô hình" gồm `sad:` và `tech/protocol`), trích `view_box/title/desc`.
  Files: `domain/outcome_review_artifact_check.go`. Verify: bộ HTML độc hại bị từ chối; ngưỡng chữ lạ ghi nhận là chưa đo.
- [ ] **BE-OR-T322** Lời nhắc vẽ (`drawPromptVersion`): container C4 level 2 `[kind: tech]` + id nguồn, giao thức trên cạnh, vai trò màu `diff-*`, mã hoá phụ, cấm hỏi người dùng, trả HTML trong khối kết quả.
  Files: `usecase/outcome_review_draw_prompt.go`. Verify: golden prompt; hình thật sau S2/S4.
- [ ] **BE-OR-T323** Draw run: gom phần trượt vào một lượt, bỏ qua khi không trượt, tối đa 1 lần sửa, `failed` + mã lỗi, `active_key` chống chạy chồng, theo pha (`design` vẽ ≤ 4 hình `design`; `delivered` vẽ ≤ 4 hình `delta`).
  Files: `usecase/outcome_review_draw_run.go`. Verify: agent giả (tốt, sửa một lần, lỗi hai lần, hỏi lại, thiếu nonce); không vòng lặp.
- [ ] **BE-OR-T324** Số liệu `hit|miss|failed|corrupt`, `bytes_stored`, `blobs_deduped`.
  Verify: `review_cache_hit_ratio` tính được.

## Hoàn tất

- [ ] **BE-OR-T399** Bộ kiểm chung + `-race`; một lượt vẽ thật có ghi lại (cần đồng ý chi phí): token, thời gian, `self_check`, ảnh chụp.
