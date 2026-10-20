---
cr_id: CR-OR-010, CR-OR-011, CR-OR-012, CR-OR-013
solution_id: BE-OR-SOL-003
product: orca
service: request-service
layer: backend
language: go
status: draft
priority: P0 (013 là P1)
effort: 10–13 ngày
depends_on: BE-OR-SOL-001, BE-OR-SOL-002
created: 2026-10-20
---

# BE-OR-SOL-003 — Kho artifact, vẽ bằng `diagram-design`, sinh lại, hồ sơ vẽ

## 1. Phân tích

- Mục tiêu chính: **không trả tiền hai lần cho cùng một hình**. Khoá cache theo digest **mô hình sơ đồ**, không theo commit hay review.
- `artifact_kind.go` là kho tài liệu 8 loại có schema; không chứa HTML/SVG. Cần bảng riêng.
- gRPC backend ở trần nhận mặc định 4 MiB: lấy từng artifact (≤ 256 KiB), không trả cả review kèm hình.
- Skill `diagram-design` (MIT, 2.6) sinh HTML + SVG tĩnh; cổng *style guide* hỏi người dùng nên phải cấp hồ sơ cố định; xuất PNG cần Playwright (không dùng). `skills/` của repo chưa có skill này; mẫu phân phối: `npx skills add https://github.com/vnp-community/orca --skill … --global`.
- Có mẫu dọn định kỳ (`proposal_cleanup.go`), digest chuẩn (`canonical_json_digest.go`), `erasable_columns.go` (ĐỌC TRƯỚC để biết có phải đăng ký cột mới).

## 2. Thiết kế

### 2.1 Bảng (migration kế tiếp, hai dialect, có `down`)

- `review_artifacts`: `id, tenant_id, request_id, section, variant, diagram_type, input_digest, preset_digest, skill_version, blob_digest, revision, state(active|superseded|failed), origin(auto|regenerated), reason, self_check, error_code, model(≤32 KiB), view_box, title, desc, created_by, created_at`. Duy nhất `(tenant, request, section, variant, input_digest, preset_digest, skill_version, revision)`; chỉ một `active` cho `(tenant, request, section, variant)` (chỉ mục duy nhất một phần; MySQL dùng cột sinh `active_key`).
- `review_artifact_blobs`: `(tenant_id, digest)` khoá chính, `media_type, size_bytes, content, created_at`.
- `review_drawing_profiles`: `id, tenant_id, project_id?, name, spec, spec_digest, version, updated_by, updated_at`.
- `outcome_review_decisions` ở SOL-004.

### 2.2 Khoá cache và tra cứu

`CacheKey = (tenant, request, section, variant, input_digest, preset_digest, skill_version)`; `Resolve(review)`: trúng (kể cả `superseded` → đặt lại `active` miễn phí), trượt, hoặc `failed` (không tự thử lại). Phạm vi `(tenant, request)`; blob dedupe `(tenant, digest)`. `preset_digest` **theo mục** (loại + núm của riêng mục). Hình `design` có `input_digest` độc lập commit nên tái dùng sang pha `delivered`.

### 2.3 Skill ghim

- Vendor `skills/diagram-design/` tại một commit cố định + `LICENSE` + `NOTICE`; bỏ hoặc vô hiệu `export_png.py`, trích xuất draw.io/Mermaid/Excalidraw; `MANIFEST.sha256` liệt kê mọi tệp.
- `skill_version = <SKILL.md version>+<commit ngắn>`.
- Khả năng dev server `reviewDraw`: skill có mặt, `MANIFEST` khớp, Python cho `self_check.py` chạy được; thiếu → `REQUEST_REVIEW_SKILL_MISSING` (chỉ chặn draw run). ĐỌC TRƯỚC `dev_server_capability.go` để thêm khả năng cùng cách `knowledgeHub`.

### 2.4 Draw run

`usecase/outcome_review_draw_run.go`: gom mọi mô hình trượt cache của pha hiện tại (design: ≤ 4; delta: ≤ 4) vào **một** lượt agent; không trượt → không gọi agent. Lời nhắc (`drawPromptVersion` ∈ `preset_digest`): nút/làn là container C4 level 2 ghi `[kind: tech]` và id nguồn; cạnh ghi giao thức; vai trò màu `diff-add|diff-mod|diff-del`, accent cho `deviation`; mã hoá phụ ngoài màu; cấm hỏi người dùng; HTML trả **trong khối kết quả** (≤ 256 KiB mỗi artifact), `self_check.py` chạy trên tệp tạm trong thư mục làm việc của agent; tối đa 1 lần sửa mỗi sơ đồ.

### 2.5 Kiểm artifact phía Orca (`domain/outcome_review_artifact_check.go`)

Từ chối: `<script>`, `on*`, `javascript:`, `data:` ngoài ảnh nhỏ, `<iframe|object|embed|foreignObject|base|meta refresh>`, `@import`, URL ngoài (cả Google Fonts; phông do hồ sơ ghi đè). Yêu cầu: XML hợp lệ, `<svg viewBox>`, `<title>`/`<desc>`, id có tiền tố. Kiểm "chữ phải thuộc mô hình": chữ hiển thị ⊆ `nhãn ∪ tech ∪ protocol ∪ chú giải cố định` (ngưỡng lạ 5%, chưa đo). Trích `view_box`, `title`, `desc` lưu kèm. Kết quả tồi → `failed` + mã lỗi, UI lùi về bảng.

### 2.6 Sinh lại

`RegenerateReviewArtifacts(request_id, scope: artifact|context|all, sections[], reason, note?, confirm_cost?, client_request_id, expected_revision)` và `RestoreReviewArtifactRevision`. Revision mới dù khoá trùng (`origin=regenerated`); khôi phục không tốn AI; quyết định gắn `item_key` giữ nguyên, `item_key` mất → `orphaned`; review `approved` gặp mục cần quyết định mới → `ready`. Hạn mức `REQUEST_REVIEW_REGEN_PER_DAY` (5) và 2 lượt đồng thời mỗi tenant; `active_key` chặn chạy chồng; xác nhận chi phí khi vượt `REQUEST_REVIEW_CONFIRM_COST`. `EstimateOutcomeReview` nhận `scope/sections` để báo số lượt, số sơ đồ vẽ, `lastActualCost` nếu có.

### 2.7 Hồ sơ vẽ (P1)

Lược đồ `review-drawing-profile/1`; `base: orca-default` bất biến trong mã; hợp nhất `project → tenant → default` theo mục; `level` của architecture/flow khoá `container`; `variants` ∈ `design|delta`; `roles` chỉ tham chiếu token; từ chối khoá lạ, hex, `variant=full`, `maxNodes > 20`. Lưu hồ sơ **không** vẽ lại; review mở hiển thị mục nào đổi. RPC `Get/SaveReviewDrawingProfile` (khoá lạc quan `version`), CLI `orca review profile show|set|reset` (mẫu `engine_cmd.go`).

### 2.8 Dọn dẹp, số liệu

Job định kỳ (kiểu `proposal_cleanup.go`): xoá artifact quá `REQUEST_REVIEW_ARTIFACT_RETENTION` (90 ngày sau khi bị thay), rồi blob mồ côi sau thời gian ân hạn; hạn mức `REQUEST_REVIEW_ARTIFACT_QUOTA_MB` theo tenant. Đếm `hit|miss|failed|corrupt`, `bytes_stored`, `blobs_deduped`. Kiểm sha256 khi đọc; lệch → coi hỏng, không phục vụ.

### 2.9 Cấu trúc file

`domain/review_artifact.go`, `review_artifact_key.go`, `outcome_review_artifact_check.go`, `outcome_review_preset.go`, `review_drawing_profile.go`; `usecase/review_artifact_store.go`, `review_artifact_cleanup.go`, `outcome_review_draw_run.go`, `outcome_review_draw_prompt.go`, `outcome_review_regenerate.go`, `review_drawing_profile.go`; `adapter/{postgres,mysql}/review_artifact.go`, `review_drawing_profile.go`; `adapter/grpc/review_artifact_server.go`.

## 3. Kiểm thử

Repository hai dialect (ràng buộc, một `active`, dedupe, revision); đồng thời (`-race`); bộ HTML độc hại cho bộ kiểm; agent giả cho draw run (tốt, sửa một lần, lỗi hai lần, hỏi lại người dùng, thiếu nonce); đếm lượt agent (mở lần hai = 0); manifest lệch → thiếu skill.

## 4. Rủi ro

Spike S1 đến S4 chưa chạy (hồ sơ không tương tác, vai trò màu điền token lúc hiển thị, chi phí thật, ký hiệu C4); lưu blob trong DB có thể phình; ngưỡng 5%/512 MB/90 ngày là phỏng đoán.
