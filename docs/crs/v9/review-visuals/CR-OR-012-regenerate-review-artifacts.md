# CR-OR-012 — Sinh lại artifact và ngữ cảnh review (có kiểm soát)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-012 |
| **Tên** | RPC `RegenerateReviewArtifacts` với ba phạm vi (một hình, ngữ cảnh, tất cả), ước lượng chi phí trước khi chạy, lý do bắt buộc, revision giữ bản cũ, khôi phục không tốn token, so sánh hai revision, hạn mức chống lạm dụng |
| **Loại** | Feature |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-011 (revision, khoá), CR-OR-010 (draw run), CR-OR-002 (context run), CR-OR-001 (Estimate) |
| **Mở khoá** | CR-OR-013 (áp hồ sơ mới cho một mục), CR-OR-021 (nút và so sánh) |
| **Tác động** | `request-service` (`usecase/outcome_review_regenerate.go`, `domain/outcome_review_regenerate_policy.go`), `proto/orca/request/v1/outcome_review.proto`, `api-gateway` (`request.review.regenerate`), `outcome_review_runs` |

---

## 1. Bối cảnh và vấn đề

1. Hình và ngữ cảnh do LLM sinh có thể **sai, cũ hoặc xấu**; lưu cache (CR-OR-011) làm "sai một lần thành sai mãi", nên bắt buộc có đường sinh lại. Yêu cầu của người dùng: lưu để khỏi tốn token **và** cho phép sinh lại.
2. KG có thể đổi sau khi context run xong; Orca không biết (CR-OR-002 Q2). Preset hoặc skill có thể đổi (khoá cache đổi → lần Generate sau tự trượt, nhưng người dùng cần một nút rõ ràng thay vì chờ).
3. Lượt agent tốn tiền; cần xác nhận chi phí và giới hạn tần suất. Hướng dẫn hiện có ghi một Plan thật 7–13 USD; chi phí review chưa đo.
4. Quyết định của người duyệt (CR-OR-022) gắn với `item_key` ổn định; sinh lại không được làm mất chúng một cách lặng lẽ.

## 2. Giải pháp đề xuất

### 2.1 Ba phạm vi

| `scope` | Làm gì | Tốn AI |
|---|---|---|
| `artifact` (một `section`) | Vẽ lại hình của mục đó từ **mô hình sơ đồ hiện có**, tạo revision mới (`origin=regenerated`) dù khoá cache trùng | Có (draw run, một sơ đồ) |
| `context` | Chạy lại context run (đọc KG + code) → mô hình mới; hình chỉ vẽ lại ở mục nào `inputDigest` đổi (các mục khác trúng cache) | Có (context run) + draw run cho phần đổi |
| `all` | `context` rồi vẽ lại **mọi** hình (bắt buộc revision mới) | Có (đầy đủ) |

Khôi phục một revision cũ (`RestoreReviewArtifactRevision`, đặt `active`) **không** gọi AI và không cần lý do bắt buộc ngoài ghi chú tuỳ chọn.

### 2.2 Ước lượng và xác nhận

`EstimateOutcomeReview` (CR-OR-001) nhận thêm `scope`/`sections[]` và trả: số lượt chạy dự kiến, số sơ đồ sẽ vẽ, `lastActualCost` của các lượt cùng loại (nếu có), `confirmRequired` khi vượt `REQUEST_REVIEW_CONFIRM_COST` (cấu hình). UI hiện số này trước khi chạy; thiếu số liệu thì ghi "chưa có số liệu", không đưa con số bịa. RPC `RegenerateReviewArtifacts(request_id, scope, sections[], reason, note?, confirm_cost?, client_request_id)`; `client_request_id` làm khoá idempotent.

### 2.3 Lý do và nhãn gợi ý "đã cũ"

`reason` bắt buộc, một trong `looks_wrong | kg_changed | code_changed | style_changed | retry_failed | other` (+ ghi chú ≤ 200). Gợi ý hiển thị (không tự chạy): "Code đã đổi" khi `EstimateOutcomeReview.stale=true`; "Cấu hình vẽ đã đổi" khi `presetDigest` hiện tại ≠ của artifact; "Bản skill mới" khi `skill_version` khác; "KG có thể đã đổi" chỉ khi `kg.snapshot` đã lưu cũ hơn `REQUEST_REVIEW_CONTEXT_TTL`. Chưa có tín hiệu thật về KG nên nhãn KG chỉ theo tuổi.

### 2.4 Revision, so sánh và quyết định

- Mỗi lần sinh lại thêm revision; bản cũ `superseded` còn đọc được (CR-OR-011). Mô hình sơ đồ của từng revision được lưu (`model`), nên so sánh hiển thị: hai hình cạnh nhau **và** danh sách nút/cạnh thêm/bớt giữa hai mô hình (tính xác định, không AI).
- Sinh lại `context` làm `items[]` thay đổi: các `item_key` còn nguyên giữ quyết định; `item_key` biến mất → quyết định chuyển thành `orphaned` (hiển thị riêng, không xoá), `item_key` mới là chưa quyết định. Review đang `approved` mà `needsDecision` mới xuất hiện → quay về `ready` kèm `warnings: ["new_decisions_required"]`, ghi sự kiện.

### 2.5 Hạn mức

Tối đa `REQUEST_REVIEW_REGEN_PER_DAY` lần mỗi Request (mặc định đề xuất 5) và 2 lượt đồng thời mỗi tenant; vượt → `REQUEST_REVIEW_BUDGET_EXCEEDED`. Chỉ quản trị project mới nâng được. Đang có lượt chạy → `REQUEST_REVIEW_RUN_IN_PROGRESS` (trả `runId`). `expected_revision` trong yêu cầu để tránh đè khi hai người cùng sinh lại: lệch → `REQUEST_REVIEW_STALE_REVISION`.

### 2.6 Hiệu ứng phụ và audit

Audit `request.review.regenerate` (scope, section, reason, người, chi phí ước lượng/thực); sự kiện `orca.request.review.artifact_regenerated`; đẩy `request.review.progress` và `request.review.changed`. Không thay đổi `RequestStatus`, không gửi gì cho agent làm code.

### 2.7 Cấu trúc file

`usecase/outcome_review_regenerate.go`, `domain/outcome_review_regenerate_policy.go` (hạn mức, quy tắc quyết định mồ côi), `domain/outcome_review_model_diff.go` (so sánh mô hình sơ đồ). Không `helpers/utils`.

## 3. Quyết định thiết kế

1. **Sinh lại luôn là hành động của người dùng**, có lý do và xác nhận chi phí; không có tự sinh lại ngầm.
2. **Vẽ lại không đổi mô hình; sinh lại ngữ cảnh mới đổi mô hình**: tách hai thứ để người dùng sửa "hình xấu" mà không trả tiền đọc lại code.
3. **Giữ cả hai revision và cho khôi phục miễn phí**: sinh lại tệ hơn thì quay lại được.
4. **Quyết định không bao giờ biến mất lặng lẽ.**
5. **Hạn mức có hiệu lực ở server**, không chỉ ở UI.

## 4. Tiêu chí chấp nhận

- [ ] `scope=artifact` tạo đúng một revision mới của mục đó dù khoá trùng; các mục khác không đổi và không tốn token.
- [ ] `scope=context` với code không đổi nhưng KG đổi: chỉ sơ đồ nào `inputDigest` đổi mới được vẽ lại (đếm lượt vẽ).
- [ ] Khôi phục revision cũ không gọi agent; `active` đổi đúng và chỉ một.
- [ ] Quyết định gắn `item_key` còn tồn tại sau sinh lại; quyết định của `item_key` mất chuyển `orphaned`, không bị xoá.
- [ ] Review `approved` gặp mục `needsDecision` mới sau sinh lại → quay về `ready`, có cảnh báo.
- [ ] Vượt hạn mức ngày hoặc đồng thời → `REQUEST_REVIEW_BUDGET_EXCEEDED`/`REQUEST_REVIEW_RUN_IN_PROGRESS`; hai yêu cầu cùng `client_request_id` chỉ chạy một lần.
- [ ] `expected_revision` lệch → `REQUEST_REVIEW_STALE_REVISION`.
- [ ] Estimate không tốn AI; khi chưa có số liệu chi phí, không hiện số.
- [ ] Mọi lần sinh lại có audit và `reason`.

## 5. Kiểm thử

- **Unit**: bảng phạm vi → lượt chạy; quy tắc quyết định mồ côi; hạn mức; so sánh mô hình sơ đồ.
- **Agent giả**: sinh lại ra hình khác; draw run lỗi (giữ revision cũ `active`, bản mới `failed`).
- **Đồng thời/idempotent**: hai người sinh lại cùng lúc; lặp `client_request_id`.
- **Hợp đồng**: Estimate khớp số lượt chạy thật.

## 6. Rủi ro và điểm chưa kiểm chứng

- Không có tín hiệu thật "KG đã đổi"; nhãn chỉ theo tuổi.
- Chi phí thực và hạn mức mặc định (5/ngày, ngưỡng xác nhận) là phỏng đoán.
- Draw run lỗi khi đang sinh lại phải không làm mất hình hiện có (revision mới `failed`, `active` giữ nguyên) — cần test rõ.
- Chưa đọc cách `ai_budget.go` tính và áp hạn mức AI của project; hạn mức ở đây phải hoà vào đó, không dựng song song.

## 7. Câu hỏi mở

- **Q1.** Sinh lại có nên cần quyền cao hơn xem (đề xuất: cùng quyền sinh lần đầu).
- **Q2.** Có nên cho "vẽ lại tất cả mục theo hồ sơ mới" như một lệnh (CR-OR-013) không, hay luôn từng mục.

## 8. Tham chiếu

- [README v9](../README.md) D7, mục 3.5–3.6
- [CR-OR-011](./CR-OR-011-review-artifact-store-and-cache.md), [CR-OR-010](./CR-OR-010-diagram-design-drawing-pipeline.md), [CR-OR-002](../outcome-review-core/CR-OR-002-context-run-kg-and-system-diff.md)
- `backend-go/services/request-service/internal/domain/ai_budget.go`
