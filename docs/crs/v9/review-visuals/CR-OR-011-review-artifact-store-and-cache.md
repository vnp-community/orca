# CR-OR-011 — Kho artifact trực quan và cache theo nội dung

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-011 |
| **Tên** | Bảng `review_artifacts` / `review_artifact_blobs`, khoá cache theo digest mô hình sơ đồ, revision, `GetReviewArtifact` / `ListReviewArtifactRevisions`, toàn vẹn, dọn dẹp, hạn mức dung lượng |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-001 (review, RPC, kênh) |
| **Mở khoá** | CR-OR-010, 012, 021 |
| **Tác động** | `request-service` (`domain/review_artifact.go`, `usecase/review_artifact_store.go`, `adapter/{postgres,mysql}/review_artifact.go`, `migrations/{postgres,mysql}/`), `proto/orca/request/v1/outcome_review.proto`, `api-gateway` (`request.review.artifact|revisions`) |

---

## 1. Bối cảnh và vấn đề

1. Mục tiêu chính của người dùng là **không sinh lại hình** nhiều lần vì tốn token. Mỗi lần mở tab Outcome review không được phát sinh lời gọi AI.
2. `artifact_kind.go` là kho tài liệu có schema (8 loại, trần 32–256 KiB); `ExportArtifactProjection` trả Markdown ≤ 1 MB (`artifact.proto`). Chúng không chứa HTML/SVG và không có khái niệm digest nội dung hay revision hình.
3. gRPC của backend Go đang ở trần nhận mặc định 4 MiB (README v7 mục 8 điểm 16): không trả cả review kèm tất cả hình trong một message.
4. `domain/canonical_json_digest.go` đã có cách chuẩn hoá JSON và digest để dùng lại.
5. Có mẫu dọn dẹp định kỳ cho worktree proposal (`proposal_cleanup.go`, 10 phút một lần theo hướng dẫn knowledge hub) và mẫu "một lượt chạy đang hoạt động" bằng khoá duy nhất (README v7 `reindex_jobs.active_key`). **Chưa đọc** `erasable_columns.go` (đăng ký cột có thể xoá) để biết bảng mới có phải đăng ký không.

## 2. Giải pháp đề xuất

### 2.1 Bảng (xem README 3.4; điều chỉnh: thêm cột `model`)

`review_artifacts`: `id`, `tenant_id`, `request_id`, `section`, `diagram_type`, `input_digest` (digest **mô hình sơ đồ**), `preset_digest` (cấu hình hiệu lực của riêng mục này), `skill_version`, `blob_digest` (rỗng khi `failed`), `revision` (≥ 1, tăng theo `(tenant, request, section)`), `state` (`active|superseded|failed`), `origin` (`auto|regenerated`), `reason` (enum + ghi chú ≤ 200), `self_check` (JSON tóm tắt), `error_code`, **`model`** (JSON mô hình sơ đồ ≤ 32 KiB, để so sánh revision), `created_by`, `created_at`.
Ràng buộc: duy nhất `(tenant_id, request_id, section, input_digest, preset_digest, skill_version, revision)`; chỉ **một** `active` cho `(tenant, request, section)` (chỉ mục duy nhất một phần; MySQL dùng cột sinh `active_key`).

`review_artifact_blobs`: khoá `(tenant_id, digest)`, `media_type` (`text/html`), `size_bytes`, `content`, `created_at`. Digest = sha256 của **byte nội dung đã lưu** (sau khi qua bộ che secret). Cùng nội dung giữa các Request/revision dùng chung một dòng.

Hai dialect, `tenant_id` ở mọi bảng, không FK chéo service; cột nội dung dùng kiểu văn bản dài (Postgres `text`, MySQL `LONGTEXT`), UTF-8. Số migration là số kế tiếp sau 0104 tại thời điểm triển khai.

### 2.2 Khoá cache và tra cứu

`CacheKey = (tenant, request, section, input_digest, preset_digest, skill_version)`. `ResolveArtifacts(review)` cho mỗi `DiagramModel`: tìm dòng `active|superseded` cùng khoá có `blob_digest` hợp lệ → **trúng** (không vẽ; nếu dòng ấy đang `superseded`, đặt lại thành `active` cho mục này, không tốn token); không có → **trượt** (đưa vào draw run, CR-OR-010); dòng `failed` cùng khoá → coi là trượt nhưng **không tự thử lại** trong cùng yêu cầu (chỉ khi người dùng bấm "Vẽ lại"). Phạm vi tra cứu là cùng `(tenant, request)`: không tái dùng hình giữa hai Request để khỏi lộ chéo quyền xem; blob thì dedupe theo `(tenant, digest)`.

`presetDigest` tính **theo mục** (loại + núm của riêng mục đó), nên đổi cấu hình `flow` không làm mất hiệu lực `architecture` (CR-OR-013).

### 2.3 Đường đọc

- `GetReviewArtifact(request_id, section, revision?)` → `{ artifact meta, content? }`; mặc định `active`; hỗ trợ `if_digest` (trả `not_modified` khi frontend đã có), nội dung ≤ 256 KiB nên nằm trong một message. Trước khi trả, **kiểm sha256(content) == digest**; lệch → coi là hỏng, ghi log, trả `state=failed` và đưa mục vào trượt ở lần Generate kế (không phục vụ nội dung hỏng).
- `ListReviewArtifactRevisions(request_id, section)` → danh sách `{revision, state, origin, reason, createdAt, createdBy, skillVersion, presetDigest}` (không kèm nội dung).
- Kênh WS `request.review.artifact`, `request.review.revisions`; quyền đọc theo quyền xem Request. Chuỗi trả về qua cùng bộ che secret (phòng thủ nhiều lớp).

### 2.4 Một lượt vẽ đang hoạt động

`outcome_review_runs` có `active_key` duy nhất `(tenant, request, kind)`; yêu cầu vẽ/sinh lại thứ hai khi đang chạy trả `REQUEST_REVIEW_RUN_IN_PROGRESS` kèm `runId` đang chạy (idempotent).

### 2.5 Hạn mức và dọn dẹp

- Trần: blob ≤ 256 KiB; tổng ≤ 2 MiB mỗi review (≤ 8 artifact); hạn mức theo tenant `REQUEST_REVIEW_ARTIFACT_QUOTA_MB` (mặc định đề xuất 512, chưa đo); vượt → `REQUEST_REVIEW_ARTIFACT_TOO_LARGE`.
- Giữ lại: artifact của review `superseded` giữ `REQUEST_REVIEW_ARTIFACT_RETENTION` (mặc định đề xuất 90 ngày sau khi bị thay thế); `active` của review còn hiệu lực luôn giữ. Công việc dọn dẹp định kỳ (cùng kiểu `proposal_cleanup.go`) xoá dòng quá hạn, rồi xoá blob không còn tham chiếu sau thời gian ân hạn (tránh đua với draw run đang ghi). Huỷ Request không xoá ngay (cần để so sánh/tra cứu); xoá theo hạn.
- Nếu bảng chứa nội dung có thể phải xoá theo yêu cầu tuân thủ, đăng ký vào `erasable_columns.go` (cần đọc để quyết định).

### 2.6 Số liệu

Đếm `hit|miss|failed|corrupt` theo mục và theo lượt Generate, `bytes_stored`, `blobs_deduped`; làm đầu vào cho CR-OR-030 (tỉ lệ trúng cache là chỉ số thành công chính của tính năng).

### 2.7 Cấu trúc file

`domain/review_artifact.go`, `domain/review_artifact_key.go` (khoá, digest, quy tắc revision), `usecase/review_artifact_store.go`, `usecase/review_artifact_cleanup.go`, `adapter/grpc/review_artifact_server.go`.

## 3. Quyết định thiết kế

1. **Khoá theo mô hình sơ đồ, không theo commit**: cùng sơ đồ sau commit mới vẫn trúng.
2. **Blob băm nội dung, tách bảng**: dedupe, kiểm toàn vẹn, truy cập từng cái (tránh trần 4 MiB).
3. **Lưu trong DB ở MVP** (O1) với truy cập qua `blob_digest` để chuyển sang object store sau mà không đổi hợp đồng.
4. **`failed` là bản ghi hợp lệ** có mã lỗi, để UI giải thích và không tự thử lại.
5. **Không tái dùng giữa Request** để không rò rỉ quyền.
6. **Một `active` cho mỗi mục**, đổi `active` giữa các revision không tốn token.

## 4. Tiêu chí chấp nhận

- [ ] Mở tab Outcome review lần thứ hai (và các lần sau) không gọi agent, không gọi AI (đếm).
- [ ] Hai Generate đồng thời cho cùng Request → một lượt chạy, lượt kia nhận `REQUEST_REVIEW_RUN_IN_PROGRESS`.
- [ ] Đổi head commit nhưng mô hình sơ đồ y hệt → trúng cache, không vẽ lại.
- [ ] Đổi preset của một mục → chỉ mục đó trượt.
- [ ] Nội dung blob bị sửa trong DB → `GetReviewArtifact` phát hiện lệch digest, không trả nội dung, ghi log.
- [ ] Revision mới của một mục đặt revision cũ thành `superseded` và chỉ có một `active`.
- [ ] Blob giống hệt giữa hai revision/Request cùng tenant chỉ có một dòng; giữa hai tenant là hai dòng.
- [ ] Dọn dẹp xoá artifact quá hạn và blob mồ côi, không xoá blob mà draw run đang ghi.
- [ ] Hai dialect cho cùng kết quả; không FK chéo service.

## 5. Kiểm thử

- **Repository (hai dialect)**: ràng buộc duy nhất, chỉ một `active`, tăng revision, dedupe blob, truy vấn khoá.
- **Unit**: tính khoá/digest ổn định; quy tắc trúng/trượt/failed; toàn vẹn.
- **Đồng thời**: nhiều Generate song song (race detector), draw run ghi trong lúc dọn dẹp.
- **Hợp đồng**: `not_modified` với `if_digest`; trần kích thước.

## 6. Rủi ro và điểm chưa kiểm chứng

- Lưu blob trong DB làm phình bảng; số liệu thực chưa có; có thể cần object store sớm hơn dự kiến (O1).
- Chưa đọc `erasable_columns.go`, chưa biết yêu cầu tuân thủ xoá dữ liệu cho cột mới.
- Hạn mức 512 MB và hạn giữ 90 ngày là phỏng đoán.
- Đổi ngữ nghĩa khoá (ví dụ thêm trường vào mô hình sơ đồ) làm mất cache; cần `schemaVersion` trong khoá (đã nằm trong digest mô hình).

## 7. Câu hỏi mở

- **Q1.** Có cần lưu thêm bản SVG tách riêng để tải xuống/nhúng vào báo cáo v7 (CR-CV-090) không.
- **Q2.** Cho tái dùng blob giữa các project cùng tenant (dedupe) có chấp nhận được về quyền không (hiện: có, vì blob không lộ qua RPC khi không có tham chiếu).

## 8. Tham chiếu

- [README v9](../README.md) D5, D6, mục 3.4
- `backend-go/services/request-service/internal/domain/artifact_kind.go`, `canonical_json_digest.go`, `proposal_cleanup.go`, `erasable_columns.go`
- [README v7](../../v7/README.md) mục 8 điểm 16
- [Hướng dẫn knowledge hub](../../../guides/request/knowledge-hub-setup.md) (dọn worktree proposal)
