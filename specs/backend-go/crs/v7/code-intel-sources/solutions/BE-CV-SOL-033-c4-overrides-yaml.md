# BE-CV-SOL-033-c4-overrides-yaml: `c4.yaml` ghi đè (schema v1, hợp nhất, lưu `c4_overrides`) và RPC `GetC4Overrides`/`SaveC4Overrides`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Phần 2/2 của CR-CV-033; task `07`–`11` (dãy `NN` nối tiếp [`BE-CV-SOL-033-c4-component-view`](./BE-CV-SOL-033-c4-component-view.md)).

**CR:** [CR-CV-033](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-033-c4-component-view.md)
**Service:** `code-intel-service` — `internal/domain/c4` (schema, merge), `internal/usecase/{merge_c4_overrides,get_c4_overrides,save_c4_overrides}.go`, `internal/adapter/c4overrides`, handler gRPC; bảng `c4_overrides` **thuộc `BE-CV-SOL-011-data-model-and-migrations`/`011-repositories-and-maintenance`**
**TDD tham chiếu:** [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) ("Multi-tenancy", "Migration conventions"), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("AuthZ", "Audit logging", "Input validation & supply chain"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md)

---

## 0. Hợp đồng áp dụng

| Mã | Áp dụng | Mục |
|---|---|---|
| PQ-05/T6 | `c4_overrides`: `id, tenant_id, repo_id, repo_binding_id, container v255, document text (≤ 64 KiB nghiệp vụ, ≤ 128 KiB cột; MySQL MEDIUMTEXT), updated_by, updated_at, version bigint`; UNIQUE `(tenant_id, repo_id, container)`; khoá nghiệp vụ `repo_id` | §4 T6 |
| PQ-14(5) | Tài liệu ≤ 64 KiB ở mọi tầng; gateway `c4.save` ≤ 96 KiB (`document` ≤ 64 KiB) | §1 |
| PQ-04 | `selector` thay `repo_binding_id`; override khoá theo `repo_id` suy từ binding | §1 |
| PQ-03 | `CODEINTEL_VERSION_CONFLICT` (`Aborted`), `CODEINTEL_PAYLOAD_TOO_LARGE`, `CODEINTEL_INVALID_PARAMS` | `CONTRACT-ui-api` §2.3 |
| OPA | `c4_write` chỉ owner/admin; `read` cho Get | §6.3 |
| §3.1 | `GetC4Overrides → {document, version, updated_by, updated_at, seed_source}`; `SaveC4Overrides → {version, warnings[]}` | §3.1 |
| H8, §8.3-3/4 | Không secret trong tài liệu (cảnh báo); mọi nhánh hai dialect test hai dialect; mọi truy vấn `tenant_id` + test cô lập | §8.3 |

## 1. Trạng thái hiện tại (re-verify)
Đã đọc: `services/infra-fleet-service/go.mod:17` và `api-gateway/go.mod:24` có `gopkg.in/yaml.v3` trực tiếp (các service khác `// indirect`); `ls docs/code-intel` không tồn tại; không `c4*.y*ml`; bảng `c4_overrides` chưa tồn tại (service chưa có); `common/dbcapability` (`SupportsRLS` false cho MySQL → lọc `tenant_id` ở ứng dụng).

### Correction relative to CR-CV-033
| # | CR nói | Hợp đồng | Xử lý |
|---|---|---|---|
| C1 | `SaveC4OverridesRequest.expected_version string`, `repo_binding_id` | PQ-04; T6 `version bigint` | Request `selector`; `expected_version` kiểu do chủ CR gán (đề xuất `int64`; UI `expectedVersion`) — G1 |
| C2 | Q3: seed `docs/code-intel/c4/<container>.yaml` | O-12: seed tuỳ chọn, **DB thắng** | Đọc seed qua `RepoSourceReader` khi chưa có bản ghi; trả `seed_source` |
| C3 | Ghi cần "quyền (CR-013)" | §6.3 `c4_write` owner/admin | Dùng action `c4_write` |
| C4 | Giới hạn "≤ 64 KiB" | PQ-14: DB ≤ 128 KiB, gateway 96 KiB args | Service kiểm 64 KiB `document`; cột 128 KiB không dùng hết |
| C5 | Phát sự kiện audit | §5 không có subject `c4.saved`; audit qua `auditclient.Append` (nuốt lỗi, thiếu `actor_type`) | Dùng audit của SOL-013, không thêm subject NATS |

## 2. Giải pháp

### A. Schema `c4.yaml` v1 (CR 2.5; chuẩn)
`version: 1` (bắt buộc), `container`, `description` ≤ 500, `components[]` (`id`, `name` ≤ 120, `description` ≤ 500, `kind ∈ domain|usecase|adapter|grpc-server|grpc-client|config|other`, `tech`, `merge[]`, `paths[]`; ≤ 200), `hide[]`, `externals[]` (`id`, `name`, `kind ∈ service|database|queue|vault|external-api`, `description`; ≤ 40), `relations.add/remove[]` (`from,to,kind,label`; ≤ 400). `id` khớp `^[a-z0-9][a-z0-9-]{0,63}$`.

### B. Xác thực khi lưu (`internal/domain/c4/overrides_schema.go`)
`yaml.v3` `Decoder.KnownFields(true)` vào `yaml.Node` trước; duyệt node **từ chối `AliasNode`**, anchor, tag tuỳ chỉnh; kiểm `version==1`, `container` = container của bản ghi, giới hạn kích thước/số lượng, chuỗi hiển thị là văn bản thuần (không HTML/Markdown, chặn ký tự điều khiển); chuỗi khớp `password|token|secret|dsn=` → cảnh báo (không chặn). Lỗi cấu trúc → `CODEINTEL_INVALID_PARAMS` (`field`), quá cỡ → `CODEINTEL_PAYLOAD_TOO_LARGE`.

### C. Hợp nhất `MergeC4Overrides(view, doc) (view, warnings)` — hàm thuần (CR 2.5 quy tắc 1–7)
Bắt đầu từ view `derived`; `components[]` khớp `id` (ghi đè `name/description/kind/tech`; `merge` thay các id bằng một component gộp: hợp `symbolCount`, quan hệ, loại cạnh nội bộ; `paths` tạo component mới và loại package khỏi component mặc định; id lạ không `merge/paths` → component rỗng `declared` + `C4_UNKNOWN_ID`); `hide`; `externals`; `relations.add/remove` khớp `(from,to,kind)` sau các bước trước; `origin` = `merged|declared|derived`; lỗi không làm hỏng view (cảnh báo `C4_UNKNOWN_ID`, `C4_RELATION_SKIPPED`…).

### D. Use case và lưu trữ
- `GetC4Overrides`: `read`; bản ghi DB (`tenant_id, repo_id, container`) → `{document, version, updated_by, updated_at}`; chưa có → seed từ repo (nếu có) với `seed_source:"repo"`, `version=0`; không có → `document` rỗng, `seed_source:""`.
- `SaveC4Overrides`: `c4_write`; validate; CAS `UPDATE … WHERE id=? AND tenant_id=? AND version=?` (0 hàng → `CODEINTEL_VERSION_CONFLICT` kèm `currentVersion`); tạo mới khi `expected_version=0` (UNIQUE vi phạm → conflict: PG `23505`, MySQL `1062`); trả `warnings[]` từ thử `MergeC4Overrides` trên view hiện tại (nếu có); audit; huỷ cache view `architecture` của binding.
- Repository (hai dialect) **do SOL-011**; solution này khai báo cổng `C4OverridesRepository` và test hợp đồng ở phía use case; ca tích hợp thêm vào suite repository của SOL-011.

## 3. Quyết định thiết kế
| Quyết định | Lý do |
|---|---|
| DB thắng seed repo | O-12; lens C4 (055) sửa trực tiếp |
| Cảnh báo thay vì lỗi khi id không còn khớp | Code đổi thì `id` biến mất |
| Cấm alias/anchor | Chống amplification (YAML bomb) |
| Merge thuần hàm | Test bảng, tái dùng ở 055 qua view |

## 4. Lệch giữa CR và hợp đồng
C1–C5. Thêm: Q4 CR "ai duyệt c4.yaml": không có quy trình duyệt ở MVP, chỉ quyền ghi (O-12 chưa chốt).

## 5. Phụ thuộc chéo khu vực
BE: `011-data-model-and-migrations` (bảng T6), `011-repositories-and-maintenance`, `012`, `013-authorization-flags-and-audit`, `030` (seed), `033-c4-component-view` (view cần merge), `040-codeintel-write-and-stream-channels` (`codeIntel.c4.get/save`), `072-security-tests-service-gateway`. FE: `FE-CV-SOL-055-architecture-c4-lens` (soạn/sửa YAML). AG: không.

## 6. Tiêu chí chấp nhận
- [ ] `SaveC4Overrides` từ chối: alias, `version≠1`, vượt giới hạn, `container` sai, trường lạ; nhận mẫu CR 2.5; xung đột `expected_version` → `VERSION_CONFLICT`; chỉ owner/admin ghi.
- [ ] Merge đúng cho `merge`, `paths`, `hide`, `externals`, `relations.add/remove`; id lạ → `C4_UNKNOWN_ID`, view vẫn dựng.
- [ ] Hai tenant cùng `repo_id`/`container`: không đọc/ghi chéo (Postgres RLS role `NOBYPASSRLS`, MySQL `WHERE tenant_id`).
- [ ] Test ma trận hai dialect cho CAS và UNIQUE; không secret trong log; không `max-lines` disable.

## 7. Kiểm thử, rủi ro, câu hỏi mở
**Kiểm thử.** Unit validator (YAML độc hại, giới hạn), merge bảng ca; integration `-tags=integration` hai dialect; cô lập tenant; cache miss khi đổi version; `go test ./services/code-intel-service/internal/{domain/c4,usecase,adapter/c4overrides}/...` (chưa chạy).
**Rủi ro.** Nội dung `description` do người dùng nhập hiển thị ở UI: FE phải coi là văn bản thuần; `yaml.v3` chỉ xác nhận trong hai module.
**Hợp đồng thiếu.** G1 (kiểu `version`); G2: không có subject sự kiện/audit chuẩn cho `c4.saved`.
**Mở.** Q3/Q4 CR (seed, duyệt), O-12.

## 8. Tham chiếu
`docs/crs/v7/code-intel-sources/CR-CV-033-*.md` §2.5; hợp đồng PQ-03/04/05/14, §4 T6, §6.3, UI §3.1 (`c4.get|save`); `backend-go/services/infra-fleet-service/go.mod`.
