# BE-CV-TASK-031-05: Nhận dạng thành ngữ RLS `DO $$ … FOREACH … format(…)`

**From Solution:** BE-CV-SOL-031-sql-migration-parser
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/sqlmigration/rls_do_block.go` (mới), `.../rls_do_block_test.go` (mới)
**Depends on:** BE-CV-TASK-031-03
**Status:** [ ] TODO

---

## Context

`mcp-service/migrations/postgres/0001_init.up.sql` dòng 47–57 sinh `ENABLE`/`FORCE ROW LEVEL SECURITY` và policy `tenant_isolation` bằng `FOREACH t IN ARRAY ARRAY[…] LOOP EXECUTE format('ALTER TABLE mcp.%I …', t); … EXECUTE format($p$CREATE POLICY … ON mcp.%I USING (…) WITH CHECK (…)$p$, t); END LOOP;`. Parser DDL thường không thấy. Có **7** file Postgres chứa `DO $` (SOL-031 C1); chỉ nhận dạng đúng thành ngữ, không chạy PL/pgSQL.

## Việc cần làm

1. `ExpandRLSDoBlock(stmt Statement) (expanded []Statement, ok bool, w *ParseWarning)`: nhận token của câu `DO $tag$ … $tag$`; khớp mẫu cứng: `DECLARE <v> text; BEGIN FOREACH <v> IN ARRAY ARRAY[<'lit'>, …] LOOP <EXECUTE format('…%I…', <v>)>+ END LOOP; END`.
2. Mỗi `EXECUTE format(<chuỗi>, v)` với chuỗi là `'…'` hoặc `$p$…$p$`: thay `%I` bằng từng phần tử mảng (bỏ nháy), sinh `Statement` DDL tương ứng (`ALTER TABLE s.t ENABLE|FORCE ROW LEVEL SECURITY`, `CREATE POLICY …`) rồi chuyển cho `ddl_postgres.go`.
3. Chỉ chấp nhận `%I` (một lần mỗi chuỗi) và đối số đúng bằng biến lặp; mọi thứ khác (nhiều `%s`, `IF`, `CASE`, truy vấn `SELECT … INTO`) → khối **không khớp**: trả `ok=false` + `ParseWarning{code:"OPAQUE_DO_BLOCK"}`; bảng bị nhắc trong mảng (nếu đọc được) đặt `RLS=unknown`.
4. Móc vào bộ điều phối: câu bắt đầu bằng `DO` đi qua hàm này trước.
5. Giới hạn: ≤ 100 phần tử mảng, ≤ 20 `EXECUTE` mỗi vòng (chống đầu vào độc hại làm nổ bộ nhớ).

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/sqlmigration/... -run RLSDo` (chưa chạy).
- `mcp-service` 0001: 3 bảng cùng `RLS=forced` và policy `tenant_isolation` (USING + WITH CHECK) mỗi bảng; sau đó `relay_read`, `relay_mark_published` (viết thẳng) có mặt. Kiểm cả `auth-service/0011` và 3 file mcp còn lại (0002, 0003, 0004/0006/0007 theo phân loại ở BE-CV-TASK-031-01).
- Ca không khớp: thêm `IF t = 'x' THEN` vào khối → `OPAQUE_DO_BLOCK`, `RLS=unknown`; khối rỗng; mảng 1000 phần tử → vượt giới hạn.
- Không gọi `exec`/bất kỳ trình thông dịch nào (test tĩnh: gói không import `os/exec`).

## Tiêu chí hoàn thành

- [ ] SOL-031 mục 6 hàng `mcp-service` đạt.
- [ ] Mỗi một trong 7 file `DO` được phân loại đúng (khớp/không khớp) và có test.
- [ ] Khối lạ không panic, không đoán.

## Rủi ro và lưu ý

- Nếu về sau migration dùng `FOREACH` với `format('%s.%I', schema, t)`, mẫu cứng sẽ từ chối → `OPAQUE_DO_BLOCK`; đó là hành vi an toàn mong muốn (H7), mở rộng mẫu cần task riêng.
