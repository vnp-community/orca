# BE-CV-TASK-031-06: Khám phá `*.up.sql`, thứ tự áp dụng, `AsOfMigration` và suite chạy toàn corpus

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/sqlmigration/migration_order.go` (mới), `.../sqlmigration/build_catalog.go` (mới, ghép: sắp → tách → áp dụng), `.../sqlmigration/migration_order_test.go`, `.../sqlmigration/corpus_test.go` (mới)
**Depends on:** BE-CV-TASK-031-03, BE-CV-TASK-031-04, BE-CV-TASK-031-05
**Status:** [ ] TODO

---

## Context

SOL-031 mục 2.E. Số migration có khoảng trống (`infra-fleet-service` thiếu `0007–0012`, số lớn nhất `0038`); không trùng số trong cùng thư mục (theo CR). `golang-migrate` áp theo số tăng dần.

## Việc cần làm

1. `migration_order.go`: `OrderUp(names []string) ([]Migration, []ParseWarning)`: giữ `*.up.sql`, bỏ `*.down.sql` và file lạ (cảnh báo `UNEXPECTED_FILE` nếu `.sql` không khớp `^\d+_.+\.(up|down)\.sql$`); sắp theo **giá trị số**; trùng số → theo tên + `DUPLICATE_VERSION`.
2. `build_catalog.go`: `BuildCatalog(ctx, dialect, defaultSchema, files []FileContent) (*Catalog, error)`: nhận nội dung file đã đọc qua `RepoSourceReader` (không tự đọc); sắp → `Tokenize` → `Split` → điều phối `ApplyStatement` → `AsOfMigration`.
3. Nhận **cả** thư mục dialect: hàm `DialectFromDir(name string) (Dialect, bool)` cho `postgres|mysql`.
4. `corpus_test.go`: (a) test trên corpus sao chép (BE-CV-TASK-031-01); (b) test "toàn repo" đi tìm `backend-go/services/*/migrations/*/*.up.sql` tương đối từ vị trí test, `t.Skip` nếu không thấy; khẳng định không panic cho **294** file (hằng `expectedUpFiles` có thể điều chỉnh bằng `-update`), in thống kê tính năng thật (số `CREATE TABLE`, `ALTER`, `CREATE INDEX`, `CHECK`, `FOREIGN KEY`, `CREATE POLICY` khai triển) để đối chiếu với CR 1.2; số `UnsupportedStatement` DDL ≤ ngưỡng đã duyệt (mặc định 0).
5. Ghi kết quả đối chiếu (lệch `grep` ↔ parser, nguyên nhân) vào `INVENTORY.md` của BE-CV-TASK-031-01.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/sqlmigration/...` (chưa chạy), kèm `-race`.
- Ca `OrderUp`: `0010` sau `0009`, `0100` sau `0099` (so sánh số, không so chuỗi), trùng số, tên lạ.
- Hiệu năng: parse 294 file < 2 s trên máy dev (mục tiêu, chưa đo); ghi số đo thật vào PR.
- Hai dialect: test chạy trên cả thư mục `postgres` và `mysql` của cùng service.

## Tiêu chí hoàn thành

- [ ] `AsOfMigration` của `infra-fleet-service` = `0038_session_origin.up.sql` (theo tên file thật tại thời điểm chạy).
- [ ] Toàn corpus không panic; thống kê khớp `grep` (lệch giải thích được) hoặc ghi vào `INVENTORY.md`.
- [ ] Không nội dung migration trong log; log chỉ `file`, `line`, `code`.

## Rủi ro và lưu ý

- Test toàn repo phụ thuộc đường dẫn tương đối; trong CI Docker có thể không có thư mục `backend-go/services` ngoài module → `t.Skip` + một job CI riêng chạy nó (liên hệ `BE-CV-SOL-070`).
- Nếu có `DUPLICATE_VERSION` thật xuất hiện về sau thì cảnh báo, không lỗi cứng.
