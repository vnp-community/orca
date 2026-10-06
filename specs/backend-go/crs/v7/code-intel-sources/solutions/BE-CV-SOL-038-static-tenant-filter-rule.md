# BE-CV-SOL-038-static-tenant-filter-rule: Quy tắc tĩnh `sql.missing-tenant-filter` / `sql.insert-missing-tenant` (AST Go, tầng độ tin cậy, bộ vàng)

> **📋 Proposed.** Chưa triển khai, chưa chạy test/CLI. P2 (đợt 6). Là một `Detector` cắm vào `ListFindings` của [`BE-CV-SOL-037-structure-findings-and-dismissals`](./BE-CV-SOL-037-structure-findings-and-dismissals.md). **Thử nghiệm mặc định** cho tới khi bộ vàng đạt ngưỡng precision (O-7). Số đo của CR là regex thô của người soạn CR, **không** phải kết quả bảo mật.

**CR:** [CR-CV-038](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md) (§2.5 và §1 "Isolation tenant")
**Service:** `code-intel-service` (mới)
**Hợp đồng áp dụng:** [`CONTRACT-codeintel-proto-and-data-map.md`](../../CONTRACT-codeintel-proto-and-data-map.md): **PQ-05** (dismissal), **PQ-06** (`rule`, `kind=missing_tenant_id`, `severity`, `confidence`, `origin`), **PQ-15** (cache), **PQ-14**, **PQ-04**; §3.1 `ListFindings`, `DismissFinding`, §4.2 T1/T5, §6 (biến môi trường: **thiếu** biến cho chế độ quy tắc này, xem mục 4). UI §4.5 `Finding`. Không có RPC mới.
**TDD tham chiếu:** [`arch/05`](../../../../tdd/architecture/05-data-architecture.md) §Multi-tenancy (RLS + `WHERE tenant_id`), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) §Multi-tenancy isolation, [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (metric precision).

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go` dòng 68–95 (`Update`: `UPDATE project.folder_workspaces SET name = $2 WHERE id = $1 RETURNING `+folderWorkspaceColumns`; `Delete`: `DELETE FROM project.folder_workspaces WHERE id = $1`; cả hai **không** có `tenant_id`; chuỗi nối `+`), `backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go` (`withTenantTx` gọi `set_config('app.tenant_id', $1, true)`; `withRelayTx` đặt `app.relay`; chỉ hai hàm của `common/outbox.Store` được dùng), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (không liên quan trực tiếp), `docs/crs/v7/.../CR-CV-038` §1 (các dẫn chứng RLS `task-service/migrations/*/0001_init.up.sql:33–52`, `share_link.go:17–20`, `annotation-service/.../repository.go:30`, `scm-integration-service/.../rate_limit_cache.go:37` — **chưa đọc lại** từng dòng, lấy từ CR), đặc tả `BE-CV-SOL-031-erd-model-and-access-scan` (`ErdTable.tenantScoped`, `accessedBy`, quét literal SQL bằng `go/parser`).

Xác nhận đúng: hai hàm `Update`/`Delete` ở `folder_workspace_repository.go` chỉ nhận `id` và SQL không có `tenant_id` (đã đọc); `mcp-service` có `withTenantTx` thật. **Chưa kiểm chứng**: use case gọi `Update/Delete` có chặn tenant trước hay không (⇒ chưa được coi là lỗ hổng thật; chỉ là "ứng viên mạnh nhất").

### Correction relative to CR-CV-038 §2.5

| # | CR nói | Thực tế / hợp đồng | Xử lý |
|---|--------|--------------------|-------|
| C1 | "~6 ứng viên `high`" từ regex thô | Chuỗi bị nối `+`, dùng `fmt.Sprintf`, `strings.Builder` làm regex bỏ sót; phân tầng "cùng-tệp" thay vì "cùng-hàm" | Không dùng số đó làm chỉ tiêu; ngưỡng nghiệm thu là precision trên bộ vàng |
| C2 | Danh sách bảng từ `ErdModel` | `ErdTable.tenantScoped` (UI §4.4) do 031 điền | Dùng `tenantScoped`; thiếu ERD ⇒ bộ **skipped** (không đoán) |
| C3 | `Finding.introduced` | PQ-06: `origin` | Theo hợp đồng |
| C4 | Tắt bằng `findings.disabledRules` trong `c4.yaml` | O-12/Q chưa chốt; `BE-CV-SOL-033-c4-overrides-yaml` có thể không có khoá này | Không hiện thực ở MVP (mục 8 Q2) |
| C5 | "Chế độ thử nghiệm: ẩn khỏi danh sách mặc định" | Hợp đồng không có cờ/biến | Cơ chế đề xuất: `Detector.Experimental()` ⇒ chỉ chạy khi `rules[]` nêu tường minh; biến `CODEINTEL_SQL_TENANT_RULE_MODE` (mới, đề xuất) |
| C6 | `outbox_events` "relay cố ý xuyên tenant" | `mcp-service` có `withRelayTx`; `common/outbox` | Allowlist bảng `outbox_events`, `processed_events` và hàm dùng `withRelayTx` |

## 2. Giải pháp

### 2.A Cây file (mới)

```
backend-go/services/code-intel-service/
  internal/adapter/gosqlscan/{sql_string_extraction.go, allow_comment_directive.go, enclosing_function.go}   # go/parser
  internal/domain/tenantfilter/{sql_statement_analysis.go, tenant_filter_rule.go, confidence_tiers.go,
                                global_key_columns.go, tenant_finding_key.go}
  internal/usecase/tenant_filter_detector.go          # cài Detector của 037
  testdata/sqltenant/{go/*.go, golden/*.yaml, README.txt}
```

Parse Go ở `adapter/` (chuẩn `go/parser`); phân tích SQL và phân tầng ở `domain/` (thuần). Không thêm dependency.

### 2.B Trích chuỗi SQL (AST Go, không regex trên mã)

Trong `internal/adapter/{postgres,mysql}/**.go` (bỏ `_test.go`): literal chuỗi, phép nối `+` giữa literal, đối số đầu của `fmt.Sprintf`, tham chiếu `const`/`var` **cùng tệp**; ghi `enclosingFunc` (tên hàm/method), dòng đầu câu lệnh. Chỉ xét chuỗi bắt đầu (sau comment `--`) bằng `SELECT|UPDATE|DELETE|INSERT|WITH`. SQL dựng động bằng `strings.Builder`/`WriteString` **không phân tích** ⇒ bỏ (không báo) và đếm `skipped_dynamic` vào metric. Chuẩn hoá: viết thường, gộp khoảng trắng, `$n` và `?` ⇒ `?`. Trường hợp `... RETURNING `+folderWorkspaceColumns` (nối với `const` ngoài hàm cùng tệp) phải chạy được (ca vàng).

### 2.C Điều kiện báo và tầng độ tin cậy

- `SELECT/UPDATE/DELETE` (kể cả CTE/subquery): có ít nhất một bảng ∈ `tenantTables(service, dialect)` (`ErdTable.tenantScoped`) **và** toàn văn truy vấn không có định danh `tenant_id` (ranh giới định danh: `tenant_id` ≠ `tenant_identity`, ≠ `x_tenant_id`) ⇒ ứng viên `sql.missing-tenant-filter`.
- `INSERT INTO <bảng tenant>` mà danh sách cột không có `tenant_id` (và không phải `INSERT … SELECT` có `tenant_id` trong select-list) ⇒ `sql.insert-missing-tenant`.

| Mức | Điều kiện | `severity` |
|---|---|---|
| `high` | không thuộc ngoại lệ dưới; MySQL, hoặc service Postgres **không** đặt `app.tenant_id` | `warning` |
| `medium` | có truy vấn **cùng bảng** có `tenant_id` trong **cùng hàm bao ngoài**; hoặc service Postgres có `set_config('app.tenant_id'` / `SET LOCAL app.tenant_id` (RLS có thể đang hiệu lực: chỉ `mcp-service` theo CR) | `info` |
| `low` | lọc theo khoá toàn cục duy nhất (`*_hash`, `token`, `code`, `delivery_id`; danh sách cấu hình, `global_key_columns.go`) | `info` |
| bỏ qua | bảng `outbox_events`, `processed_events`; hàm tên chứa `Outbox`/`relay` hoặc dùng `withRelayTx`; có chú thích cho phép hợp lệ | — |

Phát hiện "service có RLS hiệu lực" là heuristic (tìm literal trong `internal/adapter/postgres`); nhãn "RLS có thể đang hiệu lực", không khẳng định.

### 2.D `finding_key`, tắt/giảm ồn, phạm vi

`finding_key = "sql.missing-tenant-filter:" + hex(sha256("<service>::<dialect>::<đường dẫn tệp>::<enclosingFunc>::<SQL chuẩn hoá>"))[:16]` (PQ-06 `rule`→`kind:"missing_tenant_id"`; ổn định khi dòng dịch chuyển, đổi khi SQL đổi). Tắt/giảm ồn: (1) `DismissFinding` (037); (2) chú thích ngay dòng trước câu lệnh hoặc trước hàm: `// codeintel:allow sql.missing-tenant-filter <lý do ≥ 10 ký tự>` (đọc bằng AST comment; thiếu lý do ⇒ vô hiệu và báo `info` `sql.allow-without-reason`); (3) công tắc toàn quy tắc: `CODEINTEL_SQL_TENANT_RULE_MODE ∈ off|experimental|on` (đề xuất, mặc định `experimental`); không công tắc theo tenant. Phạm vi: `scope=CHANGED` chỉ tệp adapter đã đổi; `ALL` quét ≈ 171 tệp (CR đếm) như job nền (hạn mức CR-013/021), cache `(repo, commit, "sql-tenant")`. `origin:"introduced"` khi câu lệnh có ở head mà không ở base (so `finding_key`; base qua cổng đọc ở commit).

### 2.E Chế độ thử nghiệm và ngưỡng nghiệm thu

`Experimental()=true` khi mode `experimental`: bộ chỉ chạy nếu `rules[]` nêu tường minh `sql.missing-tenant-filter`/`sql.insert-missing-tenant` (danh sách mặc định không có); `info` không hiện mặc định ở UI. Điều kiện chuyển sang `on`: **bộ vàng 100 truy vấn** lấy mẫu có chủ ý (50 từ `high`+`medium`, 50 từ nhóm đã lọc), người phân loại thật/giả; precision `high` ≥ 60% (ngưỡng giả định chưa có dữ liệu, O-7). Chạy thường kỳ ở CR-070; ghi precision/recall mỗi phiên bản quy tắc.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| AST Go thay regex | Chuỗi nối `+`, `Sprintf`, `const` — regex bỏ sót (C1) |
| Mã động bỏ, không đoán | Tránh báo nhầm; đếm để biết độ phủ |
| Không coi RLS là đủ | Đa số service không đặt `app.tenant_id` (comment của chính repo), nên thiếu `WHERE tenant_id` là lỗ thật; vẫn hạ `medium` khi có `set_config` |
| Không có công tắc im lặng; tắt phải có lý do | Review sau còn ngữ cảnh |
| Mặc định thử nghiệm | Precision chưa đo |
| Chỉ SQL tĩnh; không taint/PDG (E13) | Ngoài phạm vi, tránh `analyze --pdg` |
| Không đưa mã nguồn vào finding | Chỉ `path`, `enclosingFunc`, SQL đã chuẩn hoá ≤ 300 ký tự (đã che) |

## 4. Lệch giữa CR và hợp đồng

| # | CR | Hợp đồng | Theo |
|---|----|----------|------|
| L1 | `introduced` | PQ-06 `origin` | Hợp đồng |
| L2 | `severity: warning|info` theo tầng | PQ-06 `error|warning|info` | Giống |
| L3 | Công tắc/biến cho trạng thái thử nghiệm | §6.2 không có biến | Đề xuất `CODEINTEL_SQL_TENANT_RULE_MODE`; **cần chủ hợp đồng thêm vào §6.2** |
| L4 | `findings.disabledRules` ở `c4.yaml` | chưa có | Không làm (C4) |
| L5 | `reason` bắt buộc khi dismiss | PQ-05 `reason` tuỳ chọn | Hợp đồng (xem 037 L1); riêng chú thích `codeintel:allow` vẫn yêu cầu lý do ≥ 10 ký tự (đó là mã, không phải RPC) |

## 5. Phụ thuộc chéo khu vực và thứ tự

| Cần | Từ | Dạng |
|-----|----|------|
| Giao diện `Detector`, `ListFindings`, `DismissFinding`, `finding_key` | `BE-CV-SOL-037-structure-findings-and-dismissals` | cứng |
| `ErdTable.tenantScoped`, `accessedBy` | `BE-CV-SOL-031-erd-model-and-access-scan` | cứng (thiếu ⇒ bộ `skipped`) |
| Đọc tệp adapter, base/head | `BE-CV-SOL-030-repo-file-access-gateway` | cứng |
| `changedFiles` | `BE-CV-SOL-036-change-overlay-pipeline` | mềm |
| Bộ vàng, chạy thường kỳ | `BE-CV-SOL-070-collector-golden-contract` | sau |
| Hiển thị | `FE-CV-SOL-059-contract-lens-and-findings` | sau |
| Phía agent | **không có** | — |

Thứ tự: 037 → `BE-CV-SOL-038-contract-diff` (song song được) → solution này (cuối).

## 6. Kiểm thử

- **AST**: `Sprintf`, nối `+` với `const` cùng tệp (đúng ca `… RETURNING `+folderWorkspaceColumns`), `strings.Builder` (bỏ), comment `codeintel:allow` hợp lệ/không lý do/lý do < 10 ký tự, hàm `*Outbox*`.
- **SQL**: CTE, subquery, `RETURNING`, `FOR UPDATE`, `ON CONFLICT`, `INSERT … SELECT`; hai dialect (`schema.table` Postgres, không tiền tố MySQL; `$1` vs `?`); `tenant_id` vs `tenant_identity`.
- **Tầng**: ≥ 1 ca mỗi tầng; `outbox_events` không bao giờ báo; Postgres có `set_config` ⇒ `medium`; MySQL ⇒ `high`.
- **Trên Orca** (golden thủ công/nightly): `folder_workspace_repository.go` `Update/Delete` xuất hiện `high`; `mcp-service` Postgres `medium`; truy vấn `token_hash`/`code_hash` `low`.
- **Bộ vàng bảo mật 100 truy vấn** có nhãn tay; ghi precision/recall.
- **Dismiss/key**: dịch dòng không đổi khoá; đổi SQL đổi khoá; dismiss ẩn ở `ListFindings`.
- **Cô lập tenant**: cache `sql-tenant` có `tenant_id`; hai tenant không đọc chéo; mọi truy vấn DB của solution đi qua repository 037/022 (đã có test).
- **Hai dialect DB**: solution không thêm truy vấn riêng; nhánh dialect của **phân tích** (không phải DB) có test hai dialect.
- Lệnh: `go test ./services/code-intel-service/internal/adapter/gosqlscan/... ./internal/domain/tenantfilter/... ./internal/usecase/ -run TenantFilter`. Chưa chạy bất kỳ test nào.

## 7. Rủi ro và điểm chưa kiểm chứng

- Chưa chạy hệ thống; precision chưa biết; ngưỡng 60% là giả định.
- `folder_workspaces` `Update/Delete` có thể an toàn nhờ use case (chưa kiểm); không coi là phát hiện bảo mật khi chưa xác minh.
- Heuristic RLS (`set_config`) có thể sai với wrapper khác.
- Phân tầng "cùng hàm" phụ thuộc phân giải `enclosingFunc` cho closure.
- Quét `scope=ALL` 171 tệp ≈ 1,2 MiB qua `fs.*` trên dev server xa (50–200 ms/lệnh): cần hạn mức; chưa đo.
- `relay-ssh` ngoài MVP.
- Service mới có layout adapter khác (`adapter/{postgres,mysql}`) có thể bị bỏ sót.

## 8. Câu hỏi mở

1. Ai duyệt danh sách khoá toàn cục `low`; cho cấu hình theo repo?
2. Vị trí công tắc tắt quy tắc: biến môi trường (đề xuất), `c4.yaml` (O-12), hay cấu hình tenant?
3. Mở rộng sang usecase/gRPC server (không truyền `tenant`)?
4. Chỉ `scope=CHANGED` hay thêm job nền `ALL`?
5. Có đưa chú thích `codeintel:allow` vào `guides/` cho người viết adapter?

## 9. Tiêu chí chấp nhận

- [ ] Bộ vàng chứa ≥ 1 ca mỗi tầng; `outbox_events` không báo; `codeintel:allow` có lý do ≥ 10 ký tự bị bỏ; không lý do thì vô hiệu + `info`.
- [ ] `INSERT` thiếu cột `tenant_id` báo `sql.insert-missing-tenant`.
- [ ] Trên Orca: `folder_workspace_repository.go` `Update/Delete` `high`; `mcp-service` `medium`; `token_hash` `low`.
- [ ] `finding_key` ổn định khi dịch dòng, đổi khi đổi SQL; dismiss ẩn finding.
- [ ] Precision `high` trên bộ vàng ghi lại; chưa đạt ⇒ vẫn `experimental` (không chạy khi `rules[]` rỗng).
- [ ] Mã động bị bỏ và được đếm; không có mã nguồn thô trong finding.
- [ ] Hai dialect của phân tích xanh; chéo tenant bị từ chối; không `max-lines` disable.

## 10. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/code-intel-sources/CR-CV-038-contract-diff-and-static-security.md`
- `/opt/repos/orca/backend-go/services/project-service/internal/adapter/postgres/folder_workspace_repository.go` (dòng 68–95), `/opt/repos/orca/backend-go/services/mcp-service/internal/adapter/postgres/tenant_tx.go`, `/opt/repos/orca/AGENTS.md`
- Series: `BE-CV-SOL-031-erd-model-and-access-scan`, `BE-CV-SOL-037-*`, `BE-CV-SOL-030`, `BE-CV-SOL-070`, `FE-CV-SOL-059`
