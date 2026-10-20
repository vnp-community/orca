# CR-OR-013 — Hồ sơ vẽ tuỳ chỉnh (`ReviewDrawingProfile`)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-OR-013 |
| **Tên** | Lược đồ hồ sơ vẽ `review-drawing-profile/1`, kiểm hợp lệ theo danh mục được duyệt, phạm vi tenant → project, `presetDigest` theo mục, RPC và CLI `orca review profile`, biểu mẫu cài đặt tối thiểu; **không** cho mã/skill tuỳ ý ở MVP |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-20 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-OR-010 (preset `orca-default`, khoá cache), CR-OR-011 (`preset_digest`), CR-OR-012 (vẽ lại từng mục) |
| **Mở khoá** | Mở thêm loại sơ đồ (treemap, dependency graph, data flow, swimlane) sau khi đo |
| **Tác động** | `request-service` (`domain/review_drawing_profile.go`, `usecase/review_drawing_profile.go`, bảng `review_drawing_profiles`), `proto/orca/request/v1/outcome_review.proto`, `api-gateway` (`request.review.profile.get|save`), CLI (`backend-go/cmd/orca-cli/internal/command/review_profile_cmd.go`, theo mẫu `engine_cmd.go`), frontend (`components/settings/` biểu mẫu nhỏ) |

---

## 1. Bối cảnh và vấn đề

1. Yêu cầu: cho phép tuỳ chỉnh skill vẽ nhưng **bản MVP chọn mặc định tốt nhất** (README v9 D8). Preset `orca-default` (CR-OR-010) là mặc định bất biến.
2. Skill có nhiều núm: loại sơ đồ (44–45 loại), biến thể (`minimal` sáng/tối, `full editorial`), chuyển động, kiểu phác thảo tay, biểu tượng, mức chi tiết (`faithful|balanced|simplified`), đối tượng (`engineer|mixed|executive`), cỡ khung, bảng màu và phông qua style guide.
3. Cài đặt theo project đã có mẫu: engine, agent Solution, chế độ thực thi (`engine_settings.go`, `orca engine set`); **chưa có** màn hình cài đặt engine/chế độ thực thi (handoff 2026-10-17 mục 6.4), nên một biểu mẫu mới sẽ là cái đầu tiên cho nhóm cài đặt này.
4. Khoá cache theo `preset_digest` **từng mục** (CR-OR-011 2.2) cho phép đổi cấu hình một mục mà không mất hình ở mục khác.
5. Chưa đọc `references/` của skill (danh mục loại, kích thước, vai trò màu hợp lệ); danh mục "được duyệt" ở đây chốt khi đọc.

## 2. Giải pháp đề xuất

### 2.1 Lược đồ

```jsonc
{ "schema": "review-drawing-profile/1", "name": "orca-default", "base": "orca-default",
  "sections": {
    "architecture": { "enabled": true, "type": "architecture-delta", "level": "container", "variants": ["design", "delta"], "maxNodes": 12 },
    "flow":         { "enabled": true, "type": "sequence",           "level": "container", "variants": ["design", "delta"], "maxLanes": 6 },
    "behavior":     { "enabled": true, "type": "state-machine",      "variants": ["design", "delta"], "maxNodes": 12 },
    "data":         { "enabled": true, "type": "database-schema",    "variants": ["design", "delta"], "maxNodes": 12 } },
  "style": { "variant": "minimal", "detail": "balanced", "audience": "mixed",
             "size": "medium", "motion": false, "sketchy": false, "icons": false,
             "roles": { "accent": "token:accent", "diff-add": "token:diff-add", "diff-mod": "token:diff-mod", "diff-del": "token:diff-del" } },
  "language": "en" }
```

- `type` thuộc danh mục được duyệt theo mục. MVP: đúng 4 loại ở preset; CR này thêm cho phép chọn thêm từ danh mục đã thử (ví dụ `data-flow` cho Flow, `dependency-graph` cho Architecture) **chỉ sau khi** có fixture và số đo chi phí cho loại đó.
- `roles` chỉ tham chiếu **token của Orca** (`token:<tên>` theo `main.css`), không nhận hex tuỳ ý ở bản này; bản sau có thể nhận hex kèm kiểm tương phản AA.
- `level` của `architecture` và `flow` **bị khoá ở `container`** (C4 level 2, README D13): giá trị khác bị từ chối, để người duyệt luôn thấy ranh giới service chứ không bị kéo xuống mức component. Giới hạn cứng: `maxNodes ≤ 20`, `maxLanes ≤ 7`; `motion` và `sketchy` nhận `true` nhưng cảnh báo tốn kém hơn/không dùng cho bản xuất; `variant=full` bị từ chối ở MVP.

### 2.2 Phạm vi và phân giải

Thứ tự phân giải: `project` → `tenant` → `orca-default` (bất biến, nằm trong mã). Hồ sơ tenant/project chỉ ghi **phần khác** `base` (hợp nhất theo mục). `GetReviewDrawingProfile` trả cả **hiệu lực** (đã hợp nhất) và **nguồn** từng trường. `preset_digest(section) = sha256(canonical({type, style tối thiểu liên quan, drawPromptVersion, language}))`. Bảng `review_drawing_profiles` (README 3.4): một dòng cho mỗi `(tenant, project?)`, `version` để khoá lạc quan.

### 2.3 Hiệu lực khi đổi

Lưu hồ sơ **không** vẽ lại gì và **không** tốn token. Review đang mở hiển thị "Cấu hình vẽ đã đổi cho mục X" (CR-OR-012 2.3); người dùng chọn vẽ lại mục nào (một mục hoặc "các mục đã đổi"). Review mới sẽ trượt cache ở các mục có `preset_digest` mới và vẽ khi sinh.

### 2.4 Kiểm hợp lệ

Máy chủ kiểm bằng JSON Schema + danh mục + giới hạn; sai → `REQUEST_REVIEW_PROFILE_INVALID` kèm đường dẫn trường (`sections.flow.type`) để UI hiện lỗi cạnh trường. Không nhận khoá lạ (từ chối thay vì bỏ qua).

### 2.5 Quyền, audit, CLI

Sửa hồ sơ: quản trị project (tenant: quản trị tenant); audit `request.review.profile.save` (không ghi nội dung, ghi `spec_digest` cũ/mới). CLI: `orca review profile show --project <id>`, `set --file profile.json`, `reset`; mẫu `orca engine show --check`. Biểu mẫu UI tối thiểu: chọn loại cho từng mục, biến thể, mức chi tiết, bật/tắt từng mục, nút "Đặt lại mặc định"; không có xem trước bằng AI.

### 2.6 Điều chưa mở (có chủ đích)

Skill hoặc script tuỳ chỉnh của tenant, đường dẫn skill khác, bảng màu hex tự do, mẫu (template) HTML tự viết. Lý do: mã chạy trên dev server, chuỗi cung ứng, và artifact là dữ liệu không tin cậy (CR-OR-030). Mở khi có quyết định riêng (Q1).

## 3. Quyết định thiết kế

1. **Tuỳ chỉnh là dữ liệu có lược đồ, không phải mã.**
2. **Mặc định bất biến trong mã**; hồ sơ chỉ ghi phần khác để thay đổi mặc định sau này không bị hồ sơ cũ che.
3. **Digest theo mục** nên tuỳ chỉnh một mục rẻ.
4. **Lưu không tự vẽ lại**: đổi cấu hình không bao giờ bất ngờ tốn token.
5. **Token của Orca là nguồn màu** (không phá quy tắc style guide).

## 4. Tiêu chí chấp nhận

- [ ] Không có hồ sơ nào → hiệu lực = `orca-default`; `preset_digest` ổn định giữa các tiến trình.
- [ ] Đổi `sections.flow` chỉ đổi `preset_digest` của `flow`; hình `architecture` vẫn trúng cache.
- [ ] Lưu hồ sơ không gọi agent; review mở hiển thị mục nào cần vẽ lại.
- [ ] Khoá lạ, loại ngoài danh mục, `maxNodes > 20`, `variant=full`, hex trong `roles` → `REQUEST_REVIEW_PROFILE_INVALID` kèm đường dẫn.
- [ ] Hồ sơ project che hồ sơ tenant theo mục; `Get` cho thấy nguồn từng trường.
- [ ] Người không phải quản trị không sửa được; audit không chứa nội dung hồ sơ.
- [ ] Hai người lưu cùng lúc → người sau nhận lỗi phiên bản (`version`).
- [ ] CLI `show/set/reset` hoạt động và cho kết quả giống UI.

## 5. Kiểm thử

- **Unit**: hợp nhất hồ sơ, digest theo mục, bảng ca kiểm hợp lệ, danh mục.
- **Repository (hai dialect)**: khoá lạc quan, phạm vi tenant/project.
- **Hợp đồng**: CLI ↔ RPC ↔ kênh.
- **Component**: biểu mẫu (lỗi cạnh trường, đặt lại), hai render target.

## 6. Rủi ro và điểm chưa kiểm chứng

- Danh mục loại sơ đồ phụ thuộc đọc `references/` của skill (chưa đọc).
- Một số loại (treemap, dependency graph) có thể vượt ngân sách nút; cần fixture trước khi mở.
- Biểu mẫu cài đặt là cái đầu tiên cho nhóm "cài đặt Request"; chỗ đặt trong cây cài đặt chưa quyết (handoff 6.4).
- Phông: hồ sơ phải chọn phông mà artifact không cần mạng (CR-OR-010); chưa rõ skill ghi đè phông thế nào.

## 7. Câu hỏi mở

- **Q1.** Có bao giờ cho phép skill/mẫu tuỳ chỉnh do tenant cung cấp (admin tenant, chạy sandbox)? Mặc định: không ở series này.
- **Q2.** Có hồ sơ theo loại Request (bug khác change_request) không.
- **Q3.** Tên token diff trong `main.css` (cần chốt cùng CR-OR-010 S2).

## 8. Tham chiếu

- [README v9](../README.md) D8, mục 3.4, 3.8
- [CR-OR-010](./CR-OR-010-diagram-design-drawing-pipeline.md), [CR-OR-011](./CR-OR-011-review-artifact-store-and-cache.md), [CR-OR-012](./CR-OR-012-regenerate-review-artifacts.md)
- `backend-go/services/request-service/internal/domain/engine_settings.go`, `backend-go/cmd/orca-cli/internal/command/engine_cmd.go`
- [handoff 2026-10-17](../../../handoff/2026-10-17-request-flow-and-qa-fixes.md) mục 6
