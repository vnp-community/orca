# BE-CV-TASK-011-05: Test hợp đồng schema hai dialect (`information_schema`) từ bảng mong đợi sinh theo hợp đồng

**From Solution:** BE-CV-SOL-011-data-model-and-migrations
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/postgres/schema_contract_integration_test.go`, `internal/adapter/mysql/schema_contract_integration_test.go` (mới); bảng mong đợi dùng chung: `backend-go/services/code-intel-service/internal/schemaexpect/core_tables.go` (mới, chỉ dùng trong test)
**Depends on:** BE-CV-TASK-011-02, BE-CV-TASK-011-03
**Status:** [x] DONE

---

## Context

Ngăn lệch giữa bản Postgres, bản MySQL và hợp đồng §4.2. Một nguồn mong đợi duy nhất (`core_tables.go`) liệt kê, cho bảy bảng + `outbox_events` + `processed_events`: tên cột, nhóm kiểu khái niệm (`uuid|ts|json|bool|vN|text|int|smallint|bigint`), `NOT NULL?`. Hai test dịch nhóm kiểu sang kiểu thực của từng DB. Tên gói `schemaexpect` (cụ thể, không `helpers`/`utils`); import chỉ từ `_test.go`.

## Việc cần làm

1. `core_tables.go`: bảng Go (`map[string][]ColumnExpect`) sinh tay từ T0–T7; mỗi cột `{Name, Concept, NotNull}`; chú thích trỏ tới mục hợp đồng.
2. Test Postgres: đọc `information_schema.columns` (`table_schema='codeintel'`); so tên, `is_nullable`, `data_type` (`uuid`, `timestamp with time zone`, `jsonb`, `boolean`, `character varying` + độ dài khi `vN`, `text`, `smallint`, `integer`, `bigint`). Thiếu/thừa cột → lỗi nêu bảng và cột.
3. Test MySQL: đọc `information_schema.columns` (`table_schema=DATABASE()`); `char(36)`, `timestamp`, `json`, `tinyint(1)`, `varchar(N)`, `text|mediumtext`, ...
4. Thêm kiểm tên chỉ mục/ràng buộc UNIQUE then chốt (`uq_reindex_active_key`, `scope_key` UNIQUE, `graph_snapshots` UNIQUE) qua `information_schema.statistics` / `pg_indexes`.
5. Test âm: dùng bảng mong đợi cố ý sai (cột thừa) trong test phụ để chắc test bắt được lệch.

## Kiểm thử

- `go test -tags=integration ./services/code-intel-service/internal/adapter/postgres/... -run SchemaContract`
- `go test -tags=integration ./services/code-intel-service/internal/adapter/mysql/... -run SchemaContract`
- CI: hai job ma trận của workflow service chạy test tương ứng.

## Tiêu chí hoàn thành

- [x] Cả hai dialect khớp bảng mong đợi (9 bảng).
- [x] Thêm/bớt cột mà không sửa bảng mong đợi làm test đỏ.
- [x] Bảng mong đợi chỉ import từ test.

## Rủi ro và lưu ý

- Số migration của CR sau (`0003+`) thêm bảng: test chỉ kiểm tập bảng của `0001–0002`, không fail khi có bảng lạ khác.
- Khi hợp đồng đổi cột, sửa **hợp đồng trước** rồi bảng mong đợi (quy ước §8.1).
