# BE-CV-TASK-031-04: Nhánh MySQL/TiDB của bộ phân tích DDL

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/sqlmigration/ddl_mysql.go` (mới), `.../ddl_mysql_test.go` (mới); sửa nhẹ `column_definition.go` (BE-CV-TASK-031-03)
**Depends on:** BE-CV-TASK-031-03
**Status:** [x] DONE

---

## Context

MySQL: không schema (database riêng), `CHAR(36)`, `TIMESTAMP(6)`, `JSON`, `KEY`/`UNIQUE KEY` nội dòng, `FOREIGN KEY`, `ENGINE=InnoDB`, cột sinh `GENERATED ALWAYS AS (…) STORED` thay partial index, `MODIFY COLUMN`, `ALTER TABLE … DROP INDEX`, trigger (`task-service/0001`). Mọi nhánh rẽ theo dialect phải có test hai dialect (§8.3-3): task này bổ sung phía MySQL.

## Việc cần làm

1. `ddl_mysql.go`: `CREATE TABLE` (kể cả `KEY|UNIQUE KEY|INDEX name (cols)`, `CONSTRAINT n FOREIGN KEY (c) REFERENCES t(c) ON DELETE …`, `ENGINE=`, `DEFAULT CHARSET=`, `COLLATE`), `ALTER TABLE` (`ADD COLUMN` nhiều khoá, `MODIFY [COLUMN]`, `CHANGE COLUMN`, `ADD CONSTRAINT … FOREIGN KEY|UNIQUE|CHECK`, `DROP INDEX`, `DROP FOREIGN KEY` nếu gặp), `CREATE [UNIQUE] INDEX`, `DROP INDEX n ON t`, `DROP TABLE`, `CREATE TRIGGER n … ON t` (chỉ tên + bảng; bỏ thân đến hết `END`).
2. Tên ràng buộc ngầm MySQL `<t>_ibfk_<n>` (đánh số theo thứ tự khai báo FOREIGN KEY của bảng).
3. Cột sinh `GENERATED ALWAYS AS (expr) STORED` + `UNIQUE KEY` cùng cột: đặt `EmulatesPartialUnique=true` (suy luận) trên index đó.
4. Chuẩn hoá kiểu MySQL: `CHAR(36)` → `canonicalType` `uuid` **chỉ** khi cột là khoá hoặc tên `*_id` (nhãn suy luận do solution `erd-model` hiển thị), `TIMESTAMP(6)` → `timestamp`, `JSON` → `json`, `TINYINT(1)` → `bool`, `DOUBLE` → `float`, `BLOB` → `bytes`, `DECIMAL` → `numeric`.
5. Chọn nhánh theo `Catalog.Dialect`; một bảng điều phối `ApplyStatement(dialect, …)` duy nhất, không rẽ nhánh rải rác.
6. Bỏ qua comment MySQL dạng `/*! … */` (coi là DDL điều kiện: giữ nội dung nếu bắt đầu `/*!` — nếu gặp thì cảnh báo `MYSQL_CONDITIONAL_COMMENT`; chưa thấy trong repo).

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/sqlmigration/... -run MySQL` (chưa chạy).
- Corpus: `task-service/mysql/0001` (trigger `trg_task_edges_cascade_to`), `0012` (cột sinh `project_key`, `UNIQUE KEY idx_task_sources_unique`), `ai-provider-service/mysql/0003` (ALTER nhiều khoá có comment xen), `infra-fleet-service/mysql/0001`/`0019`.
- Golden `Catalog` MySQL của `infra-fleet-service`; so tập bảng với bản Postgres (khác biệt nằm ở `DIALECT_DRIFT` của solution `erd-model`, không ở đây).
- Ma trận: các test bảng ca chạy cho cả hai dialect với cùng khung (`for _, d := range []Dialect{Postgres, MySQL}`) đối với phần chung (PK, UNIQUE, CHECK, REFERENCES).

## Tiêu chí hoàn thành

- [x] SOL-031 mục 6 các ô MySQL (`task/0012`, trigger, `FOREIGN KEY` thêm bằng `ADD CONSTRAINT` ở `project-service/0019`) đạt.
- [x] Không có tính năng Postgres nào rò sang nhánh MySQL (ví dụ `ENABLE ROW LEVEL SECURITY` trong comment MySQL không tạo RLS).
- [x] `go vet` sạch.

## Rủi ro và lưu ý

- TiDB dùng chung nhánh MySQL (O1); cú pháp riêng TiDB (`AUTO_RANDOM`, `SHARD_ROW_ID_BITS`) chưa quét: gặp thì `UNSUPPORTED_*` cảnh báo.
- Số `CREATE SCHEMA` MySQL (14 theo `grep`) có thể là comment; parser quyết định bằng token, không bằng `grep`.
