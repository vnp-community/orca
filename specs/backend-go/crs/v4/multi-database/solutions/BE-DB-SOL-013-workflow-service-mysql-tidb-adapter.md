# BE-DB-SOL-013: MySQL/TiDB adapter for `workflow-service`

**CR:** CR-DB-002, CR-DB-003
**Service:** `workflow-service`
**Pattern gốc:** TASK-BE-DB-002~007 (`usage-service`, pilot) — replicate nguyên vẹn, không thiết kế lại. Tham chiếu thêm BE-DB-SOL-004 (outbox-pattern, 2 repository) và BE-DB-SOL-005 (RLS-drop + RowsAffected pitfall).

---

## 1. Trạng thái thật của `workflow-service` (audit trực tiếp, không suy đoán)

`workflow-service` là service phức tạp nhất đã rollout tới thời điểm này trong batch 3+4+5 (24 migration, 3 "repository" — thực chất 4 interface: `TemplateRepository`, `ApprovalRepository` + tx-scoped `TemplateRepositoryTx`/`ApprovalRepositoryTx`, `ExecutionRepository`, `StepExecutionRepository` — dồn trên 3 file adapter: `repository.go`, `approval_repository.go`, `template_tx.go`).

Khác biệt lớn nhất so với mọi service đã rollout trước đó: `workflow-service` có **transaction-scoped sub-interfaces** (`WithTx(fn func(tx TemplateRepositoryTx) error) error`) — usecase như `PublishTemplate`/`ResolveApproval`/`RateTemplate` cần nhiều write atomically trong 1 transaction, kể cả write CHÉO giữa `ApprovalRepositoryTx` và `TemplateRepositoryTx` (`ApprovalRepositoryTx.Templates()` trả về 1 `TemplateRepositoryTx` cùng transaction). Pilot (`usage-service`) không có pattern này — đây là thiết kế MỚI, không sao chép được nguyên xi từ pilot, chỉ tuân theo tinh thần "không đổi interface, dịch đúng ngữ nghĩa".

Xác nhận qua đọc trực tiếp (không suy đoán):
- KHÔNG dùng `gen_random_uuid()` ở tầng ứng dụng — mọi `id`/`dispatch_token` đều sinh ở Go (`uuid.NewString()`), xác nhận bằng grep từng call site (`create_template.go`, `execute.go`, `wave_dispatcher.go`, `execute_ad_hoc_step.go`, `publish_template.go`). Cột `DEFAULT gen_random_uuid()` trong migration Postgres là dead code từ góc nhìn ứng dụng — giống phát hiện của `usage-service`/`annotation-service`.
- CÓ JSONB (`dag_json`, `output`, `overrides`, `inject_steps`, `remove_steps`, `payload`), CÓ RLS (4 bảng: `templates`, `executions`, `step_executions` qua join, `ratings` qua join, `approvals`), CÓ `text[]` (`tags` — KHÔNG có ở bất kỳ service nào đã rollout trước đó trong 15-service này, cần dịch thật, không có tiền lệ để copy), CÓ `WITH RECURSIVE` (`ResolveChain` — MySQL 8.0.14+/TiDB đều hỗ trợ, không cần viết lại ở Go), CÓ partial unique index (`idx_workflow_approvals_one_pending_per_template ... WHERE status = 'pending'`, `idx_workflow_templates_share_token ... WHERE share_token IS NOT NULL`), CÓ full-text search (`to_tsvector`/`plainto_tsquery`).
- `CreateTemplate`'s INSERT KHÔNG ghi `owner_id` dù domain struct có field này — comment trong `adapter/postgres/repository.go` xác nhận đây là gap tiền tồn tại, ngoài phạm vi CR-DB. Adapter MySQL tái tạo lại ĐÚNG gap này (không tự ý sửa), theo đúng nguyên tắc "dịch hành vi hiện có, không cải thiện ẩn".

## 2. `impact()` chạy thật trước khi sửa (bắt buộc theo CLAUDE.md/AGENTS.md)

Chạy trên repo `orca` qua GitNexus MCP, `direction: upstream`:

| Symbol | File | Risk | impactedCount | Ghi chú |
|---|---|---|---|---|
| `TemplateRepository` | `internal/usecase/ports.go` | **HIGH** | 16 | Toàn bộ depth=1 là `IMPORTS` cấp FILE (mọi file trong `internal/adapter/*` import package `usecase`), không phải phụ thuộc chữ ký thật — xác nhận bằng cách đọc danh sách đầy đủ, cùng dạng false-positive-do-file-fan-out đã ghi nhận ở TASK-BE-DB-009 (`issue-tracking-service`, MEDIUM 7). Không đổi interface signature nào → an toàn tiếp tục. |
| `ApprovalRepository` | `internal/usecase/ports.go` | **HIGH** | 16 | Cùng lý do — cùng danh sách 16 file `IMPORTS` cấp file. |
| `ExecutionRepository` | `internal/usecase/ports.go` | **HIGH** | 16 | Cùng lý do. |
| `StepExecutionRepository` | `internal/usecase/ports.go` | **HIGH** | 16 | Cùng lý do. |
| `New` | `internal/adapter/postgres/repository.go` | LOW | 2 | 1 caller thật (`run`, `cmd/server/main.go`). |
| `NewApprovalStore` | `internal/adapter/postgres/approval_repository.go` | LOW | 2 | 1 caller thật (`run`). |
| `run` | `cmd/server/main.go` | LOW | 1 | Không ai gọi `run` ngoài `main`. |

**Cảnh báo HIGH theo yêu cầu CLAUDE.md/AGENTS.md**: 4 interface đều báo HIGH — đã kiểm tra chi tiết (không dừng ở summary): mọi item depth=1 là quan hệ `IMPORTS` ở cấp FILE (file nào import package `usecase` thì bị liệt kê), KHÔNG phải lời gọi tới method cụ thể của các interface này. Không method nào trong 4 interface bị đổi signature bởi task này (adapter MySQL implement lại y hệt) → risk thật sự là **LOW cho mục đích của task này**, HIGH chỉ phản ánh "package `usecase` được import rộng", một sự thật cấu trúc không đổi bởi commit này. Tiếp tục sửa, đúng tiền lệ TASK-BE-DB-009's cùng dạng phát hiện.

## 3. Migration dialect-safe — điểm dịch không tầm thường

Tách `migrations/postgres/` (`git mv`, nội dung y hệt, 24 file) + `migrations/mysql/` (mới, 24 file). Điểm khác biệt đáng chú ý nhất so với mọi service rollout trước:

1. **`tags TEXT[]` → `tags JSON`** (0008) — MySQL không có array type hay GIN-index tương đương cho containment. Query Postgres gốc: `tags @> $N::text[]` (mọi tag liệt kê phải có mặt). Dịch thành N predicate `JSON_CONTAINS(tags, ?, '$')` (mỗi tag 1 predicate, AND lại) ở tầng Go — xem `internal/adapter/mysql/repository.go`'s `tagConditions`. Không có index tăng tốc tương đương GIN — full/prefix scan, một chi phí thật, ghi rõ trong migration comment, không giấu.
2. **`idx_workflow_approvals_one_pending_per_template ... WHERE status = 'pending'` (partial unique index, 0009)** — MySQL không có partial index. Dùng **generated column** (`pending_template_id GENERATED ALWAYS AS (CASE WHEN status='pending' THEN template_id ELSE NULL END) STORED` + `UNIQUE INDEX` trên cột đó) — NULL không tham gia UNIQUE index của MySQL (cùng quy tắc "nhiều NULL được phép" dùng cho `share_token`), nên constraint chỉ thật sự áp dụng cho các dòng đang pending. Đã viết test xác nhận (`TestApprovalStore_CreateTx_SecondPendingForSameTemplateConflicts`) — pass thật, xem §6.

   **Bug thật phát hiện + sửa khi verify thật (không suy đoán, tái hiện trực tiếp trên container)**: bản đầu của `0009_template_visibility_sharing.up.sql` giữ `fk_workflow_approvals_template ... ON DELETE CASCADE` (y hệt Postgres) — `migrate up` fail thật với `Error 1215: Cannot add foreign key constraint` ngay tại statement thêm `pending_template_id`. Cô lập bằng tay từng statement trên container `mysql:8` (resolve ra `8.4.11`) xác nhận: **MySQL/InnoDB từ chối `ON DELETE CASCADE`/`SET NULL`/`ON UPDATE CASCADE`/`SET NULL` trên 1 FK mà cột của nó cũng là base column của 1 STORED generated column khác trong CÙNG bảng** — `template_id` vừa là cột FK vừa là input của `pending_template_id`. Sửa: bỏ `ON DELETE CASCADE` khỏi `fk_workflow_approvals_template` (dùng default RESTRICT) — không mất chức năng thật nào, vì `usecase.TemplateRepository` **không có method Delete nào** (grep xác nhận `ports.go`), CASCADE này chưa từng có đường kích hoạt thật kể cả ở Postgres. Xem migration comment đầy đủ tại chỗ.

   **Bug thật thứ 2 phát hiện khi test `down` migration**: `0003_template_parent_chain.down.sql` gốc DROP INDEX trước khi DROP FOREIGN KEY phụ thuộc index đó — fail thật với `Error 1553: Cannot drop index ... needed in a foreign key constraint`. Sửa: đổi thứ tự (DROP FOREIGN KEY trước, DROP INDEX sau) — xác nhận bằng cách chạy trọn vẹn `migrate up` rồi `migrate down -all` trên cùng 1 container, cả 2 chiều đều thành công sau khi sửa.
3. **`share_token TEXT UNIQUE` (nullable, NULL cho tới khi public)** — MySQL's plain `UNIQUE` index trên cột nullable đã tự nhiên cho phép nhiều NULL (cùng ngữ nghĩa với Postgres partial index `WHERE share_token IS NOT NULL`) — KHÔNG cần generated-column workaround ở đây, chỉ cần đổi `TEXT` → `VARCHAR(255)` (InnoDB từ chối `TEXT` trong UNIQUE key không có prefix length).
4. **`to_tsvector('english', ...) @@ plainto_tsquery('english', ...)` → `FULLTEXT INDEX` + `MATCH(...) AGAINST(... IN NATURAL LANGUAGE MODE)`** — InnoDB FULLTEXT (MySQL 5.6+), không có "50%-threshold" quirk của MyISAM. Khác biệt thật về recall: `ft_min_word_len` mặc định + stopword list riêng của InnoDB khác 'english' text search config của Postgres — ghi rõ trong migration comment, không phải bug, là giới hạn dialect thật.
5. **`WITH RECURSIVE` (`ResolveChain`)** — MySQL 8.0.14+/TiDB hỗ trợ sẵn, KHÔNG cần viết lại thành vòng lặp Go — dịch gần như nguyên văn.
6. **Row-value subquery comparison** (`(created_at, id) < (SELECT created_at, id FROM ...)`, dùng ở `ListExecutions`'s keyset cursor) — MySQL/TiDB hỗ trợ cú pháp này y hệt Postgres, không cần viết lại.
7. **`gen_random_uuid()`/`BIGSERIAL`/`RETURNING`** — không áp dụng như mọi service khác (§1): mọi `id` sinh ở Go, không `RETURNING` nào phụ thuộc giá trị DB-generated.

## 4. `internal/adapter/mysql/` — 3 file mirror `adapter/postgres`'s 3 file

`repository.go` (Template CRUD + Execution + StepExecution + outbox `Store`), `template_tx.go` (`Repository.WithTx`/`GetByShareToken`/`SetShareToken` + `templateTx`), `approval_repository.go` (`ApprovalStore` + `approvalTx`) — cùng cấu trúc file, cùng tên type/method, KHÔNG đổi signature interface nào.

### 4.1 MySQL RowsAffected() pitfall (BE-DB-SOL-005 §3.1's phát hiện, áp dụng RỘNG hơn ở đây)

`workflow-service` có NHIỀU write-by-primary-key hơn bất kỳ service nào đã rollout (`Update` version-conditional, `UpdateExecution`, `UpdateStepExecution`, `SetShareToken`, `UpdateVisibility`, `SetVisibility`, `IncrementUsageCount`, `approvalTx.Update`) — mỗi cái đều dùng `tag.RowsAffected() == 0` ở bản Postgres để phát hiện "not found". Thay vì áp dụng riêng lẻ cách né tránh của BE-DB-SOL-005 (UPDATE rồi SELECT lại), solution này dùng **`SELECT ... FOR UPDATE` bên trong transaction** trước mỗi UPDATE — vừa tránh hoàn toàn ambiguity của `RowsAffected()`, vừa khoá row chống race, mạnh hơn "UPDATE rồi đọc lại" (không có khoảng hở TOCTOU giữa UPDATE và SELECT xác nhận). `Repository.Update` (version-conditional) dùng pattern này để thay thế hoàn toàn `UPDATE ... RETURNING` atomic của Postgres — khoá dòng, so sánh `version`, UPDATE, đọc lại, cùng 1 transaction.

### 4.2 `approvalTx.CreateTx`'s duplicate-key mapping

`pgUniqueViolation` (SQLSTATE `23505`) → MySQL's `ER_DUP_ENTRY` (error number `1062`, qua `github.com/go-sql-driver/mysql`'s `*mysql.MySQLError`) — cùng nguyên tắc, khác mã lỗi cụ thể theo driver.

## 5. Wiring `cmd/server/main.go`

Theo đúng `switch caps.Dialect` của `usage-service`, nhưng `workflow-service`'s `main.go` đã KHÔNG dùng `secrets.DatabaseCredentialsFromFile` từ trước (đọc thẳng `cfg.DatabaseDSN`, để lại comment "not wired in this scaffold") — task này đóng luôn gap đó như một tác dụng phụ hợp lý của việc wiring dialect factory (cùng tiền lệ BE-DB-SOL-005 §5 làm cho `annotation-service`), thêm `DatabaseCredentialsFile` vào `internal/config/config.go`.

`repo` cần thoả mãn ĐỒNG THỜI `TemplateRepository`+`ExecutionRepository`+`StepExecutionRepository`+`outbox.Store` (4 interface, không phải 1 `usecase.Repository` gộp như pilot) — định nghĩa 1 interface cục bộ `workflowStore` trong `main.go` gộp cả 4, gán từ `*postgres.Repository`/`*mysql.Repository` cụ thể mỗi nhánh dialect (cả 2 type đều implement đủ 4 interface). `approvalStore usecase.ApprovalRepository` tương tự, gán từ `NewApprovalStore` mỗi nhánh.

## Rủi ro / Phụ thuộc

| Hạng mục | Rủi ro | Ghi chú |
|---|---|---|
| `tags` JSON_CONTAINS thay GIN containment | Trung bình | Đúng ngữ nghĩa, mất tăng tốc index — chấp nhận được cho khối lượng dữ liệu hiện tại, ghi rõ không giấu |
| Generated-column unique index cho approval pending-gate | Thấp | Đã test thật xác nhận hoạt động đúng cả 2 chiều (chặn 2nd pending, cho phép pending mới sau khi resolve) |
| FULLTEXT recall khác `to_tsvector`/`english` config | Trung bình | Dialect difference thật, không phải bug — ghi trong migration comment |
| `impact()` HIGH trên 4 interface | Thấp (đã giải thích) | Toàn depth=1 là file-level IMPORTS, không phải chữ ký — xem §2 |

## Không thuộc phạm vi solution này

- 6 service còn lại của batch 3+4+5 — nhân rộng, ngoài phạm vi (mỗi service 1 agent riêng theo `ROLLOUT-TRACKING.md`).
- Sửa `common/dbcapability`/`common/testutil` — không tìm thấy bug nào khi dùng cho service này, không có gì để "flag" (đúng chỉ dẫn "flag don't fix").
- Business logic thực thi workflow (execution engine, wave dispatch, step executor) — chỉ đụng tầng adapter SQL, đúng giới hạn nhiệm vụ.

## Liên quan

- `backend-go/services/workflow-service/internal/usecase/ports.go` (4 interface)
- `backend-go/services/workflow-service/internal/adapter/postgres/{repository,approval_repository,template_tx}.go` (bản gốc để dịch)
- `backend-go/services/workflow-service/internal/adapter/mysql/{repository,approval_repository,template_tx}.go` (bản dịch, MỚI)
- [BE-DB-SOL-001](./BE-DB-SOL-001-dialect-capability-layer-usage-service.md)/[BE-DB-SOL-002](./BE-DB-SOL-002-usage-service-mysql-tidb-adapter.md) (pattern gốc)
- [BE-DB-SOL-004](./BE-DB-SOL-004-issue-tracking-service-mysql-tidb-adapter.md) (outbox-pattern, 2-repository tiền lệ)
- [BE-DB-SOL-005](./BE-DB-SOL-005-annotation-service-mysql-tidb-adapter.md) (RLS-drop, RowsAffected pitfall gốc)
- [TASK-BE-DB-018](../tasks/TASK-BE-DB-018-workflow-service-mysql-rollout.md) (task doc, kết quả thực tế đầy đủ)
